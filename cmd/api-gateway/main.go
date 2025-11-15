package main

import (
	"context"
	"fmt"
	"log"
	"net/http/pprof"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/noah-isme/sma-adp-api/api/swagger"
	internalhandler "github.com/noah-isme/sma-adp-api/internal/handler"
	internalmiddleware "github.com/noah-isme/sma-adp-api/internal/middleware"
	"github.com/noah-isme/sma-adp-api/internal/models"
	"github.com/noah-isme/sma-adp-api/internal/repository"
	"github.com/noah-isme/sma-adp-api/internal/service"
	"github.com/noah-isme/sma-adp-api/pkg/cache"
	"github.com/noah-isme/sma-adp-api/pkg/config"
	"github.com/noah-isme/sma-adp-api/pkg/database"
	"github.com/noah-isme/sma-adp-api/pkg/jobs"
	"github.com/noah-isme/sma-adp-api/pkg/logger"
	corsmiddleware "github.com/noah-isme/sma-adp-api/pkg/middleware/cors"
	reqidmiddleware "github.com/noah-isme/sma-adp-api/pkg/middleware/requestid"
	"github.com/noah-isme/sma-adp-api/pkg/storage"
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

	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		logr.Sugar().Fatalw("failed to initialise database", "error", err)
	}
	defer db.Close()

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

	api := r.Group(cfg.APIPrefix)

	authRepo := repository.NewUserRepository(db)
	authSvc := service.NewAuthService(authRepo, nil, logr, service.AuthConfig{
		AccessTokenSecret:  cfg.JWT.Secret,
		AccessTokenExpiry:  cfg.JWT.Expiration,
		RefreshTokenExpiry: cfg.JWT.RefreshExpiration,
		Issuer:             "sma-adp-api",
		Audience:           []string{"sma-adp-clients"},
	})
	authHandler := internalhandler.NewAuthHandler(authSvc)

	authRoutes := api.Group("/auth")
	authRoutes.POST("/login", authHandler.Login)
	authRoutes.POST("/refresh", authHandler.Refresh)
	authRoutes.POST("/forgot-password", authHandler.ForgotPassword)
	authRoutes.POST("/reset-password", authHandler.ResetPassword)
	protectedAuth := authRoutes.Group("")
	protectedAuth.Use(internalmiddleware.JWT(authSvc))
	protectedAuth.POST("/logout", authHandler.Logout)
	protectedAuth.POST("/change-password", authHandler.ChangePassword)

	teacherRepo := repository.NewTeacherRepository(db)
	classRepo := repository.NewClassRepository(db)
	subjectRepo := repository.NewSubjectRepository(db)
	termRepo := repository.NewTermRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	assignmentRepo := repository.NewTeacherAssignmentRepository(db)
	homeroomRepo := repository.NewHomeroomRepository(db)
	preferenceRepo := repository.NewTeacherPreferenceRepository(db)
	calendarRepo := repository.NewCalendarRepository(db)
	enrollmentRepo := repository.NewEnrollmentRepository(db)
	semesterScheduleRepo := repository.NewSemesterScheduleRepository(db)
	semesterSlotRepo := repository.NewSemesterScheduleSlotRepository(db)
	configurationRepo := repository.NewConfigurationRepository(db)

	teacherSvc := service.NewTeacherService(teacherRepo, nil, logr)
	calendarSvc := service.NewCalendarService(calendarRepo, nil, logr)
	assignmentSvc := service.NewTeacherAssignmentService(
		teacherRepo,
		classRepo,
		subjectRepo,
		termRepo,
		assignmentRepo,
		scheduleRepo,
		preferenceRepo,
		nil,
		logr,
	)
	preferenceSvc := service.NewTeacherPreferenceService(teacherRepo, preferenceRepo, nil, logr)
	teacherHandler := internalhandler.NewTeacherHandler(teacherSvc, assignmentSvc, preferenceSvc)
	var schedulePreferenceHandler *internalhandler.SchedulePreferenceAliasHandler
	if preferenceSvc != nil {
		schedulePreferenceHandler = internalhandler.NewSchedulePreferenceHandler(preferenceSvc)
	}

	var homeroomHandler *internalhandler.HomeroomHandler
	if cfg.Homerooms.Enabled {
		homeroomSvc := service.NewHomeroomService(
			homeroomRepo,
			classRepo,
			termRepo,
			teacherRepo,
			subjectRepo,
			assignmentRepo,
			authRepo,
			nil,
			logr,
		)
		homeroomHandler = internalhandler.NewHomeroomHandler(homeroomSvc)
	}

	var calendarAliasHandler *internalhandler.CalendarAliasHandler
	if cfg.Aliases.CalendarEnabled {
		calendarAliasSvc := service.NewCalendarAliasService(calendarSvc, termRepo, assignmentSvc, classRepo, logr)
		calendarAliasHandler = internalhandler.NewCalendarAliasHandler(calendarAliasSvc, logr)
	}

	var attendanceSvc *service.AttendanceService
	var attendanceSummaryRepo *repository.AttendanceAliasRepository
	if cfg.Aliases.AttendanceEnabled {
		dailyAttendanceRepo := repository.NewDailyAttendanceRepository(db)
		subjectAttendanceRepo := repository.NewSubjectAttendanceRepository(db)
		attendanceSvc = service.NewAttendanceService(dailyAttendanceRepo, subjectAttendanceRepo, nil, logr)
		attendanceSummaryRepo = repository.NewAttendanceAliasRepository(db)
	}

	var attendanceAliasHandler *internalhandler.AttendanceAliasHandler

	var configurationHandler *internalhandler.ConfigurationHandler
	if cfg.Configuration.Enabled {
		defaults := map[string]string{}
		if cfg.Configuration.ActiveTermID != "" {
			defaults["active_term_id"] = cfg.Configuration.ActiveTermID
		}
		if cfg.Configuration.DefaultDashboardTermID != "" {
			defaults["default_dashboard_term_id"] = cfg.Configuration.DefaultDashboardTermID
		}
		if cfg.Configuration.DefaultCalendarTermID != "" {
			defaults["default_calendar_term_id"] = cfg.Configuration.DefaultCalendarTermID
		}
		configurationSvc := service.NewConfigurationService(
			configurationRepo,
			termRepo,
			authRepo,
			nil,
			logr,
			service.ConfigurationServiceConfig{Defaults: defaults},
		)
		configurationHandler = internalhandler.NewConfigurationHandler(configurationSvc)
	}

	var schedulerHandler *internalhandler.ScheduleGeneratorHandler
	if cfg.Scheduler.Enabled {
		schedulerSvc := service.NewScheduleGeneratorService(
			termRepo,
			classRepo,
			subjectRepo,
			assignmentRepo,
			preferenceRepo,
			scheduleRepo,
			semesterScheduleRepo,
			semesterSlotRepo,
			nil,
			db,
			nil,
			logr,
			service.ScheduleGeneratorConfig{ProposalTTL: cfg.Scheduler.ProposalTTL},
		)
		schedulerHandler = internalhandler.NewScheduleGeneratorHandler(schedulerSvc)
	}

	var analyticsRepo *repository.AnalyticsRepository
	if cfg.Analytics.Enabled || cfg.Dashboard.Enabled || cfg.Reports.Enabled || cfg.Aliases.AttendanceEnabled {
		analyticsRepo = repository.NewAnalyticsRepository(db)
	}

	var cacheRepo service.CacheRepository
	var cacheCloser interface{ Close() error }
	if cfg.Analytics.Enabled || cfg.Dashboard.Enabled {
		if client, err := cache.NewRedis(cfg.Redis); err != nil {
			logr.Sugar().Warnw("cache disabled", "error", err)
		} else {
			cacheCloser = client
			cacheRepo = repository.NewCacheRepository(client, logr)
		}
	}
	if cacheCloser != nil {
		defer cacheCloser.Close()
	}

	var analyticsSvc *service.AnalyticsService
	if cfg.Analytics.Enabled {
		cacheSvc := service.NewCacheService(cacheRepo, metricsSvc, cfg.Analytics.CacheTTL, logr, cacheRepo != nil)
		analyticsSvc = service.NewAnalyticsService(analyticsRepo, cacheSvc, metricsSvc, logr)
		analyticsHandler := internalhandler.NewAnalyticsHandler(analyticsSvc)

		analyticsGroup := api.Group("/analytics")
		analyticsGroup.Use(internalmiddleware.WithResponseMeta())
		analyticsGroup.GET("/attendance", analyticsHandler.Attendance)
		analyticsGroup.GET("/grades", analyticsHandler.Grades)
		analyticsGroup.GET("/behavior", analyticsHandler.Behavior)
		analyticsGroup.GET("/system", analyticsHandler.System)

		registerPprof(r)
	}

	if cfg.Aliases.AttendanceEnabled && attendanceSvc != nil && attendanceSummaryRepo != nil {
		attendanceAliasSvc := service.NewAttendanceAliasService(attendanceSvc, analyticsSvc, attendanceSummaryRepo, assignmentRepo, enrollmentRepo, termRepo, logr)
		attendanceAliasHandler = internalhandler.NewAttendanceAliasHandler(attendanceAliasSvc)
	}

	var reportHandler *internalhandler.ReportHandler
	if cfg.Reports.Enabled {
		if analyticsRepo == nil {
			analyticsRepo = repository.NewAnalyticsRepository(db)
		}
		reportRepo := repository.NewReportRepository(db)
		fileStore, err := storage.NewLocalStorage(cfg.Reports.StorageDir)
		if err != nil {
			logr.Sugar().Fatalw("failed to init report storage", "error", err)
		}
		signer := storage.NewSignedURLSigner(cfg.Reports.SignedURLSecret, cfg.Reports.SignedURLTTL)
		exportCfg := service.ExportConfig{APIPrefix: cfg.APIPrefix, ResultTTL: cfg.Reports.SignedURLTTL}
		exportSvc := service.NewExportService(analyticsRepo, fileStore, signer, exportCfg, logr, nil, nil)
		reportWorker := service.NewReportWorker(reportRepo, exportSvc, cfg.Reports.WorkerRetries, logr)
		workers := cfg.Reports.WorkerConcurrency
		if workers <= 0 {
			workers = 1
		}
		queueCfg := jobs.QueueConfig{
			Workers:    workers,
			BufferSize: workers * 4,
			MaxRetries: cfg.Reports.WorkerRetries,
			RetryDelay: 5 * time.Second,
			Logger:     logr,
		}
		queueCtx, cancel := context.WithCancel(context.Background())
		reportQueue := jobs.NewQueue("reports", reportWorker.Handle, queueCfg)
		reportQueue.Start(queueCtx)
		defer func() {
			cancel()
			reportQueue.Stop()
		}()
		reportSvc := service.NewReportService(reportRepo, assignmentRepo, reportQueue, exportSvc, logr, service.ReportServiceConfig{
			ResultTTL:       cfg.Reports.SignedURLTTL,
			CleanupInterval: cfg.Reports.CleanupInterval,
			MaxRetries:      cfg.Reports.WorkerRetries,
		})
		reportSvc.RecoverPendingJobs(queueCtx)
		reportSvc.StartCleanup(queueCtx)
		reportHandler = internalhandler.NewReportHandler(reportSvc, nil)
	}

	var mutationHandler *internalhandler.MutationHandler
	if cfg.Mutations.Enabled {
		mutationRepo := repository.NewMutationRepository(db)
		studentRepo := repository.NewStudentRepository(db)
		mutationSvc := service.NewMutationService(mutationRepo, authRepo, logr, service.WithMutationAppliers(map[string]service.MutationApplier{
			"student": service.NewStudentMutationApplier(studentRepo, logr),
		}))
		mutationHandler = internalhandler.NewMutationHandler(mutationSvc)
	}

	var archiveHandler *internalhandler.ArchiveHandler
	if cfg.Archives.Enabled {
		if cfg.Archives.SignedURLSecret == "" {
			logr.Sugar().Fatal("archives signed url secret not configured")
		}
		archiveRepo := repository.NewArchiveRepository(db)
		archiveStore, err := storage.NewLocalStorage(cfg.Archives.StorageDir)
		if err != nil {
			logr.Sugar().Fatalw("failed to init archive storage", "error", err)
		}
		archiveSigner := storage.NewSignedURLSigner(cfg.Archives.SignedURLSecret, cfg.Archives.SignedURLTTL)
		archiveSvc := service.NewArchiveService(
			archiveRepo,
			assignmentRepo,
			enrollmentRepo,
			archiveStore,
			archiveSigner,
			authRepo,
			logr,
			service.ArchiveServiceConfig{
				MaxFileSize:  cfg.Archives.MaxFileSizeBytes,
				AllowedMIMEs: cfg.Archives.AllowedMIMEs,
				APIPrefix:    cfg.APIPrefix,
			},
		)
		archiveHandler = internalhandler.NewArchiveHandler(archiveSvc)
	}

	secured := api.Group("")
	secured.Use(internalmiddleware.JWT(authSvc))

	teachersGroup := secured.Group("/teachers")
	teachersGroup.GET("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.List)
	teachersGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.Create)
	teachersGroup.GET("/:id", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.Get)
	teachersGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.Update)
	teachersGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), teacherHandler.Delete)
	teachersGroup.GET("/:id/assignments", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.ListAssignments)
	teachersGroup.POST("/:id/assignments", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.CreateAssignment)
	teachersGroup.DELETE("/:id/assignments/:aid", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.DeleteAssignment)
	teachersGroup.GET("/:id/preferences", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.GetPreferences)
	teachersGroup.PUT("/:id/preferences", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.UpsertPreferences)

	if calendarAliasHandler != nil {
		secured.GET("/calendar", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarAliasHandler.List)
	}

	if attendanceAliasHandler != nil {
		attendanceGroup := secured.Group("/attendance")
		attendanceGroup.Use(internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)))
		attendanceGroup.GET("", attendanceAliasHandler.Summary)
		attendanceGroup.GET("/daily", attendanceAliasHandler.Daily)
	}

	if configurationHandler != nil {
		configGroup := secured.Group("/configuration")
		configGroup.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)))
		configGroup.GET("", configurationHandler.List)
		configGroup.GET("/:key", configurationHandler.Get)
		configGroup.PUT("/:key", configurationHandler.Update)
		configGroup.PUT("/bulk", configurationHandler.BulkUpdate)
	}

	if homeroomHandler != nil {
		homerooms := secured.Group("/homerooms")
		homerooms.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), homeroomHandler.List)
		homerooms.GET("/:classId", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), homeroomHandler.Get)
		homerooms.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), homeroomHandler.Set)
	}

	if schedulerHandler != nil {
		schedulerGroup := secured.Group("")
		schedulerGroup.POST("/schedule/generate", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulerHandler.Generate)
		schedulerGroup.POST("/schedules/generator", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulerHandler.GenerateAlias)
		schedulerGroup.POST("/schedule/save", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulerHandler.Save)
		schedulerGroup.GET("/semester-schedule", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulerHandler.List)
		schedulerGroup.GET("/semester-schedule/:id/slots", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulerHandler.Slots)
		schedulerGroup.DELETE("/semester-schedule/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), schedulerHandler.Delete)
	}

	if schedulePreferenceHandler != nil {
		schedulesGroup := secured.Group("/schedules")
		schedulesGroup.GET("/preferences", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulePreferenceHandler.Get)
		schedulesGroup.POST("/preferences", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulePreferenceHandler.Upsert)
	}

	if reportHandler != nil {
		reportsGroup := secured.Group("/reports")
		reportsGroup.POST("/generate", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), reportHandler.GenerateReport)
		reportsGroup.GET("/status/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), reportHandler.ReportStatus)
		secured.GET("/export/:token", reportHandler.DownloadReport)
	}

	if mutationHandler != nil {
		mutations := secured.Group("/mutations")
		mutations.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), mutationHandler.Create)
		mutations.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), mutationHandler.List)
		mutations.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), mutationHandler.Get)
		mutations.POST("/:id/review", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), mutationHandler.Review)
	}

	if archiveHandler != nil {
		archives := secured.Group("/archives")
		archives.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), archiveHandler.Upload)
		archives.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), archiveHandler.List)
		archives.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), archiveHandler.Get)
		archives.GET("/:id/download", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), archiveHandler.Download)
		archives.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), archiveHandler.Delete)
	}

	if cfg.Dashboard.Enabled {
		dashboardCache := service.NewCacheService(cacheRepo, metricsSvc, cfg.Dashboard.CacheTTL, logr, cacheRepo != nil)
		announcementSvc := service.NewAnnouncementService(repository.NewAnnouncementRepository(db), nil, logr)
		scheduleSvc := service.NewScheduleService(scheduleRepo, nil, logr)
		dashboardSvc := service.NewDashboardService(service.DashboardServiceParams{
			Analytics:     analyticsSvc,
			AnalyticsRepo: analyticsRepo,
			Calendar:      calendarSvc,
			Announcements: announcementSvc,
			Schedules:     scheduleSvc,
			Assignments:   assignmentSvc,
			Cache:         dashboardCache,
			Logger:        logr,
			Config:        service.DashboardServiceConfig{CacheTTL: cfg.Dashboard.CacheTTL},
		})
		dashboardHandler := internalhandler.NewDashboardHandler(dashboardSvc)

		dashboardGroup := secured.Group("")
		dashboardGroup.Use(internalmiddleware.WithResponseMeta())
		dashboardGroup.GET("/dashboard", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), dashboardHandler.Admin)
		dashboardGroup.GET("/dashboard/academics", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), dashboardHandler.Teacher)
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
