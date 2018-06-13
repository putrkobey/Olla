package logger

import (
	"log/slog"

	"github.com/putrkobey/Olla/internal/core/domain"

	"github.com/putrkobey/Olla/theme"
)

// StyledLogger interface for different formatting strategies
type StyledLogger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	ResetLine()
	InfoWithStatus(msg string, status string, args ...any)
	InfoWithCount(msg string, count int, args ...any)
	InfoWithEndpoint(msg string, endpoint string, args ...any)
	InfoWithHealthCheck(msg string, endpoint string, args ...any)
	InfoWithNumbers(msg string, numbers ...int64)
	WarnWithEndpoint(msg string, endpoint string, args ...any)
	ErrorWithEndpoint(msg string, endpoint string, args ...any)
	InfoHealthy(msg string, endpoint string, args ...any)
	InfoHealthStatus(msg string, name string, status domain.EndpointStatus, args ...any)
	GetUnderlying() *slog.Logger
	WithRequestID(requestID string) StyledLogger
	InfoConfigChange(oldName, newName string)
	WithAttrs(attrs ...slog.Attr) StyledLogger
	With(args ...any) StyledLogger
	InfoWithContext(msg string, endpoint string, ctx LogContext)
	WarnWithContext(msg string, endpoint string, ctx LogContext)
	ErrorWithContext(msg string, endpoint string, ctx LogContext)
}

func NewStyledLogger(logger *slog.Logger, theme *theme.Theme, prettyMode bool) StyledLogger {
	if prettyMode {
		return NewPrettyStyledLogger(logger, theme)
	}
	return NewPlainStyledLogger(logger)
}

func NewWithTheme(cfg *Config) (*slog.Logger, StyledLogger, func(), error) {
	logger, cleanup, err := New(cfg)
	if err != nil {
		return nil, nil, nil, err
	}

	appTheme := theme.GetTheme(cfg.Theme)
	styledLogger := NewStyledLogger(logger, appTheme, cfg.PrettyLogs)

	return logger, styledLogger, cleanup, nil
}

