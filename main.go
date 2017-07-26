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
func main() {
	// flag.Parse must run here, not in init(): init() runs under `go test` too,
	// and parsing the test binary's own flags (-test.*) against our flag set
	// fails the build. main() only runs for the real binary.
	flag.Parse()

	if os.Getenv("OLLA_ENABLE_PROFILER") == "true" {
		enableProfiling = true
	}

	if os.Getenv("OLLA_SHOW_VERSION") == "true" {
		showVersion = true
	}

	startTime := time.Now()
	vlog := log.New(log.Writer(), "", 0)
	profileAddress := "localhost:19841"

	if showVersion {
		version.PrintVersionInfo(true, vlog)
		os.Exit(0)
	} else {
		version.PrintVersionInfo(false, vlog)
	}

	if validateConfigFlag {
		report, exitCode := runValidateConfig(configFile)
		fmt.Print(report)
		os.Exit(exitCode)
	}

	if enableProfiling {
		profiler.InitialiseProfiler(profileAddress)
		vlog.Printf(theme.ColourProfiler("Profiling server started at http://%s/debug/pprof/\n"), profileAddress)
	}

	// Setup logging. This is a bootstrap logger only: OLLA_LOG_LEVEL controls it,
	// and it exists purely so pre-config messages (version banner, config load
	// errors) have somewhere to go. Once the config file is loaded below, we
	// rebuild the logger against logging.level so the file's setting actually
	// takes effect instead of being silently ignored.
	lcfg := buildLoggerConfig()
	logInstance, styledLogger, loggerCleanup, err := logger.NewWithTheme(lcfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialise logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { loggerCleanup() }()

	slog.SetDefault(logInstance)

	styledLogger.Info("Initialising", "version", version.Version, "pid", os.Getpid())
	styledLogger.Info("System Configuration", "isContainerised", container.IsContainerised(), "numCPU", runtime.NumCPU(), "goVersion", runtime.Version())

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	var shutdownTime time.Time

	go func() {
		sig := <-sigCh
		styledLogger.Info("Shutdown signal received", "signal", sig.String())
		shutdownTime = time.Now()
		cancel()
	}()

