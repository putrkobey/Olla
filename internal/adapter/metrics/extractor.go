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
}

// ExtractMetrics extracts metrics from response data and headers.
// gjson parsing is synchronous and sub-millisecond for typical payloads,
// so we call doExtract directly rather than racing against a goroutine timeout,
// which is unreliable under scheduler pressure and causes spurious nil returns.
func (e *Extractor) ExtractMetrics(ctx context.Context, data []byte, headers http.Header, providerName string) *domain.ProviderMetrics {
	if len(data) == 0 && headers == nil {
		return nil
	}

	// Bail early if the caller's context is already done.
	select {
	case <-ctx.Done():
		e.failures.Add(1)
		e.logger.Debug("Metrics extraction skipped: context cancelled", "provider", providerName)
		return nil
	default:
	}

	e.extractionCount.Add(1)
	return e.doExtract(data, headers, providerName)
}

// ExtractFromChunk extracts metrics from a response chunk
func (e *Extractor) ExtractFromChunk(ctx context.Context, chunk []byte, providerName string) *domain.ProviderMetrics {
	return e.ExtractMetrics(ctx, chunk, nil, providerName)
}

// doExtract performs the actual extraction using gjson for speed
func (e *Extractor) doExtract(data []byte, headers http.Header, providerName string) *domain.ProviderMetrics {
	if len(data) == 0 && headers == nil {
		return nil
	}

	profile, err := e.profileFactory.GetProfile(providerName)
	if err != nil || profile == nil {
		return nil
	}

	config := profile.GetConfig()
	if config == nil || !config.Metrics.Extraction.Enabled {
		return nil
	}

	// Ensure profile is validated (only validate once per profile)
	profileName := profile.GetName()
	if _, validated := e.validatedProfiles.Load(profileName); !validated {
		if err := e.ValidateProfile(profile); err == nil {
			e.validatedProfiles.Store(profileName, true)
		}
	}

	metricsConfig := config.Metrics.Extraction
	metrics := &domain.ProviderMetrics{}

	// Parse JSON once using gjson (much faster than encoding/json)
	var jsonResult gjson.Result
	if len(data) > 0 && metricsConfig.Source != "response_headers" {
		jsonResult = gjson.ParseBytes(data)
		if !jsonResult.Exists() {
			return nil
		}
	}

	// Extract values using cached gjson paths
	values := make(map[string]interface{})
	for field := range metricsConfig.Paths {
		cacheKey := fmt.Sprintf("%s:%s", profile.GetName(), field)
		if pathObj, ok := e.fieldPaths.Load(cacheKey); ok {
			path, ok := pathObj.(string)
			if !ok {
				continue
			}
			result := jsonResult.Get(path)
			if result.Exists() {
				// Store the raw value for calculations
				values[field] = result.Value()

				// Also map directly to metrics fields
				e.mapFieldToMetrics(field, result, metrics)
			}
		}
	}

	// Execute pre-compiled calculations
	e.runCalculations(profile.GetName(), values, metricsConfig.Calculations, metrics)

	// Extract from headers if configured
	if headers != nil && len(metricsConfig.Headers) > 0 {
		e.extractFromHeaders(headers, metricsConfig, metrics)
	}

	return metrics
}

// mapFieldToMetrics maps extracted field values directly to metrics struct
func (e *Extractor) mapFieldToMetrics(field string, result gjson.Result, metrics *domain.ProviderMetrics) {
	switch field {
	case "input_tokens":
		metrics.InputTokens = util.SafeInt32(result.Int())
	case "output_tokens":
		metrics.OutputTokens = util.SafeInt32(result.Int())
	case "total_tokens":
		metrics.TotalTokens = util.SafeInt32(result.Int())
	case "model":
		metrics.Model = result.String()
	case "finish_reason":
		metrics.FinishReason = result.String()
	case "done", "is_complete":
		metrics.IsComplete = result.Bool()
	case "prompt_duration_ns", "prompt_eval_duration":
		ms := result.Int() / 1_000_000
		metrics.PromptMs = util.SafeInt32(ms)
		metrics.TTFTMs = metrics.PromptMs // TTFT approximation
