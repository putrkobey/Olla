package logger

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/pterm/pterm"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/theme"
)

// PrettyStyledLogger implements StyledLogger with pterm formatting
type PrettyStyledLogger struct {
	logger *slog.Logger
	Theme  *theme.Theme
}

func NewPrettyStyledLogger(logger *slog.Logger, theme *theme.Theme) *PrettyStyledLogger {
	return &PrettyStyledLogger{
		logger: logger,
		Theme:  theme,
	}
}
func (sl *PrettyStyledLogger) Debug(msg string, args ...any) {
	sl.logger.Debug(msg, args...)
}
func (sl *PrettyStyledLogger) InfoWithStatus(msg string, status string, args ...any) {
	styledMsg := fmt.Sprintf("[ %s ] %s", sl.Theme.Good.Sprint(status), msg)
	sl.logger.Info(styledMsg, args...)
}

func (sl *PrettyStyledLogger) ResetLine() {
	fmt.Print("\033[1A\033[2K")
}
func (sl *PrettyStyledLogger) Info(msg string, args ...any) {
	sl.logger.Info(msg, args...)
