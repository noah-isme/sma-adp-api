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
	"github.com/noah-isme/sma-adp-api/internal/archives"
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
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

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
		if cfg.JWT.Secret == "" || cfg.JWT.Secret == "dev_secret" || cfg.JWT.Secret == "change_me_in_prod" || cfg.JWT.Secret == "REPLACE_IN_SECRET_STORE" {
			logr.Sugar().Fatal("JWT_SECRET must be set to a non-development secret in production")
		}
		if cfg.Database.SSLMode != "require" {
			logr.Sugar().Fatal("DB_SSL_MODE=require is required in production")
		}
		if len(cfg.CORS.AllowedOrigins) == 0 {
			logr.Sugar().Fatal("ALLOWED_ORIGINS must contain the production admin origin")
		}
		if cfg.PasswordReset.URL == "" {
			logr.Sugar().Fatal("PASSWORD_RESET_URL must be set in production")
		}
		if cfg.SMTP.TLSMode != "starttls" && cfg.SMTP.TLSMode != "tls" {
			logr.Sugar().Fatal("SMTP_TLS_MODE must be starttls or tls in production")
		}
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

	// Feature discovery is unauthenticated on purpose: the admin panel reads it
	// before login to decide which navigation entries to render. It returns only
	// which modules are mounted, never data or configuration values.
	featureHandler := internalhandler.NewFeatureHandler(cfg)
	api.GET("/features", featureHandler.List)

	authRepo := repository.NewUserRepository(db)
	teacherRepo := repository.NewTeacherRepository(db)
	passwordResetDelivery := service.PasswordResetEmailDelivery(service.NoopPasswordResetEmailDelivery{})
	if cfg.Env != config.EnvProduction {
		passwordResetDelivery = service.NewLoggingPasswordResetEmailDelivery(logr)
	} else {
		if !cfg.SMTP.Enabled {
			logr.Sugar().Fatal("SMTP_ENABLED=true is required in production so password-reset requests are deliverable")
		}
		passwordResetDelivery, err = service.NewSMTPPasswordResetEmailDelivery(service.SMTPPasswordResetEmailConfig{
			Host:     cfg.SMTP.Host,
			Port:     cfg.SMTP.Port,
			Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password,
			From:     cfg.SMTP.From,
			TLSMode:  cfg.SMTP.TLSMode,
			Timeout:  cfg.SMTP.Timeout,
			Subject:  cfg.PasswordReset.Subject,
		})
		if err != nil {
			logr.Sugar().Fatalw("failed to configure password reset SMTP delivery", "error", err)
		}
	}
	authSvc := service.NewAuthServiceWithEmailDelivery(authRepo, teacherRepo, nil, logr, service.AuthConfig{
		AccessTokenSecret:     cfg.JWT.Secret,
		AccessTokenExpiry:     cfg.JWT.Expiration,
		RefreshTokenExpiry:    cfg.JWT.RefreshExpiration,
		PasswordResetTokenTTL: cfg.PasswordReset.TokenTTL,
		PasswordResetURL:      cfg.PasswordReset.URL,
		Issuer:                "sma-adp-api",
		Audience:              []string{"sma-adp-clients"},
	}, passwordResetDelivery)
	authHandler := internalhandler.NewAuthHandler(authSvc)

	// Initialize student repo early for portal auth
	studentRepo := repository.NewStudentRepository(db)

	// Portal repositories (needed for PortalLookup)
	parentStudentRepo := repository.NewParentStudentRepository(db)
	portalPreferencesRepo := repository.NewPortalPreferencesRepository(db)
	deviceTokenRepo := repository.NewDeviceTokenRepository(db)

	// Portal Lookup composite
	portalLookup := repository.NewPortalLookup(studentRepo, parentStudentRepo, portalPreferencesRepo, deviceTokenRepo)

	// Portal Auth Service
	portalAuthConfig := service.PortalAuthConfig{
		AccessTokenSecret:     cfg.JWT.Secret,
		AccessTokenExpiry:     cfg.JWT.Expiration,
		RefreshTokenExpiry:    cfg.JWT.RefreshExpiration,
		PasswordResetTokenTTL: cfg.PasswordReset.TokenTTL,
		PasswordResetURL:      cfg.PasswordReset.PortalURL,
		Issuer:                "sma-adp-api",
		Audience:              []string{"sma-adp-clients"},
	}
	portalAuthSvc := service.NewPortalAuthServiceWithEmailDelivery(authRepo, portalLookup, nil, logr, portalAuthConfig, passwordResetDelivery)
	portalAuthHandler := internalhandler.NewPortalAuthHandler(portalAuthSvc)

	authRoutes := api.Group("/auth")
	authRoutes.POST("/login", authHandler.Login)
	authRoutes.POST("/refresh", authHandler.Refresh)
	authRoutes.POST("/forgot-password", authHandler.ForgotPassword)
	authRoutes.POST("/reset-password", authHandler.ResetPassword)
	// Logout only revokes the HttpOnly refresh cookie and is safe to call after
	// an access token expires, so it must not be behind the JWT middleware.
	authRoutes.POST("/logout", authHandler.Logout)
	protectedAuth := authRoutes.Group("")
	protectedAuth.Use(internalmiddleware.JWT(authSvc))
	protectedAuth.GET("/me", authHandler.Me)
	protectedAuth.POST("/change-password", authHandler.ChangePassword)

	// Portal Auth Routes (public)
	portalAuthRoutes := api.Group("/portal/auth")
	portalAuthRoutes.POST("/login", portalAuthHandler.PortalLogin)
	portalAuthRoutes.POST("/refresh", portalAuthHandler.PortalRefresh)
	portalAuthRoutes.POST("/forgot-password", portalAuthHandler.PortalForgotPassword)
	portalAuthRoutes.POST("/reset-password", portalAuthHandler.PortalResetPassword)

	// Protected Portal Auth Routes
	protectedPortalAuth := portalAuthRoutes.Group("")
	protectedPortalAuth.Use(internalmiddleware.PortalJWT(portalAuthSvc))
	protectedPortalAuth.GET("/me", portalAuthHandler.PortalMe)
	protectedPortalAuth.POST("/logout", portalAuthHandler.PortalLogout)

	// Portal Profile Routes (protected)
	portalProfileRoutes := api.Group("/portal/profile")
	portalProfileRoutes.Use(internalmiddleware.PortalJWT(portalAuthSvc))
	portalProfileRoutes.GET("", portalAuthHandler.PortalProfile)
	portalProfileRoutes.PUT("/preferences", portalAuthHandler.UpdatePortalPreferences)
	portalProfileRoutes.POST("/device-tokens", portalAuthHandler.RegisterDeviceToken)
	portalProfileRoutes.DELETE("/device-tokens/:tokenId", portalAuthHandler.UnregisterDeviceToken)

	// Portal Parent-Student Link Routes (protected)
	portalParentRoutes := api.Group("/portal/parent")
	portalParentRoutes.Use(internalmiddleware.PortalJWT(portalAuthSvc))
	portalParentRoutes.GET("/students", portalAuthHandler.GetLinkedStudents)
	portalParentRoutes.POST("/students", portalAuthHandler.CreateParentStudentLink)
	portalParentRoutes.PUT("/students/:linkId", portalAuthHandler.UpdateParentStudentLink)
	portalParentRoutes.DELETE("/students/:linkId", portalAuthHandler.DeleteParentStudentLink)

	// Portal Data Routes (protected) - registered after service initialization
	// portalDataRoutes := api.Group("/portal")
	// portalDataRoutes.Use(internalmiddleware.PortalJWT(portalAuthSvc))
	// portalDataRoutes.GET("/grades", portalDataHandler.GetGrades)
	// portalDataRoutes.GET("/grades/report-card", portalDataHandler.GetReportCard)
	// portalDataRoutes.GET("/attendance", portalDataHandler.GetAttendance)
	// portalDataRoutes.GET("/attendance/percentage", portalDataHandler.GetAttendanceStats)
	// portalDataRoutes.GET("/announcements", portalDataHandler.GetAnnouncements)
	// portalDataRoutes.GET("/announcements/:id", portalDataHandler.GetAnnouncementByID)
	// portalDataRoutes.GET("/behavior-notes", portalDataHandler.GetBehaviorNotes)
	// portalDataRoutes.GET("/behavior-notes/summary", portalDataHandler.GetBehaviorSummary)
	// portalDataRoutes.GET("/calendar", portalDataHandler.GetCalendarEvents)
	// portalDataRoutes.GET("/calendar/upcoming", portalDataHandler.GetUpcomingEvents)

	classRepo := repository.NewClassRepository(db)
	classSubjectRepo := repository.NewClassSubjectRepository(db)
	subjectRepo := repository.NewSubjectRepository(db)
	termRepo := repository.NewTermRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	assignmentRepo := repository.NewTeacherAssignmentRepository(db)
	homeroomRepo := repository.NewHomeroomRepository(db)
	preferenceRepo := repository.NewTeacherPreferenceRepository(db)
	calendarRepo := repository.NewCalendarRepository(db)
	enrollmentRepo := repository.NewEnrollmentRepository(db)
	gradeRepo := repository.NewGradeRepository(db)
	gradeFinalRepo := repository.NewGradeFinalRepository(db)
	gradeConfigRepo := repository.NewGradeConfigRepository(db)
	gradeComponentRepo := repository.NewGradeComponentRepository(db)
	announcementRepo := repository.NewAnnouncementRepository(db)
	behaviorRepo := repository.NewBehaviorRepository(db)
	semesterScheduleRepo := repository.NewSemesterScheduleRepository(db)
	semesterSlotRepo := repository.NewSemesterScheduleSlotRepository(db)
	configurationRepo := repository.NewConfigurationRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	// Initialize portal data repositories
	dailyAttendanceRepo := repository.NewDailyAttendanceRepository(db)

	// Portal Data Services
	portalGradesSvc := service.NewPortalGradesService(enrollmentRepo, gradeFinalRepo, studentRepo, nil, logr)
	portalAttendanceSvc := service.NewPortalAttendanceService(dailyAttendanceRepo, enrollmentRepo, studentRepo, nil, logr)
	portalAnnouncementsSvc := service.NewPortalAnnouncementsService(announcementRepo, enrollmentRepo, studentRepo, nil, logr)
	portalBehaviorSvc := service.NewPortalBehaviorService(behaviorRepo, enrollmentRepo, studentRepo, nil, logr)
	portalCalendarSvc := service.NewPortalCalendarService(calendarRepo, enrollmentRepo, studentRepo, nil, logr)
	portalHomeroomSvc := service.NewPortalHomeroomService(homeroomRepo, enrollmentRepo, studentRepo, termRepo, nil, logr)
	portalDataHandler := internalhandler.NewPortalDataHandler(
		portalGradesSvc,
		portalAttendanceSvc,
		portalAnnouncementsSvc,
		portalBehaviorSvc,
		portalCalendarSvc,
		portalHomeroomSvc,
		portalLookup,
	)

	// Register Portal Data Routes
	portalDataRoutes := api.Group("/portal")
	portalDataRoutes.Use(internalmiddleware.PortalJWT(portalAuthSvc))
	portalDataRoutes.GET("/grades", portalDataHandler.GetGrades)
	portalDataRoutes.GET("/grades/report-card", portalDataHandler.GetReportCard)
	portalDataRoutes.GET("/attendance", portalDataHandler.GetAttendance)
	portalDataRoutes.GET("/attendance/percentage", portalDataHandler.GetAttendanceStats)
	portalDataRoutes.GET("/announcements", portalDataHandler.GetAnnouncements)
	portalDataRoutes.GET("/announcements/:id", portalDataHandler.GetAnnouncementByID)
	portalDataRoutes.GET("/behavior-notes", portalDataHandler.GetBehaviorNotes)
	portalDataRoutes.GET("/behavior-notes/summary", portalDataHandler.GetBehaviorSummary)
	portalDataRoutes.GET("/calendar", portalDataHandler.GetCalendarEvents)
	portalDataRoutes.GET("/calendar/upcoming", portalDataHandler.GetUpcomingEvents)
	portalDataRoutes.GET("/homeroom", portalDataHandler.GetHomeroom)

	userSvc := service.NewUserService(authRepo, nil, logr)
	userHandler := internalhandler.NewUserHandler(userSvc)
	termSvc := service.NewTermService(termRepo, nil, logr)
	termHandler := internalhandler.NewTermHandler(termSvc)
	subjectSvc := service.NewSubjectService(subjectRepo, nil, logr)
	subjectHandler := internalhandler.NewSubjectHandler(subjectSvc)
	classSvc := service.NewClassService(classRepo, subjectRepo, classSubjectRepo, nil, logr)
	classHandler := internalhandler.NewClassHandler(classSvc)
	classSubjectHandler := internalhandler.NewClassSubjectHandler(classSvc)
	scheduleSvc := service.NewScheduleService(scheduleRepo, nil, logr)
	scheduleHandler := internalhandler.NewScheduleHandler(scheduleSvc)
	studentSvc := service.NewStudentService(studentRepo, nil, logr)
	importRunRepo := repository.NewImportRunRepository(db)
	studentHandler := internalhandler.NewStudentHandler(studentSvc, importRunRepo)
	enrollmentSvc := service.NewEnrollmentService(enrollmentRepo, studentRepo, classRepo, termRepo, nil, logr)
	enrollmentHandler := internalhandler.NewEnrollmentHandler(enrollmentSvc)
	gradeComponentSvc := service.NewGradeComponentService(gradeComponentRepo, nil, logr)
	gradeComponentHandler := internalhandler.NewGradeComponentHandler(gradeComponentSvc)
	gradeConfigSvc := service.NewGradeConfigService(gradeConfigRepo, gradeComponentRepo, nil, logr)
	gradeConfigHandler := internalhandler.NewGradeConfigHandler(gradeConfigSvc)
	gradeSvc := service.NewGradeService(gradeRepo, gradeFinalRepo, enrollmentRepo, gradeConfigRepo, gradeComponentRepo, nil, logr)
	gradeHandler := internalhandler.NewGradeHandler(gradeSvc)
	exportCompatibilityHandler := internalhandler.NewExportCompatibilityHandler(db)
	announcementSvc := service.NewAnnouncementService(announcementRepo, nil, logr)
	announcementHandler := internalhandler.NewAnnouncementHandler(announcementSvc)
	behaviorSvc := service.NewBehaviorService(behaviorRepo, nil, logr)
	behaviorHandler := internalhandler.NewBehaviorHandler(behaviorSvc)
	teacherSvc := service.NewTeacherService(teacherRepo, nil, logr)
	calendarSvc := service.NewCalendarService(calendarRepo, nil, logr)
	calendarHandler := internalhandler.NewCalendarHandler(calendarSvc)
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
	teacherHandler := internalhandler.NewTeacherHandler(teacherSvc, assignmentSvc, preferenceSvc, importRunRepo)
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

	if attendanceSvc != nil && attendanceSummaryRepo != nil {
		attendanceAliasSvc := service.NewAttendanceAliasService(attendanceSvc, analyticsSvc, attendanceSummaryRepo, assignmentRepo, enrollmentRepo, termRepo, logr)
		attendanceAliasHandler = internalhandler.NewAttendanceAliasHandler(attendanceAliasSvc)
	}

	// CRUD Attendance Handler (always available)
	var attendanceHandler *internalhandler.AttendanceHandler
	if attendanceSvc != nil {
		attendanceHandler = internalhandler.NewAttendanceHandler(attendanceSvc)
	}

	// Teacher Preferences Handler
	teacherPreferenceHandler := internalhandler.NewTeacherPreferenceHandler(preferenceSvc)

	// Audit trail reads are always available: entries are written unconditionally
	// by the middleware and services, so gating the viewer would only hide data
	// that already exists.
	auditHandler := internalhandler.NewAuditHandler(service.NewAuditService(auditRepo, logr))

	reportHandler := internalhandler.NewReportHandler(nil, gradeSvc)
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
		reportHandler = internalhandler.NewReportHandler(reportSvc, gradeSvc)
	}

	var mutationHandler *internalhandler.MutationHandler
	if cfg.Mutations.Enabled {
		mutationRepo := repository.NewMutationRepository(db)
		mutationSvc := service.NewMutationService(mutationRepo, authRepo, logr, service.WithMutationAppliers(map[string]service.MutationApplier{
			"student": service.NewStudentMutationApplier(studentRepo, logr),
		}))
		mutationHandler = internalhandler.NewMutationHandler(mutationSvc)
	}

	var archivesHandler *archives.ArchiveHandler
	if cfg.Archives.Enabled {
		if cfg.Archives.SignedURLSecret == "" {
			logr.Sugar().Fatal("archives signed url secret not configured")
		}
		var arcRepo archives.Repository
		if db != nil {
			arcRepo = archives.NewPostgresRepository(db)
		} else {
			arcRepo = archives.NewMemoryRepository()
		}
		arcSearchEngine := archives.NewMemorySearchEngine()
		arcWorkerPool := archives.NewGoOCRWorkerPool(4, 100, arcRepo, arcSearchEngine)
		arcSigner := archives.NewHMACSignedURLSigner(cfg.Archives.SignedURLSecret, cfg.APIPrefix+"/archives")
		arcRetentionEngine := archives.NewRetentionEngine(arcRepo)
		arcSvc := archives.NewArchiveService(arcRepo, arcSearchEngine, arcWorkerPool, arcSigner, arcRetentionEngine)
		_ = arcSvc.StartBackgroundTasks(context.Background())
		archivesHandler = archives.NewArchiveHandler(arcSvc)
	}

	secured := api.Group("")
	secured.Use(internalmiddleware.JWT(authSvc))

	usersGroup := secured.Group("/users")
	usersGroup.GET("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), userHandler.List)
	usersGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), userHandler.Create)
	usersGroup.GET("/:id", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), userHandler.Get)
	usersGroup.PUT("/:id", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), userHandler.Update)
	usersGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), userHandler.Delete)

	termsGroup := secured.Group("/terms")
	termsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), termHandler.List)
	termsGroup.GET("/active", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), termHandler.GetActive)
	termsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), termHandler.Create)
	termsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), termHandler.Update)
	termsGroup.POST("/set-active", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), termHandler.SetActive)
	termsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), termHandler.Delete)

	subjectsGroup := secured.Group("/subjects")
	subjectsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), subjectHandler.List)
	subjectsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), subjectHandler.Create)
	subjectsGroup.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), subjectHandler.Get)
	subjectsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), subjectHandler.Update)
	subjectsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), subjectHandler.Delete)

	classesGroup := secured.Group("/classes")
	classesGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), classHandler.List)
	classesGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), classHandler.Create)
	classesGroup.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), classHandler.Get)
	classesGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), classHandler.Update)
	classesGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), classHandler.Delete)
	classesGroup.GET("/:id/subjects", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), classSubjectHandler.List)
	classesGroup.POST("/:id/subjects", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), classSubjectHandler.Assign)
	classesGroup.GET("/:id/schedules", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), scheduleHandler.ListByClass)

	// Standalone class-subjects endpoint for frontend list views
	classSubjectsGroup := secured.Group("/class-subjects")
	classSubjectsGroup.Use(internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)))
	classSubjectsGroup.GET("", classSubjectHandler.ListAll)

	schedulesGroup := secured.Group("/schedules")
	schedulesGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), scheduleHandler.List)
	schedulesGroup.GET("/export/pdf", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), scheduleHandler.ExportPDF) // GET /schedules/export/pdf
	schedulesGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), scheduleHandler.Create)
	schedulesGroup.POST("/bulk", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), scheduleHandler.BulkCreate)
	schedulesGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), scheduleHandler.Update)
	schedulesGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), scheduleHandler.Delete)

	studentsGroup := secured.Group("/students")
	studentsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), studentHandler.List)
	studentsGroup.GET("/roster", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), studentHandler.Roster)
	studentsGroup.POST("/import", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), studentHandler.ImportCSV)
	studentsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), studentHandler.Create)
	studentsGroup.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), studentHandler.Get)
	studentsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), studentHandler.Update)
	studentsGroup.PATCH("/:id/status", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), studentHandler.UpdateStatus)
	studentsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), studentHandler.Delete)
	studentsGroup.GET("/:id/behavior-summary", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), behaviorHandler.Summary)

	enrollmentsGroup := secured.Group("/enrollments")
	enrollmentsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), enrollmentHandler.List)
	enrollmentsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), enrollmentHandler.Create)
	enrollmentsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), enrollmentHandler.Update)
	enrollmentsGroup.PUT("/:id/transfer", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), enrollmentHandler.Transfer)
	enrollmentsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), enrollmentHandler.Delete)

	gradeComponentsGroup := secured.Group("/grade-components")
	gradeComponentsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeComponentHandler.List)
	gradeComponentsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeComponentHandler.Create)
	gradeComponentsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeComponentHandler.Update)
	gradeComponentsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeComponentHandler.Delete)

	gradeConfigsGroup := secured.Group("/grade-configs")
	gradeConfigsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeConfigHandler.List)
	gradeConfigsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeConfigHandler.Create)
	gradeConfigsGroup.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeConfigHandler.Get)
	gradeConfigsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeConfigHandler.Update)
	gradeConfigsGroup.POST("/:id/finalize", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeConfigHandler.Finalize)

	gradesGroup := secured.Group("/grades")
	gradesGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.List)
	gradesGroup.GET("/report", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Report)
	gradesGroup.POST("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Upsert)
	gradesGroup.PATCH("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Update)
	gradesGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Update)
	gradesGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Delete)
	gradesGroup.POST("/bulk", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Bulk)
	gradesGroup.POST("/recalculate", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Recalculate)
	gradesGroup.POST("/finalize", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), gradeHandler.Finalize)

	announcementsGroup := secured.Group("/announcements")
	announcementsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), announcementHandler.List)
	announcementsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), announcementHandler.Create)
	announcementsGroup.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), announcementHandler.Get)
	announcementsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), announcementHandler.Update)
	announcementsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), announcementHandler.Delete)

	behaviorGroup := secured.Group("/behavior-notes")
	behaviorGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), behaviorHandler.List)
	behaviorGroup.POST("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), behaviorHandler.Create)
	behaviorGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), behaviorHandler.Update)
	behaviorGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), behaviorHandler.Delete)

	calendarEventsGroup := secured.Group("/calendar-events")
	calendarEventsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.List)
	calendarEventsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.Create)
	calendarEventsGroup.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.Get)
	calendarEventsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.Update)
	calendarEventsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), calendarHandler.Delete)

	// Backward-compatible alias for the admin calendar. Exam events are stored
	// as calendar events; clients distinguish them through their event_type.
	examEventsGroup := secured.Group("/exam-events")
	examEventsGroup.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.List)
	examEventsGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.Create)
	examEventsGroup.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.Get)
	examEventsGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarHandler.Update)
	examEventsGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), calendarHandler.Delete)

	reportJSONGroup := secured.Group("/reports")
	reportJSONGroup.GET("/students/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), reportHandler.StudentReport)
	reportJSONGroup.GET("/classes/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), reportHandler.ClassReport)
	exportCompatibilityGroup := secured.Group("/export")
	exportCompatibilityGroup.Use(internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)))
	exportCompatibilityGroup.GET("/students", exportCompatibilityHandler.Students)
	exportCompatibilityGroup.GET("/grades", exportCompatibilityHandler.Grades)
	exportCompatibilityGroup.GET("/attendance", exportCompatibilityHandler.Attendance)

	teachersGroup := secured.Group("/teachers")
	teachersGroup.GET("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.List)
	teachersGroup.GET("/roster", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.Roster)
	teachersGroup.POST("/import", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.ImportCSV)
	teachersGroup.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.Create)
	teachersGroup.GET("/:id", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.Get)
	teachersGroup.PUT("/:id", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.Update)
	teachersGroup.PATCH("/:id/status", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.UpdateStatus)
	teachersGroup.DELETE("/:id", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), teacherHandler.Delete)
	teachersGroup.GET("/:id/assignments", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.ListAssignments)
	teachersGroup.POST("/:id/assignments", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.CreateAssignment)
	teachersGroup.DELETE("/:id/assignments/:aid", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.DeleteAssignment)
	teachersGroup.GET("/:id/preferences", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.GetPreferences)
	teachersGroup.PUT("/:id/preferences", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), teacherHandler.UpsertPreferences)
	teachersGroup.GET("/:id/schedules", internalmiddleware.RBAC("SELF", string(models.RoleAdmin), string(models.RoleSuperAdmin)), scheduleHandler.ListByTeacher)

	if calendarAliasHandler != nil {
		secured.GET("/calendar", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), calendarAliasHandler.List)
	}

	if attendanceAliasHandler != nil {
		attendanceGroup := secured.Group("/attendance")
		attendanceGroup.Use(internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)))
		attendanceGroup.GET("", attendanceAliasHandler.Summary)
		attendanceGroup.GET("/daily", attendanceAliasHandler.Daily)
	}

	// CRUD Attendance endpoints
	if attendanceHandler != nil {
		attendanceCRUDGroup := secured.Group("/attendance")
		attendanceCRUDGroup.Use(internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)))
		attendanceCRUDGroup.POST("/daily", attendanceHandler.MarkDaily)
		attendanceCRUDGroup.POST("/daily/bulk", attendanceHandler.BulkMarkDaily)
		attendanceCRUDGroup.POST("/subject", attendanceHandler.MarkSubject)
		attendanceCRUDGroup.POST("/subject/bulk", attendanceHandler.BulkMarkSubject)
		// Lesson-level (per-subject) reads. The admin panel's lesson attendance
		// screen needs to load an existing session and a class-subject history,
		// which previously had no backend counterpart.
		attendanceCRUDGroup.GET("/subject", attendanceHandler.ListSubject)
		attendanceCRUDGroup.GET("/subject/summary", attendanceHandler.SubjectSummary)
		attendanceCRUDGroup.GET("/subject/:id", attendanceHandler.GetSubject)
		attendanceCRUDGroup.DELETE("/subject/:id", attendanceHandler.DeleteSubject)
		attendanceCRUDGroup.POST("", attendanceHandler.LegacyUpsert)
		attendanceCRUDGroup.PUT("/:id", attendanceHandler.LegacyUpsert)
		attendanceCRUDGroup.PATCH("/:id", attendanceHandler.LegacyUpsert)
	}

	// Standalone teacher-preferences endpoint
	teacherPrefsGroup := secured.Group("/teacher-preferences")
	teacherPrefsGroup.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)))
	teacherPrefsGroup.GET("", teacherPreferenceHandler.ListAll)
	teacherPrefsGroup.POST("", teacherPreferenceHandler.LegacyUpsert)
	teacherPrefsGroup.PUT("/:id", teacherPreferenceHandler.LegacyUpsert)

	// Audit trail viewer. Reads are restricted to SUPERADMIN because entries
	// expose cross-tenant actor identity and request metadata.
	auditGroup := secured.Group("/audit-logs")
	auditGroup.Use(internalmiddleware.RBAC(string(models.RoleSuperAdmin)))
	auditGroup.GET("", auditHandler.List)
	auditGroup.GET("/facets", auditHandler.Facets)
	auditGroup.GET("/:id", auditHandler.Get)

	if configurationHandler != nil {
		configGroup := secured.Group("/configuration")
		configGroup.Use(internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)))
		configGroup.GET("", configurationHandler.List)
		configGroup.GET("/:key", configurationHandler.Get)
		configGroup.PUT("/:key", func(c *gin.Context) {
			if c.Param("key") == "bulk" {
				configurationHandler.BulkUpdate(c)
				return
			}
			configurationHandler.Update(c)
		})
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
		schedulesGroup.GET("/preferences", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulePreferenceHandler.Get)
		schedulesGroup.POST("/preferences", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), schedulePreferenceHandler.Upsert)
	}

	if cfg.Reports.Enabled {
		reportJSONGroup.POST("/generate", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), reportHandler.GenerateReport)
		reportJSONGroup.GET("/status/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), reportHandler.ReportStatus)
		secured.GET("/export/:token", reportHandler.DownloadReport)
	}

	if mutationHandler != nil {
		mutations := secured.Group("/mutations")
		mutations.POST("", internalmiddleware.RBAC(string(models.RoleAdmin), string(models.RoleSuperAdmin)), mutationHandler.Create)
		mutations.GET("", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), mutationHandler.List)
		mutations.GET("/:id", internalmiddleware.RBAC(string(models.RoleTeacher), string(models.RoleAdmin), string(models.RoleSuperAdmin)), mutationHandler.Get)
		mutations.POST("/:id/review", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), mutationHandler.Review)
		mutations.PATCH("/:id/approve", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), mutationHandler.Approve)
		mutations.PATCH("/:id/reject", internalmiddleware.RBAC(string(models.RoleSuperAdmin)), mutationHandler.Reject)
	}

	if archivesHandler != nil {
		archivesGroup := secured.Group("/archives")
		archivesHandler.RegisterRoutes(archivesGroup)

		documentsGroup := secured.Group("/documents")
		archivesHandler.RegisterRoutes(documentsGroup)
	}

	if cfg.Dashboard.Enabled {
		dashboardCache := service.NewCacheService(cacheRepo, metricsSvc, cfg.Dashboard.CacheTTL, logr, cacheRepo != nil)
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
		dashboardHandler := internalhandler.NewDashboardHandler(dashboardSvc, teacherRepo)

		dashboardGroup := secured.Group("")
		dashboardGroup.Use(internalmiddleware.WithResponseMeta())
		// The payload is school-wide aggregate attendance, grades, behaviour, and
		// operations, which is precisely a principal's view, so KEPALA_SEKOLAH
		// reads it too. It exposes no per-student detail beyond the leaderboards
		// already available to that role through /behavior-notes.
		dashboardGroup.GET("/dashboard", internalmiddleware.RBAC(string(models.RoleKepalaSekolah), string(models.RoleAdmin), string(models.RoleSuperAdmin)), dashboardHandler.Admin)
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
