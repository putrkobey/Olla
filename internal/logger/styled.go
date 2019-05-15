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
