package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/noah-isme/sma-adp-api/api/swagger"
	"github.com/noah-isme/sma-adp-api/pkg/config"
	"github.com/noah-isme/sma-adp-api/pkg/logger"
	corsmiddleware "github.com/noah-isme/sma-adp-api/pkg/middleware/cors"
	reqidmiddleware "github.com/noah-isme/sma-adp-api/pkg/middleware/requestid"
)

// @title SMA ADP API
// @version 0.1.0
// @description Bootstrap server for Golang migration (Phase 0)
// @BasePath /
// @schemes http

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logr, err := logger.New(cfg)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logr.Sync() //nolint:errcheck

	if cfg.Env == config.EnvProduction {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(reqidmiddleware.Middleware())
	r.Use(logger.GinMiddleware(logr))
	r.Use(corsmiddleware.New(cfg.CORS.AllowedOrigins))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	if cfg.Env != config.EnvProduction {
		r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	logr.Sugar().Infow("server starting", "addr", addr, "env", cfg.Env)
	if err := r.Run(addr); err != nil {
		logr.Sugar().Fatalw("server failed", "error", err)
	}
}
