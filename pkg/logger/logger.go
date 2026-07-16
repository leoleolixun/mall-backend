package logger

import (
	"fmt"
	"strings"

	"go-mall/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(cfg config.LogConfig) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if value := strings.TrimSpace(cfg.Level); value != "" {
		if err := level.Set(value); err != nil {
			return nil, fmt.Errorf("无效日志级别 %q: %w", value, err)
		}
	}

	var zapCfg zap.Config
	if strings.EqualFold(strings.TrimSpace(cfg.Format), "console") {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.OutputPaths = []string{"stdout"}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	return zapCfg.Build()
}
