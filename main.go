package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/putrkobey/Olla/pkg/container"

	"github.com/putrkobey/Olla/pkg/profiler"
	"github.com/putrkobey/Olla/theme"

	"github.com/putrkobey/Olla/internal/app"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/env"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/util"
	"github.com/putrkobey/Olla/internal/version"
	"github.com/putrkobey/Olla/pkg/format"
	"github.com/putrkobey/Olla/pkg/nerdstats"
)

const (
	DefaultLoggerLevel   = "info"
	DefaultPrettyLogs    = true
	DefaultFileOutput    = true
	DefaultLogDir        = "./logs"
	DefaultLogSizeMB     = 1
	DefaultLogMaxBackups = 7
	DefaultLogMaxAgeDays = 14
	DefaultTheme         = "default"

	// Runtime (post-config) logging.format and logging.output defaults -
	// these match LoggingConfig's defaults in internal/config, not the
	// bootstrap logger's DefaultPrettyLogs/DefaultFileOutput above, which
	// are deliberately different (a friendly pretty banner before the
	// config file is even read).
	DefaultLoggerFormat = "json"
	DefaultLoggerOutput = "stdout"

	LogFormatJSON = "json"
	LogFormatText = "text"

	LogOutputStdout = "stdout"
	LogOutputFile   = "file"
)

var (
	enableProfiling    bool
	showVersion        bool
	configFile         string
	validateConfigFlag bool
)

func init() {
	flag.BoolVar(&enableProfiling, "profile", false, "Enable pprof profiling server")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.StringVar(&configFile, "c", "", "Config file path")
	flag.StringVar(&configFile, "config", "", "Config file path")
	flag.BoolVar(&validateConfigFlag, "validate-config", false, "Validate config.yaml, models.yaml and profiles, then exit without starting the server")
}
