package metrics

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/tidwall/gjson"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/util"
)

// ProfileFactory provides access to provider profiles
type ProfileFactory interface {
	GetProfile(profileType string) (domain.InferenceProfile, error)
}

// Extractor implements high-performance metrics extraction using robust libraries
type Extractor struct {
	profileFactory ProfileFactory
	logger         logger.StyledLogger

	// Pre-compiled expressions cache
	compiledExprs sync.Map // map[string]*vm.Program

	// Pre-parsed field paths cache
	fieldPaths sync.Map // map[string]string - profile:field -> gjson path

	// Track which profiles have been validated
	validatedProfiles sync.Map // map[string]bool

	// Monitoring counters
	extractionCount atomic.Int64
	failures        atomic.Int64
}

// NewExtractor creates a new metrics extractor using robust libraries
func NewExtractor(profileFactory ProfileFactory, logger logger.StyledLogger) (*Extractor, error) {
	return &Extractor{
		profileFactory: profileFactory,
		logger:         logger,
	}, nil
}

// ValidateProfile validates and pre-compiles expressions at startup
func (e *Extractor) ValidateProfile(profile domain.InferenceProfile) error {
	config := profile.GetConfig()
	if config == nil || !config.Metrics.Extraction.Enabled {
		return nil
	}

	metricsConfig := config.Metrics.Extraction

	// Cache field paths for gjson
	for field, path := range metricsConfig.Paths {
		if path == "" {
			continue
		}

		// Convert JSONPath to gjson syntax if needed
		gjsonPath := convertToGjsonPath(path)
		cacheKey := fmt.Sprintf("%s:%s", profile.GetName(), field)
		e.fieldPaths.Store(cacheKey, gjsonPath)
	}

	// Pre-compile calculation expressions using expr library
	for field, expression := range metricsConfig.Calculations {
		if expression == "" {
			continue
		}

		// Create environment with expected variables
		env := map[string]interface{}{
			"input_tokens":       0,
			"output_tokens":      0,
			"eval_duration_ns":   0,
			"prompt_duration_ns": 0,
			"total_duration_ns":  0,
			"load_duration_ns":   0,
		}

		// Add all path fields as potential variables
		for fieldName := range metricsConfig.Paths {
			env[fieldName] = 0
		}

		// Compile the expression
		program, err := expr.Compile(expression,
			expr.Env(env),
			expr.AsFloat64(), // Ensure output is float64
		)
		if err != nil {
			return fmt.Errorf("profile %s: invalid calculation for field %s: %s - %w",
				profile.GetName(), field, expression, err)
		}

		cacheKey := fmt.Sprintf("%s:%s", profile.GetName(), field)
		e.compiledExprs.Store(cacheKey, program)
	}

	e.logger.Debug("Validated metrics configuration with robust libraries",
		"profile", profile.GetName(),
		"paths", len(metricsConfig.Paths),
		"calculations", len(metricsConfig.Calculations))

	return nil
