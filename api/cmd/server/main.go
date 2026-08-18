package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guijoazeiro/MiniPaaS/api/internal/caddy"
	"github.com/guijoazeiro/MiniPaaS/api/internal/config"
	"github.com/guijoazeiro/MiniPaaS/api/internal/crypto"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/domain"
	"github.com/guijoazeiro/MiniPaaS/api/internal/githubapp"
	"github.com/guijoazeiro/MiniPaaS/api/internal/handler"
	"github.com/guijoazeiro/MiniPaaS/api/internal/handler/middleware"
	"github.com/guijoazeiro/MiniPaaS/api/internal/health"
	"github.com/guijoazeiro/MiniPaaS/api/internal/service"
	"github.com/guijoazeiro/MiniPaaS/api/internal/sourcegit"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
	wspkg "github.com/guijoazeiro/MiniPaaS/api/internal/ws"
	"github.com/joho/godotenv"
)

func main() {
	for _, p := range []string{".env", "../.env"} {
		_ = godotenv.Load(p)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres.Connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	dockerCli, err := docker.New(cfg.DockerHost)
	if err != nil {
		log.Error("docker.New", "err", err)
		os.Exit(1)
	}
	defer dockerCli.Close()

	caddyCli := caddy.New(cfg.CaddyAdminURL, cfg.BaseDomain)
	if err := caddyCli.EnsureBase(ctx); err != nil {
		log.Warn("caddy.EnsureBase (proxy may be down; deploys will still record but skip routes)", "err", err)
	}

	cipher, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		log.Error("crypto.New", "err", err)
		os.Exit(1)
	}

	q := sqlc.New(pool)
	appStore := postgres.NewAppStore(q)
	depStore := postgres.NewDeploymentStore(q)
	domainStore := postgres.NewCustomDomainStore(q)
	depLogStore := postgres.NewDeploymentLogStore(q)
	userStore := postgres.NewUserStore(q)
	apiTokenStore := postgres.NewAPITokenStore(q)
	envStore := postgres.NewEnvStore(q)
	rollbackStore := postgres.NewRollbackStore(q)
	gitSourceStore := postgres.NewGitSourceStore(q)
	githubInstallationStore := postgres.NewGitHubInstallationStore(q)
	githubWebhookStore := postgres.NewGitHubWebhookDeliveryStore(q)
	auditStore := postgres.NewAuditStore(q)

	var githubClient service.GitHubAppClient
	var githubTokens sourcegit.InstallationTokenProvider
	var githubStates service.GitHubStateSigner
	if cfg.GitHubAppID > 0 {
		client, err := githubapp.NewFromFile(cfg.GitHubAppID, cfg.GitHubAppSlug, cfg.GitHubAppPrivateKeyPath, cfg.GitHubAPIURL)
		if err != nil {
			log.Error("githubapp.New", "err", err)
			os.Exit(1)
		}
		githubClient = client
		githubTokens = client
		githubStates = githubapp.NewStateSigner([]byte(cfg.JWTSecret))
	}

	authSvc := service.NewAuthService(userStore, []byte(cfg.JWTSecret), cfg.TokenTTL, log)
	apiTokenSvc := service.NewAPITokenService(apiTokenStore)
	authenticator := service.NewAuthenticator(authSvc, apiTokenSvc)
	envSvc := service.NewEnvService(envStore, cipher)
	appSvc := service.NewAppService(appStore, service.AppServiceOptions{MaxAppsPerUser: cfg.MaxAppsPerUser})
	domainSvc := service.NewCustomDomainService(domainStore, appStore, depStore, caddyCli, cfg.BaseDomain, cfg.PublicIP)
	metricsSvc := service.NewMetricsService(appStore, depStore, depLogStore, dockerCli)
	depSvc := service.NewDeploymentService(depStore, appStore, rollbackStore, dockerCli, caddyCli, envSvc, cfg.ImageRetention, cfg.RestartPolicy, cfg.RestartMaxRetries, log, service.DeploymentServiceOptions{
		Logs:                depLogStore,
		ReadyTimeout:        cfg.DeployReadyTimeout,
		BuildTimeout:        cfg.BuildTimeout,
		MaxConcurrentBuilds: cfg.MaxConcurrentBuilds,
		CustomDomains:       domainSvc,
		RuntimeLimits:       &docker.ResourceLimits{MemoryBytes: cfg.ContainerMemoryBytes, NanoCPUs: cfg.ContainerNanoCPUs, PidsLimit: cfg.ContainerPidsLimit},
	})
	capacitySvc := service.NewCapacityService(appStore, depSvc, service.CapacityOptions{
		MaxAppsPerUser:            cfg.MaxAppsPerUser,
		ContainerMemoryLimitBytes: cfg.ContainerMemoryBytes,
		ContainerNanoCPUs:         cfg.ContainerNanoCPUs,
		ContainerPidsLimit:        cfg.ContainerPidsLimit,
	})
	if err := depSvc.RecoverCandidates(ctx); err != nil {
		log.Warn("recover deployment candidates", "err", err)
	}
	reconcileCtx, cancelReconcile := context.WithTimeout(context.Background(), 30*time.Second)
	if removed, err := service.ReconcileManagedContainers(reconcileCtx, dockerCli, depStore); err != nil {
		log.Warn("reconcile managed containers", "err", err)
	} else if removed > 0 {
		log.Info("reconciled managed containers", "removed", removed)
	}
	cancelReconcile()
	githubSvc := service.NewGitHubAppService(appStore, githubInstallationStore, githubClient, githubStates)
	gitSourceSvc := service.NewGitSourceService(appStore, gitSourceStore, githubSvc)
	gitPreparer := sourcegit.New(cfg.MaxRepositorySize)
	if githubTokens != nil {
		gitPreparer = sourcegit.NewWithTokenProvider(cfg.MaxRepositorySize, githubTokens)
	}
	gitDepSvc := service.NewGitDeploymentService(gitSourceStore, appStore, depSvc, gitPreparer, cfg.GitCloneTimeout)
	githubWebhookSvc := service.NewGitHubWebhookService(gitSourceStore, appStore, githubWebhookStore, gitDepSvc, log)
	webhooksEnabled := cfg.GitHubAppID > 0 && cfg.GitHubWebhookSecret != ""
	webhookSecret := cfg.GitHubWebhookSecret
	if !webhooksEnabled {
		webhookSecret = ""
	}
	healthChecker := health.New(depStore, appStore, dockerCli, cfg.HealthCheckInterval, log, depLogStore)

	if err := authSvc.SeedAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Error("seed admin", "err", err)
		os.Exit(1)
	}

	authH := handler.NewAuthHandler(authSvc, log)
	apiTokenH := handler.NewAPITokenHandler(apiTokenSvc, log)
	appH := handler.NewAppHandler(appSvc, depSvc, healthChecker, log)
	domainH := handler.NewCustomDomainHandler(domainSvc, log)
	metricsH := handler.NewMetricsHandler(metricsSvc, log)
	capacityH := handler.NewCapacityHandler(capacitySvc, log)
	depH := handler.NewDeploymentHandler(depSvc, appStore, log, cfg.MaxDeploySize, depLogStore, gitDepSvc)
	gitH := handler.NewGitSourceHandler(gitSourceSvc, gitDepSvc, log, webhooksEnabled)
	githubH := handler.NewGitHubAppHandler(githubSvc, cfg.DashboardOrigin, log, webhooksEnabled)
	githubWebhookH := handler.NewGitHubWebhookHandler(webhookSecret, githubWebhookSvc, log)
	envH := handler.NewEnvHandler(envSvc, appStore, log)
	auditH := handler.NewAuditHandler(auditStore, log)
	wsH := wspkg.New(appStore, dockerCli, depStore, log, cfg.DashboardOrigin)
	metricsWS := wspkg.NewMetricsStreamHandler(appStore, depStore, dockerCli, log, cfg.DashboardOrigin)
	readyH := handler.NewReadinessHandler(cfg.ReadinessTimeout, map[string]handler.ReadinessProbe{
		"database": pool.Ping,
		"docker":   dockerCli.Ping,
		"caddy":    caddyCli.Ping,
	}, log)

	if !strings.EqualFold(cfg.LogLevel, "debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.RequestID(), gin.Recovery(), requestLogger(log), middleware.Audit(auditStore, log), corsMiddleware(cfg.DashboardOrigin))
	authRateLimiter := middleware.NewRateLimiter(cfg.AuthRateLimit, cfg.RateLimitWindow)
	webhookRateLimiter := middleware.NewRateLimiter(cfg.WebhookRateLimit, cfg.RateLimitWindow)

	r.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ready", readyH.Serve)
	r.POST("/auth/login", authRateLimiter.RateLimit(middleware.RemoteIPKey), authH.Login)
	r.POST("/auth/web-login", authRateLimiter.RateLimit(middleware.RemoteIPKey), authH.WebLogin)
	r.POST("/auth/register", authRateLimiter.RateLimit(middleware.RemoteIPKey), authH.Register)
	r.POST("/auth/logout", authH.Logout)
	r.POST("/integrations/github/webhook", webhookRateLimiter.RateLimit(middleware.RemoteIPKey), githubWebhookH.Handle)

	auth := r.Group("/", middleware.Auth(authenticator))

	auth.GET("/me", middleware.RequireScope(domain.APITokenScopeRead), authH.Me)
	auth.PATCH("/me", middleware.RequireSession(), authH.UpdateMe)
	auth.PATCH("/me/password", middleware.RequireSession(), authH.UpdatePassword)
	auth.POST("/me/tokens", middleware.RequireSession(), apiTokenH.Create)
	auth.GET("/me/tokens", middleware.RequireSession(), apiTokenH.List)
	auth.DELETE("/me/tokens/:id", middleware.RequireSession(), apiTokenH.Revoke)
	auth.POST("/apps", middleware.RequireScope(domain.APITokenScopeManage), appH.Create)
	auth.GET("/apps", middleware.RequireScope(domain.APITokenScopeRead), appH.List)
	auth.GET("/capacity", middleware.RequireScope(domain.APITokenScopeRead), capacityH.Get)
	auth.GET("/apps/:name", middleware.RequireScope(domain.APITokenScopeRead), appH.Get)
	auth.POST("/apps/:name/stop", middleware.RequireScope(domain.APITokenScopeManage), appH.Stop)
	auth.DELETE("/apps/:name", middleware.RequireScope(domain.APITokenScopeManage), appH.Delete)
	auth.GET("/apps/:name/metrics", middleware.RequireScope(domain.APITokenScopeRead), metricsH.Get)
	auth.GET("/apps/:name/domains", middleware.RequireScope(domain.APITokenScopeRead), domainH.List)
	auth.POST("/apps/:name/domains", middleware.RequireScope(domain.APITokenScopeManage), domainH.Create)
	auth.POST("/apps/:name/domains/:domainID/verify", middleware.RequireScope(domain.APITokenScopeManage), domainH.Verify)
	auth.DELETE("/apps/:name/domains/:domainID", middleware.RequireScope(domain.APITokenScopeManage), domainH.Delete)

	auth.POST("/apps/:name/deployments", middleware.RequireScope(domain.APITokenScopeDeploy), depH.Create)
	auth.GET("/deployments", middleware.RequireScope(domain.APITokenScopeRead), depH.ListAll)
	auth.POST("/apps/:name/deployments/git", middleware.RequireScope(domain.APITokenScopeDeploy), gitH.Deploy)
	auth.GET("/apps/:name/deployments", middleware.RequireScope(domain.APITokenScopeRead), depH.List)
	auth.GET("/apps/:name/deployments/:id", middleware.RequireScope(domain.APITokenScopeRead), depH.Get)
	auth.GET("/apps/:name/deployments/:id/logs", middleware.RequireScope(domain.APITokenScopeRead), depH.Logs)
	auth.POST("/apps/:name/deployments/:id/cancel", middleware.RequireScope(domain.APITokenScopeDeploy), depH.Cancel)
	auth.POST("/apps/:name/deployments/:id/retry", middleware.RequireScope(domain.APITokenScopeDeploy), depH.Retry)
	auth.POST("/apps/:name/rollback", middleware.RequireScope(domain.APITokenScopeDeploy), depH.Rollback)
	auth.PUT("/apps/:name/source/git", middleware.RequireScope(domain.APITokenScopeManage), gitH.Configure)
	auth.PUT("/apps/:name/source/github-app", middleware.RequireScope(domain.APITokenScopeManage), gitH.ConfigureGitHubApp)
	auth.GET("/apps/:name/source/git", middleware.RequireScope(domain.APITokenScopeRead), gitH.Get)
	auth.DELETE("/apps/:name/source/git", middleware.RequireScope(domain.APITokenScopeManage), gitH.Delete)
	auth.PATCH("/apps/:name/source/git/auto-deploy", middleware.RequireScope(domain.APITokenScopeManage), gitH.SetAutoDeploy)

	auth.GET("/integrations/github/status", middleware.RequireScope(domain.APITokenScopeRead), githubH.Status)
	auth.GET("/integrations/github/install-url", middleware.RequireSession(), githubH.InstallURL)
	auth.GET("/integrations/github/account-install-url", middleware.RequireSession(), githubH.AccountInstallURL)
	auth.GET("/integrations/github/callback", middleware.RequireSession(), githubH.Callback)
	auth.GET("/integrations/github/installations", middleware.RequireScope(domain.APITokenScopeRead), githubH.ListInstallations)
	auth.GET("/integrations/github/installations/:id/repositories", middleware.RequireScope(domain.APITokenScopeRead), githubH.ListRepositories)
	auth.GET("/audit", middleware.RequireScope(domain.APITokenScopeRead), auditH.List)

	auth.GET("/apps/:name/env", middleware.RequireScope(domain.APITokenScopeRead), envH.List)
	auth.PUT("/apps/:name/env/:key", middleware.RequireScope(domain.APITokenScopeManage), envH.Set)
	auth.DELETE("/apps/:name/env/:key", middleware.RequireScope(domain.APITokenScopeManage), envH.Delete)

	auth.GET("/apps/:name/logs", middleware.RequireScope(domain.APITokenScopeRead), wsH.Serve)
	auth.GET("/apps/:name/metrics/stream", middleware.RequireScope(domain.APITokenScopeRead), metricsWS.Serve)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		// Global ReadTimeout and WriteTimeout are omitted because deploy uploads
		// and long-lived WebSockets have different lifetimes. Upload bodies are
		// size-limited in their handler; WebSockets manage their own deadlines.
	}

	go func() {
		log.Info("http listening", "addr", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http", "err", err)
			os.Exit(1)
		}
	}()

	healthCtx, cancelHealth := context.WithCancel(context.Background())
	healthChecker.Start(healthCtx)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")
	cancelHealth()
	healthChecker.Stop()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			"request_id", middleware.RequestIDValue(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"dur_ms", time.Since(start).Milliseconds(),
		)
	}
}

func corsMiddleware(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == allowedOrigin {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
