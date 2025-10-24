package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/noah-isme/sma-adp-api/pkg/config"
	"github.com/noah-isme/sma-adp-api/pkg/middleware/requestid"
)

func New(cfg *config.Config) (*zap.Logger, error) {
	var zapCfg zap.Config
	if cfg.Env == config.EnvProduction {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
	}

	switch cfg.Log.Format {
	case "console":
		zapCfg.Encoding = "console"
	default:
		zapCfg.Encoding = "json"
	}

	if cfg.Log.Level != "" {
		if err := zapCfg.Level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
			zapCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		}
	}

	zapCfg.EncoderConfig.TimeKey = "timestamp"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return zapCfg.Build()
}

func GinMiddleware(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		reqID := requestid.Value(c)

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
		}
		if reqID != "" {
			fields = append(fields, zap.String("request_id", reqID))
		}

		l.Info("http_request", fields...)
	}
}
