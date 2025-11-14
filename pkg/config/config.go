package config

import (
	"errors"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

type Config struct {
	Env       string
	Port      int
	APIPrefix string

	Database      DatabaseConfig
	Redis         RedisConfig
	JWT           JWTConfig
	CORS          CORSConfig
	Log           LogConfig
	Analytics     AnalyticsConfig
	Dashboard     DashboardConfig
	Cutover       CutoverConfig
	Scheduler     SchedulerConfig
	Reports       ReportsConfig
	Mutations     MutationsConfig
	Archives      ArchivesConfig
	Homerooms     HomeroomConfig
	Aliases       AliasConfig
	Configuration ConfigurationAPIConfig
}

type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type JWTConfig struct {
	Secret            string
	Expiration        time.Duration
	RefreshExpiration time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

type LogConfig struct {
	Level  string
	Format string
}

// ReportsConfig configures asynchronous report generation.
type ReportsConfig struct {
	Enabled           bool
	StorageDir        string
	SignedURLSecret   string
	SignedURLTTL      time.Duration
	CleanupInterval   time.Duration
	WorkerConcurrency int
	WorkerRetries     int
}

// MutationsConfig toggles workflow exposure.
type MutationsConfig struct {
	Enabled bool
}

// ArchivesConfig controls archive storage & validation.
type ArchivesConfig struct {
	Enabled          bool
	StorageDir       string
	SignedURLSecret  string
	SignedURLTTL     time.Duration
	MaxFileSizeBytes int64
	AllowedMIMEs     []string
}

// HomeroomConfig gates the homeroom management endpoints.
type HomeroomConfig struct {
	Enabled bool
}

// AliasConfig toggles thin alias endpoints for existing modules.
type AliasConfig struct {
	CalendarEnabled   bool
	AttendanceEnabled bool
}

// ConfigurationAPIConfig toggles the configuration admin API.
type ConfigurationAPIConfig struct {
	Enabled                bool
	ActiveTermID           string
	DefaultDashboardTermID string
	DefaultCalendarTermID  string
}

// SchedulerConfig toggles the constraint-based schedule generator.
type SchedulerConfig struct {
	Enabled     bool
	ProposalTTL time.Duration
}

// AnalyticsConfig governs feature flagging and cache behaviour for analytics endpoints.
type AnalyticsConfig struct {
	Enabled  bool
	CacheTTL time.Duration
}

// DashboardConfig governs dashboard exposure and cache tuning.
type DashboardConfig struct {
	Enabled  bool
	CacheTTL time.Duration
}

// CutoverConfig defines feature flags and routing controls for the legacy decommission.
type CutoverConfig struct {
	RouteToGo           bool
	ShadowTraffic       bool
	LegacyReadOnly      bool
	CanaryPercentage    int
	StageHeader         string
	ClientSegmentHeader string
	LegacyHealthURL     string
	GoHealthURL         string
	HealthCheckTimeout  time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
	}

	cfg := &Config{}

	cfg.Env = v.GetString("ENV")
	cfg.Port = v.GetInt("PORT")
	cfg.APIPrefix = v.GetString("API_PREFIX")

	cfg.Database = DatabaseConfig{
		Host:         v.GetString("DB_HOST"),
		Port:         v.GetInt("DB_PORT"),
		User:         v.GetString("DB_USER"),
		Password:     v.GetString("DB_PASSWORD"),
		Name:         v.GetString("DB_NAME"),
		SSLMode:      v.GetString("DB_SSL_MODE"),
		MaxOpenConns: v.GetInt("DB_MAX_OPEN_CONNS"),
		MaxIdleConns: v.GetInt("DB_MAX_IDLE_CONNS"),
	}

	cfg.Redis = RedisConfig{
		Host:     v.GetString("REDIS_HOST"),
		Port:     v.GetInt("REDIS_PORT"),
		Password: v.GetString("REDIS_PASSWORD"),
		DB:       v.GetInt("REDIS_DB"),
	}

	cfg.JWT = JWTConfig{
		Secret:            v.GetString("JWT_SECRET"),
		Expiration:        parseDuration(v.GetString("JWT_EXPIRATION"), 24*time.Hour),
		RefreshExpiration: parseDuration(v.GetString("REFRESH_TOKEN_EXPIRATION"), 7*24*time.Hour),
	}

	cfg.CORS = CORSConfig{AllowedOrigins: splitAndTrim(v.GetString("ALLOWED_ORIGINS"))}

	cfg.Log = LogConfig{
		Level:  v.GetString("LOG_LEVEL"),
		Format: v.GetString("LOG_FORMAT"),
	}

	cfg.Analytics = AnalyticsConfig{
		Enabled:  v.GetBool("ENABLE_ANALYTICS"),
		CacheTTL: parseDuration(v.GetString("ANALYTICS_CACHE_TTL"), 10*time.Minute),
	}

	cfg.Dashboard = DashboardConfig{
		Enabled:  v.GetBool("ENABLE_DASHBOARD"),
		CacheTTL: parseDuration(v.GetString("DASHBOARD_CACHE_TTL"), 5*time.Minute),
	}

	cfg.Scheduler = SchedulerConfig{
		Enabled:     v.GetBool("ENABLE_SCHEDULER"),
		ProposalTTL: parseDuration(v.GetString("SCHEDULER_PROPOSAL_TTL"), 30*time.Minute),
	}

	cfg.Cutover = CutoverConfig{
		RouteToGo:           v.GetBool("ROUTE_TO_GO"),
		ShadowTraffic:       v.GetBool("SHADOW_TRAFFIC"),
		LegacyReadOnly:      v.GetBool("LEGACY_READONLY"),
		CanaryPercentage:    v.GetInt("CANARY_PERCENTAGE"),
		StageHeader:         v.GetString("CUTOVER_STAGE_HEADER"),
		ClientSegmentHeader: v.GetString("CUTOVER_SEGMENT_HEADER"),
		LegacyHealthURL:     v.GetString("LEGACY_HEALTH_URL"),
		GoHealthURL:         v.GetString("GO_HEALTH_URL"),
		HealthCheckTimeout:  parseDuration(v.GetString("CUTOVER_HEALTH_TIMEOUT"), 2*time.Second),
	}

	cfg.Reports = ReportsConfig{
		Enabled:           v.GetBool("ENABLE_REPORTS"),
		StorageDir:        v.GetString("REPORTS_STORAGE_DIR"),
		SignedURLSecret:   v.GetString("REPORTS_SIGNED_URL_SECRET"),
		SignedURLTTL:      parseDuration(v.GetString("REPORTS_SIGNED_URL_TTL"), 24*time.Hour),
		CleanupInterval:   parseDuration(v.GetString("REPORTS_CLEANUP_INTERVAL"), time.Hour),
		WorkerConcurrency: v.GetInt("REPORTS_WORKER_CONCURRENCY"),
		WorkerRetries:     v.GetInt("REPORTS_WORKER_RETRIES"),
	}

	cfg.Mutations = MutationsConfig{
		Enabled: v.GetBool("ENABLE_MUTATIONS"),
	}

	maxArchiveSize := v.GetInt64("ARCHIVES_MAX_FILE_SIZE")
	if maxArchiveSize <= 0 {
		maxArchiveSize = 10 * 1024 * 1024
	}
	cfg.Archives = ArchivesConfig{
		Enabled:          v.GetBool("ENABLE_ARCHIVES"),
		StorageDir:       v.GetString("ARCHIVES_STORAGE_DIR"),
		SignedURLSecret:  v.GetString("ARCHIVES_SIGNED_URL_SECRET"),
		SignedURLTTL:     parseDuration(v.GetString("ARCHIVES_SIGNED_URL_TTL"), 30*time.Minute),
		MaxFileSizeBytes: maxArchiveSize,
		AllowedMIMEs:     splitAndTrim(v.GetString("ARCHIVES_ALLOWED_MIME_TYPES")),
	}

	cfg.Homerooms = HomeroomConfig{
		Enabled: v.GetBool("ENABLE_HOMEROOMS"),
	}

	cfg.Aliases = AliasConfig{
		CalendarEnabled:   v.GetBool("ENABLE_CALENDAR_ALIAS"),
		AttendanceEnabled: v.GetBool("ENABLE_ATTENDANCE_ALIAS"),
	}

	cfg.Configuration = ConfigurationAPIConfig{
		Enabled:                v.GetBool("ENABLE_CONFIGURATION_API"),
		ActiveTermID:           v.GetString("CONFIG_ACTIVE_TERM_ID"),
		DefaultDashboardTermID: v.GetString("CONFIG_DEFAULT_DASHBOARD_TERM_ID"),
		DefaultCalendarTermID:  v.GetString("CONFIG_DEFAULT_CALENDAR_TERM_ID"),
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("ENV", EnvDevelopment)
	v.SetDefault("PORT", 8080)
	v.SetDefault("API_PREFIX", "/api/v1")

	v.SetDefault("DB_HOST", "localhost")
	v.SetDefault("DB_PORT", 5432)
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_PASSWORD", "postgres")
	v.SetDefault("DB_NAME", "admin_panel_sma")
	v.SetDefault("DB_SSL_MODE", "disable")
	v.SetDefault("DB_MAX_OPEN_CONNS", 10)
	v.SetDefault("DB_MAX_IDLE_CONNS", 5)

	v.SetDefault("REDIS_HOST", "localhost")
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_PASSWORD", "")
	v.SetDefault("REDIS_DB", 0)

	v.SetDefault("JWT_SECRET", "dev_secret")
	v.SetDefault("JWT_EXPIRATION", "24h")
	v.SetDefault("REFRESH_TOKEN_EXPIRATION", "168h")

	v.SetDefault("ALLOWED_ORIGINS", "")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")

	v.SetDefault("ENABLE_ANALYTICS", false)
	v.SetDefault("ANALYTICS_CACHE_TTL", "10m")
	v.SetDefault("ENABLE_DASHBOARD", false)
	v.SetDefault("DASHBOARD_CACHE_TTL", "5m")

	v.SetDefault("ENABLE_SCHEDULER", false)
	v.SetDefault("SCHEDULER_PROPOSAL_TTL", "30m")

	v.SetDefault("ROUTE_TO_GO", false)
	v.SetDefault("SHADOW_TRAFFIC", false)
	v.SetDefault("LEGACY_READONLY", false)
	v.SetDefault("CANARY_PERCENTAGE", 0)
	v.SetDefault("CUTOVER_STAGE_HEADER", "X-Cutover-Stage")
	v.SetDefault("CUTOVER_SEGMENT_HEADER", "X-Client-Segment")
	v.SetDefault("LEGACY_HEALTH_URL", "http://localhost:3000/health")
	v.SetDefault("GO_HEALTH_URL", "http://localhost:8080/health")
	v.SetDefault("CUTOVER_HEALTH_TIMEOUT", "2s")

	v.SetDefault("ENABLE_REPORTS", false)
	v.SetDefault("REPORTS_STORAGE_DIR", "./exports")
	v.SetDefault("REPORTS_SIGNED_URL_SECRET", "dev_reports_secret")
	v.SetDefault("REPORTS_SIGNED_URL_TTL", "24h")
	v.SetDefault("REPORTS_CLEANUP_INTERVAL", "1h")
	v.SetDefault("REPORTS_WORKER_CONCURRENCY", 1)
	v.SetDefault("REPORTS_WORKER_RETRIES", 3)

	v.SetDefault("ENABLE_MUTATIONS", false)
	v.SetDefault("ENABLE_ARCHIVES", false)
	v.SetDefault("ARCHIVES_STORAGE_DIR", "./archives")
	v.SetDefault("ARCHIVES_SIGNED_URL_SECRET", "dev_archives_secret")
	v.SetDefault("ARCHIVES_SIGNED_URL_TTL", "30m")
	v.SetDefault("ARCHIVES_MAX_FILE_SIZE", 10*1024*1024)
	v.SetDefault("ARCHIVES_ALLOWED_MIME_TYPES", "application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/zip")
	v.SetDefault("ENABLE_HOMEROOMS", false)
	v.SetDefault("ENABLE_CALENDAR_ALIAS", false)
	v.SetDefault("ENABLE_ATTENDANCE_ALIAS", false)
	v.SetDefault("ENABLE_CONFIGURATION_API", false)
	v.SetDefault("CONFIG_ACTIVE_TERM_ID", "")
	v.SetDefault("CONFIG_DEFAULT_DASHBOARD_TERM_ID", "")
	v.SetDefault("CONFIG_DEFAULT_CALENDAR_TERM_ID", "")
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}

	return d
}

func splitAndTrim(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
