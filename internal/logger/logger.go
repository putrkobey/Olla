package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/putrkobey/Olla/internal/util"
	"github.com/putrkobey/Olla/theme"
)

type Config struct {
	Level      string
	LogDir     string
	Theme      string
	MaxSize    int // megabytes
	MaxBackups int
	MaxAge     int // days
	FileOutput bool
	PrettyLogs bool
}

const (
	DefaultLogOutputName  = "olla.log"
	DefaultDetailedCookie = "detailed"

	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarn    = "warn"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
	LogLevelFatal   = "fatal"
	LogLevelPanic   = "panic"
)

// regex for stripping ANSI escape codes from log messages
// Matches all ANSI escape sequences: \x1b[...m, \x1b[...K, \x1b[...A, etc.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func New(cfg *Config) (*slog.Logger, func(), error) {
	level := parseLevel(cfg.Level)
	appTheme := theme.GetTheme(cfg.Theme)

	var cleanupFuncs []func()
	var handlers []slog.Handler

	// if folks want pretty, prioritise that
	// unless we have NO_COLOR set, then we use text
	if cfg.PrettyLogs {
		if util.ShouldUseColors() {
			handlers = append(handlers, createPTermHandler(level, appTheme))
		} else {
			handlers = append(handlers, createTextHandler(level))
		}
	} else {
		handlers = append(handlers, createJSONHandler(level, os.Stdout))
	}

	// Optional file handler if they really want it
	if cfg.FileOutput {
		fileHandler, cleanup, err := createFileHandler(cfg, level)
		if err != nil {
			return nil, nil, err
		}
		cleanupFuncs = append(cleanupFuncs, cleanup)
		handlers = append(handlers, fileHandler)
	}

	var logger *slog.Logger
	if len(handlers) == 1 {
		logger = slog.New(handlers[0])
	} else {
		logger = slog.New(&multiHandler{handlers: handlers})
	}

	cleanup := func() {
		for _, fn := range cleanupFuncs {
			fn()
		}
	}

	return logger, cleanup, nil
}

// createPTermHandler creates a colorful pterm-based handler for terminals
func createPTermHandler(level slog.Level, appTheme *theme.Theme) slog.Handler {
	plogger := pterm.DefaultLogger.
		WithLevel(convertToPTermLevel(level)).
		WithWriter(os.Stdout).
		WithFormatter(pterm.LogFormatterColorful)

	keyStyles := map[string]pterm.Style{
		"level": *appTheme.Info,
		"msg":   *appTheme.Info,
		"time":  *appTheme.Muted,
	}
	plogger = plogger.WithKeyStyles(keyStyles)

	return &terminalFilterHandler{
		handler: pterm.NewSlogHandler(plogger),
	}
}

// createTextHandler creates a plain text handler for non-TTY environments
func createTextHandler(level slog.Level) slog.Handler {
	return &terminalFilterHandler{
		handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: false,
		}),
	}
}

// createJSONHandler creates a JSON handler for the specified writer
func createJSONHandler(level slog.Level, writer *os.File) slog.Handler {
	return slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:       level,
		AddSource:   false,
		ReplaceAttr: replaceTimeAttr,
	})
}

// createFileHandler creates a file-based JSON handler with rotation
// wraps with ansiStripHandler to ensure clean JSON output without ANSI codes
func createFileHandler(cfg *Config, level slog.Level) (slog.Handler, func(), error) {
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, nil, err
	}

	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.LogDir, DefaultLogOutputName),
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   true,
	}

	jsonHandler := slog.NewJSONHandler(rotator, &slog.HandlerOptions{
		Level:       level,
		AddSource:   false,
		ReplaceAttr: replaceTimeAttr,
	})

	// wrap with ANSI stripping handler to ensure lean JSON output
	handler := &ansiStripHandler{handler: jsonHandler}

	cleanup := func() {
		_ = rotator.Close()
	}

	return handler, cleanup, nil
}
