package logger

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// PlainStyledLogger implements StyledLogger without formatting
type PlainStyledLogger struct {
	logger *slog.Logger
}

func NewPlainStyledLogger(logger *slog.Logger) *PlainStyledLogger {
	return &PlainStyledLogger{
		logger: logger,
	}
}
func (sl *PlainStyledLogger) ResetLine() {
	fmt.Print("\033[1A\033[2K")
}
func (sl *PlainStyledLogger) InfoWithStatus(msg string, status string, args ...any) {
	styledMsg := fmt.Sprintf("[ %s ] %s", status, msg)
	sl.logger.Info(styledMsg, args...)
}
func (sl *PlainStyledLogger) Debug(msg string, args ...any) {
	sl.logger.Debug(msg, args...)
}

func (sl *PlainStyledLogger) Info(msg string, args ...any) {
	sl.logger.Info(msg, args...)
}

func (sl *PlainStyledLogger) Warn(msg string, args ...any) {
	sl.logger.Warn(msg, args...)
}

func (sl *PlainStyledLogger) Error(msg string, args ...any) {
	sl.logger.Error(msg, args...)
}

