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

	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	CORS     CORSConfig
	Log      LogConfig
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
