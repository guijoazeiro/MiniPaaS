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
	envStore := postgres.NewEnvStore(q)
	rollbackStore := postgres.NewRollbackStore(q)
	gitSourceStore := postgres.NewGitSourceStore(q)
	githubInstallationStore := postgres.NewGitHubInstallationStore(q)
	githubWebhookStore := postgres.NewGitHubWebhookDeliveryStore(q)

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
	envSvc := service.NewEnvService(envStore, cipher)
	appSvc := service.NewAppService(appStore)
	domainSvc := service.NewCustomDomainService(domainStore, appStore, depStore, caddyCli, cfg.BaseDomain, cfg.PublicIP)
	metricsSvc := service.NewMetricsService(appStore, depStore, depLogStore, dockerCli)
	depSvc := service.NewDeploymentService(depStore, appStore, rollbackStore, dockerCli, caddyCli, envSvc, cfg.ImageRetention, cfg.RestartPolicy, cfg.RestartMaxRetries, log, service.DeploymentServiceOptions{
		Logs:          depLogStore,
		ReadyTimeout:  cfg.DeployReadyTimeout,
		CustomDomains: domainSvc,
		RuntimeLimits: docker.ResourceLimits{MemoryBytes: cfg.ContainerMemoryBytes, NanoCPUs: cfg.ContainerNanoCPUs, PidsLimit: cfg.ContainerPidsLimit},
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
	appH := handler.NewAppHandler(appSvc, depSvc, healthChecker, log)
	domainH := handler.NewCustomDomainHandler(domainSvc, log)
	metricsH := handler.NewMetricsHandler(metricsSvc, log)
	depH := handler.NewDeploymentHandler(depSvc, appStore, log, cfg.MaxDeploySize, depLogStore, gitDepSvc)
	gitH := handler.NewGitSourceHandler(gitSourceSvc, gitDepSvc, log, webhooksEnabled)
	githubH := handler.NewGitHubAppHandler(githubSvc, cfg.DashboardOrigin, log, webhooksEnabled)
	githubWebhookH := handler.NewGitHubWebhookHandler(webhookSecret, githubWebhookSvc, log)
	envH := handler.NewEnvHandler(envSvc, appStore, log)
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
	r.Use(middleware.RequestID(), gin.Recovery(), requestLogger(log), corsMiddleware(cfg.DashboardOrigin))
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
	r.POST("/auth/logout", authH.Logout)
	r.POST("/integrations/github/webhook", webhookRateLimiter.RateLimit(middleware.RemoteIPKey), githubWebhookH.Handle)

	auth := r.Group("/", middleware.Auth(authSvc))

	auth.POST("/apps", appH.Create)
	auth.GET("/apps", appH.List)
	auth.GET("/apps/:name", appH.Get)
	auth.POST("/apps/:name/stop", appH.Stop)
	auth.DELETE("/apps/:name", appH.Delete)
	auth.GET("/apps/:name/metrics", metricsH.Get)
	auth.GET("/apps/:name/domains", domainH.List)
	auth.POST("/apps/:name/domains", domainH.Create)
	auth.POST("/apps/:name/domains/:domainID/verify", domainH.Verify)
	auth.DELETE("/apps/:name/domains/:domainID", domainH.Delete)

	auth.POST("/apps/:name/deployments", depH.Create)
	auth.GET("/deployments", depH.ListAll)
	auth.POST("/apps/:name/deployments/git", gitH.Deploy)
	auth.GET("/apps/:name/deployments", depH.List)
	auth.GET("/apps/:name/deployments/:id", depH.Get)
	auth.GET("/apps/:name/deployments/:id/logs", depH.Logs)
	auth.POST("/apps/:name/deployments/:id/cancel", depH.Cancel)
	auth.POST("/apps/:name/deployments/:id/retry", depH.Retry)
	auth.POST("/apps/:name/rollback", depH.Rollback)
	auth.PUT("/apps/:name/source/git", gitH.Configure)
	auth.PUT("/apps/:name/source/github-app", gitH.ConfigureGitHubApp)
	auth.GET("/apps/:name/source/git", gitH.Get)
	auth.DELETE("/apps/:name/source/git", gitH.Delete)
	auth.PATCH("/apps/:name/source/git/auto-deploy", gitH.SetAutoDeploy)

	auth.GET("/integrations/github/status", githubH.Status)
	auth.GET("/integrations/github/install-url", githubH.InstallURL)
	auth.GET("/integrations/github/callback", githubH.Callback)
	auth.GET("/integrations/github/installations", githubH.ListInstallations)
	auth.GET("/integrations/github/installations/:id/repositories", githubH.ListRepositories)

	auth.GET("/apps/:name/env", envH.List)
	auth.PUT("/apps/:name/env/:key", envH.Set)
	auth.DELETE("/apps/:name/env/:key", envH.Delete)

	auth.GET("/apps/:name/logs", wsH.Serve)
	auth.GET("/apps/:name/metrics/stream", metricsWS.Serve)

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
