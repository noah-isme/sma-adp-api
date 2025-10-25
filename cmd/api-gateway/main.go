package main

import (
	"fmt"
	"log"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/noah-isme/sma-adp-api/api/swagger"
	internalhandler "github.com/noah-isme/sma-adp-api/internal/handler"
	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	"github.com/noah-isme/sma-adp-api/internal/service"
	"github.com/noah-isme/sma-adp-api/pkg/cache"
	"github.com/noah-isme/sma-adp-api/pkg/config"
	"github.com/noah-isme/sma-adp-api/pkg/database"
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

	metricsSvc := service.NewMetricsService()
	metricsHandler := internalhandler.NewMetricsHandler(metricsSvc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(reqidmiddleware.Middleware())
	r.Use(logger.GinMiddleware(logr))
	r.Use(corsmiddleware.New(cfg.CORS.AllowedOrigins))
	cutoverSvc := service.NewCutoverService(cfg.Cutover, metricsSvc)

	r.Use(internalmiddleware.CutoverStage(cutoverSvc))
	r.Use(internalmiddleware.Metrics(metricsSvc))

	r.GET("/health", metricsHandler.Health)

	r.GET("/ready", metricsHandler.Health)

	if cfg.Env != config.EnvProduction {
		r.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	r.GET("/metrics", metricsHandler.Prometheus)

	cutoverHandler := internalhandler.NewCutoverHandler(cutoverSvc)

	internalGroup := r.Group("/internal")
	internalGroup.GET("/ping-legacy", cutoverHandler.PingLegacy)
	internalGroup.GET("/ping-go", cutoverHandler.PingGo)

	if cfg.Analytics.Enabled {
		db, err := database.NewPostgres(cfg.Database)
		if err != nil {
			logr.Sugar().Fatalw("failed to initialise database", "error", err)
		}
		defer db.Close()

		var cacheRepo service.CacheRepository
		if redisClient, err := cache.NewRedis(cfg.Redis); err != nil {
			logr.Sugar().Warnw("analytics cache disabled", "error", err)
		} else {
			defer redisClient.Close()
			cacheRepo = repository.NewCacheRepository(redisClient, logr)
		}

		cacheSvc := service.NewCacheService(cacheRepo, metricsSvc, cfg.Analytics.CacheTTL, logr, cacheRepo != nil)
		analyticsSvc := service.NewAnalyticsService(repository.NewAnalyticsRepository(db), cacheSvc, metricsSvc, logr)
		analyticsHandler := internalhandler.NewAnalyticsHandler(analyticsSvc)

		api := r.Group(cfg.APIPrefix)
		analyticsGroup := api.Group("/analytics")
		analyticsGroup.Use(internalmiddleware.WithResponseMeta())
		analyticsGroup.GET("/attendance", analyticsHandler.Attendance)
		analyticsGroup.GET("/grades", analyticsHandler.Grades)
		analyticsGroup.GET("/behavior", analyticsHandler.Behavior)
		analyticsGroup.GET("/system", analyticsHandler.System)

		registerPprof(r)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	logr.Sugar().Infow("server starting", "addr", addr, "env", cfg.Env)
	if err := r.Run(addr); err != nil {
		logr.Sugar().Fatalw("server failed", "error", err)
	}
}

func registerPprof(r *gin.Engine) {
	group := r.Group("/debug/pprof")
	group.GET("/", gin.WrapF(pprof.Index))
	group.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	group.GET("/profile", gin.WrapF(pprof.Profile))
	group.POST("/symbol", gin.WrapF(pprof.Symbol))
	group.GET("/symbol", gin.WrapF(pprof.Symbol))
	group.GET("/trace", gin.WrapF(pprof.Trace))
	group.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	group.GET("/block", gin.WrapH(pprof.Handler("block")))
	group.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	group.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	group.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	group.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}
