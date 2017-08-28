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

func (sl *PlainStyledLogger) InfoWithCount(msg string, count int, args ...any) {
	styledMsg := fmt.Sprintf("%s (%d)", msg, count)
	sl.logger.Info(styledMsg, args...)
}

func (sl *PlainStyledLogger) InfoWithEndpoint(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, endpoint)
	sl.logger.Info(styledMsg, args...)
}

func (sl *PlainStyledLogger) InfoWithHealthCheck(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, endpoint)
	sl.logger.Info(styledMsg, args...)
}

func (sl *PlainStyledLogger) InfoWithNumbers(msg string, numbers ...int64) {
	var formattedNums []string
	for _, num := range numbers {
		formattedNums = append(formattedNums, strconv.FormatInt(num, 10))
	}

	styledMsg := fmt.Sprintf(msg, toInterfaceSlice(formattedNums)...)
	sl.logger.Info(styledMsg)
}

func (sl *PlainStyledLogger) WarnWithEndpoint(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, endpoint)
	sl.logger.Warn(styledMsg, args...)
}

func (sl *PlainStyledLogger) ErrorWithEndpoint(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, endpoint)
	sl.logger.Error(styledMsg, args...)
}

func (sl *PlainStyledLogger) InfoHealthy(msg string, endpoint string, args ...any) {
	styledMsg := fmt.Sprintf("%s %s", msg, endpoint)
	sl.logger.Info(styledMsg, args...)
}

func (sl *PlainStyledLogger) InfoHealthStatus(msg string, name string, status domain.EndpointStatus, args ...any) {
	var statusText string

	switch status {
	case domain.StatusHealthy:
		statusText = "Healthy"
	case domain.StatusBusy:
