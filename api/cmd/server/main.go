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
	"github.com/guijoazeiro/MiniPaaS/api/internal/config"
	"github.com/guijoazeiro/MiniPaaS/api/internal/docker"
	"github.com/guijoazeiro/MiniPaaS/api/internal/handler"
	"github.com/guijoazeiro/MiniPaaS/api/internal/service"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres"
	"github.com/guijoazeiro/MiniPaaS/api/internal/store/postgres/sqlc"
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

	q := sqlc.New(pool)
	appStore := postgres.NewAppStore(q)
	depStore := postgres.NewDeploymentStore(q)

	appSvc := service.NewAppService(appStore)
	depSvc := service.NewDeploymentService(depStore, appStore, dockerCli, log)
	facade := &service.AppFacade{App: appSvc, Dep: depSvc}

	appH := handler.NewAppHandler(facade, log)
	depH := handler.NewDeploymentHandler(depSvc, appStore, log)

	if !strings.EqualFold(cfg.LogLevel, "debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), requestLogger(log), corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/apps", appH.Create)
	r.GET("/apps", appH.List)
	r.GET("/apps/:name", appH.Get)
	r.DELETE("/apps/:name", appH.Delete)

	r.POST("/apps/:name/deployments", depH.Create)
	r.GET("/apps/:name/deployments", depH.List)
	r.GET("/apps/:name/deployments/:id", depH.Get)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("http listening", "addr", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutting down")

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
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"dur_ms", time.Since(start).Milliseconds(),
		)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
