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
}

func (sl *PrettyStyledLogger) Warn(msg string, args ...any) {
	sl.logger.Warn(msg, args...)
}

func (sl *PrettyStyledLogger) Error(msg string, args ...any) {
	sl.logger.Error(msg, args...)
}

func (sl *PrettyStyledLogger) InfoWithCount(msg string, count int, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, sl.Theme.Counts.Sprint("(", count, ")"))
	sl.logger.Info(styledMsg, args...)
}

func (sl *PrettyStyledLogger) InfoWithEndpoint(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, sl.Theme.Endpoint.Sprint(endpoint))
	sl.logger.Info(styledMsg, args...)
}

func (sl *PrettyStyledLogger) InfoWithHealthCheck(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, sl.Theme.HealthCheck.Sprint(endpoint))
	sl.logger.Info(styledMsg, args...)
}

func (sl *PrettyStyledLogger) InfoWithNumbers(msg string, numbers ...int64) {
	var formattedNums []string
	for _, num := range numbers {
		formattedNums = append(formattedNums, sl.Theme.Numbers.Sprint(num))
	}

	styledMsg := fmt.Sprintf(msg, toInterfaceSlice(formattedNums)...)
	sl.logger.Info(styledMsg)
}

func (sl *PrettyStyledLogger) WarnWithEndpoint(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, sl.Theme.Endpoint.Sprint(endpoint))
	sl.logger.Warn(styledMsg, args...)
}

