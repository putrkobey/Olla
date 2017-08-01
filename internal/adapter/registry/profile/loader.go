package profile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/filter"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"gopkg.in/yaml.v3"
)

type ProfileLoader struct {
	filter        ports.Filter
	profiles      map[string]domain.InferenceProfile
	profileFilter *domain.FilterConfig
	profilesDir   string
	loadWarnings  []string
	mu            sync.RWMutex
}

func NewProfileLoader(profilesDir string) *ProfileLoader {
	return &ProfileLoader{
		profilesDir: profilesDir,
		profiles:    make(map[string]domain.InferenceProfile),
		filter:      filter.NewGlobFilter(),
	}
}

// NewProfileLoaderWithFilter creates a new ProfileLoader with a custom filter
func NewProfileLoaderWithFilter(profilesDir string, profileFilter *domain.FilterConfig, customFilter ports.Filter) *ProfileLoader {
	filterToUse := customFilter
	if filterToUse == nil {
		filterToUse = filter.NewGlobFilter()
	}

	return &ProfileLoader{
		profilesDir:   profilesDir,
		profiles:      make(map[string]domain.InferenceProfile),
		profileFilter: profileFilter,
		filter:        filterToUse,
	}
}

const DefaultModelKey = "model"
const DefaultModelsUri = "/v1/models"
const defaultNameFormat = "{{.Name}}"

func (l *ProfileLoader) LoadProfiles() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// reset per-call so a reload doesn't accumulate warnings from a
	// previously-broken file that's since been fixed or removed
	l.loadWarnings = nil

	allProfiles := make(map[string]domain.InferenceProfile)

	// built-ins ensure it works out of the box, even without config files
	l.loadBuiltInProfilesInto(allProfiles)

	if _, err := os.Stat(l.profilesDir); os.IsNotExist(err) {
		// no config dir is fine - built-ins cover the common cases
		// Apply filtering before returning
		return l.applyProfileFilter(allProfiles)
	}

	// yaml files in config dir override built-ins
	err := filepath.WalkDir(l.profilesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		profile, err := l.loadProfile(path)
		if err != nil {
			// don't fail everything because of one bad yaml file - but don't
			// go silent either (issue #204's bug class): record it so
			// --validate-config and any caller with a real logger can
			// surface it, and warn now via slog so it shows up in ordinary
			// startup logs too.
			warning := fmt.Sprintf("%s: %v", path, err)
			l.loadWarnings = append(l.loadWarnings, warning)
			slog.Warn("profile failed to load, skipping", "path", path, "error", err)
			return nil
		}

		allProfiles[profile.GetName()] = profile
		return nil
	})

	if err != nil {
		return err
	}

	// Apply filtering to all loaded profiles
	return l.applyProfileFilter(allProfiles)
}

// LoadWarnings returns any profile files that were found but failed to load
// during the most recent LoadProfiles call - e.g. malformed YAML or a
// missing required field. Empty when every discovered profile loaded
// cleanly. Exposed so callers like --validate-config can surface these
// without depending on log capture.
func (l *ProfileLoader) LoadWarnings() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return append([]string(nil), l.loadWarnings...)
}

// applyProfileFilter applies the configured filter to the profiles
func (l *ProfileLoader) applyProfileFilter(allProfiles map[string]domain.InferenceProfile) error {
	// If no filter is configured, use all profiles
	if l.profileFilter == nil || l.profileFilter.IsEmpty() {
		l.profiles = allProfiles
		return nil
	}

	// Apply the filter
	ctx := context.Background()
	filteredProfiles, err := l.filter.ApplyToMap(ctx, l.profileFilter, convertProfilesToMap(allProfiles))
	if err != nil {
		return fmt.Errorf("failed to apply profile filter: %w", err)
	}

	// Convert back to typed map
	l.profiles = make(map[string]domain.InferenceProfile)
	for name, profile := range filteredProfiles {
		if p, ok := profile.(domain.InferenceProfile); ok {
			l.profiles[name] = p
		}
	}

	return nil
}

// convertProfilesToMap converts typed profile map to generic map for filtering
func convertProfilesToMap(profiles map[string]domain.InferenceProfile) map[string]interface{} {
	result := make(map[string]interface{}, len(profiles))
	for name, profile := range profiles {
		result[name] = profile
	}
	return result
}

func (l *ProfileLoader) loadProfile(path string) (domain.InferenceProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var config domain.ProfileConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse profile YAML: %w", err)
	}

	if config.Name == "" {
		return nil, errors.New("profile name is required")
	}
	if config.Version == "" {
		config.Version = "1.0"
	}

	return NewConfigurableProfile(&config), nil
}

// loadBuiltInProfilesInto loads built-in profiles into the provided map
func (l *ProfileLoader) loadBuiltInProfilesInto(profiles map[string]domain.InferenceProfile) {
	// hardcoded defaults for the common platforms everyone uses

	ollamaConfig := &domain.ProfileConfig{
		Name:        domain.ProfileOllama,
		Version:     "1.0",
		DisplayName: "Ollama",
		Description: "Local Ollama instance for running GGUF models",
	}
	ollamaConfig.Routing.Prefixes = []string{"ollama"}
	ollamaConfig.API.OpenAICompatible = true
	ollamaConfig.API.AnthropicSupport = &domain.AnthropicSupportConfig{
		Enabled:      true,
		MessagesPath: "/v1/messages",
		MinVersion:   "0.14.0",
		Limitations:  []string{"token_counting_404"},
	}
	ollamaConfig.API.Paths = []string{
		"/", // health check
		"/api/generate",
		"/api/chat",
		"/api/embeddings",
		"/api/tags", // models
		"/api/show",
		DefaultModelsUri,
		constants.PathV1ChatCompletions,
		constants.PathV1Completions,
		"/v1/embeddings",
		"/api/embed", // supersedes /api/embeddings per Ollama's docs
		"/api/ps",
		"/api/version",
		"/v1/responses",
		"/v1/messages",
	}
	ollamaConfig.API.ModelDiscoveryPath = "/api/tags"
	ollamaConfig.API.HealthCheckPath = "/"
	ollamaConfig.Characteristics.Timeout = 5 * time.Minute
	ollamaConfig.Characteristics.MaxConcurrentRequests = 10
	ollamaConfig.Characteristics.DefaultPriority = 100
	ollamaConfig.Characteristics.StreamingSupport = true
	ollamaConfig.Characteristics.Auth = domain.AuthHint{Types: []string{"bearer"}}
	ollamaConfig.Detection.UserAgentPatterns = []string{"ollama/"}
	ollamaConfig.Detection.Headers = []string{"X-Ollama-Version"}
	ollamaConfig.Detection.PathIndicators = []string{"/", "/api/tags"}
	ollamaConfig.Detection.DefaultPorts = []int{11434}
	ollamaConfig.Models.NameFormat = defaultNameFormat
	ollamaConfig.Metrics.Extraction = domain.MetricsExtractionConfig{
		Enabled: true,
		Source:  "response_body",
		Format:  "json",
		Paths: map[string]string{ //nolint:gosec // JSONPath extraction map keys, not credentials
			"model":              "$.model",
			"is_complete":        "$.done",
			"finish_reason":      "$.finish_reason",
			"input_tokens":       "$.prompt_eval_count",
			"output_tokens":      "$.eval_count",
			"total_duration_ns":  "$.total_duration",
			"load_duration_ns":   "$.load_duration",
			"prompt_duration_ns": "$.prompt_eval_duration",
			"eval_duration_ns":   "$.eval_duration",
		},
		Calculations: map[string]string{ //nolint:gosec // derived-metric expressions, not credentials
			"tokens_per_second": "eval_duration_ns > 0 ? (output_tokens * 1000000000.0) / eval_duration_ns : 0",
			"ttft_ms":           "prompt_duration_ns / 1000000",
			"total_ms":          "total_duration_ns / 1000000",
			"model_load_ms":     "load_duration_ns / 1000000",
		},
	}
	ollamaConfig.Request.ResponseFormat = "ollama"
	ollamaConfig.Request.ModelFieldPaths = []string{DefaultModelKey}
	ollamaConfig.Request.ParsingRules.ChatCompletionsPath = "/api/chat"
	ollamaConfig.Request.ParsingRules.CompletionsPath = "/api/generate"
	ollamaConfig.Request.ParsingRules.GeneratePath = "/api/generate"
	ollamaConfig.Request.ParsingRules.ModelFieldName = DefaultModelKey
	ollamaConfig.Request.ParsingRules.SupportsStreaming = true
	ollamaConfig.PathIndices.Health = 0
	ollamaConfig.PathIndices.Models = 4
	ollamaConfig.PathIndices.Completions = 1
	ollamaConfig.PathIndices.ChatCompletions = 2
	ollamaConfig.PathIndices.Embeddings = 3

	// Resource patterns for built-in Ollama profile
	ollamaConfig.Resources.ModelSizes = []domain.ModelSizePattern{
		{Patterns: []string{"70b", "72b"}, MinMemoryGB: 40, RecommendedMemoryGB: 48, MinGPUMemoryGB: 40, EstimatedLoadTimeMS: 300000},
		{Patterns: []string{"65b"}, MinMemoryGB: 35, RecommendedMemoryGB: 40, MinGPUMemoryGB: 35, EstimatedLoadTimeMS: 240000},
		{Patterns: []string{"34b", "33b", "30b"}, MinMemoryGB: 20, RecommendedMemoryGB: 24, MinGPUMemoryGB: 20, EstimatedLoadTimeMS: 120000},
		{Patterns: []string{"13b", "14b"}, MinMemoryGB: 10, RecommendedMemoryGB: 16, MinGPUMemoryGB: 10, EstimatedLoadTimeMS: 60000},
		{Patterns: []string{"7b", "8b"}, MinMemoryGB: 6, RecommendedMemoryGB: 8, MinGPUMemoryGB: 6, EstimatedLoadTimeMS: 30000},
		{Patterns: []string{"3b"}, MinMemoryGB: 3, RecommendedMemoryGB: 4, MinGPUMemoryGB: 3, EstimatedLoadTimeMS: 15000},
		{Patterns: []string{"1b", "1.5b"}, MinMemoryGB: 2, RecommendedMemoryGB: 3, MinGPUMemoryGB: 2, EstimatedLoadTimeMS: 10000},
	}
	ollamaConfig.Resources.Quantization.Multipliers = map[string]float64{
		"q4": 0.5,
		"q5": 0.625,
		"q6": 0.75,
		"q8": 0.875,
	}
	ollamaConfig.Resources.Defaults = domain.ResourceRequirements{
		MinMemoryGB: 4, RecommendedMemoryGB: 8, MinGPUMemoryGB: 4, RequiresGPU: false, EstimatedLoadTimeMS: 5000,
	}

	// Model capability patterns
	ollamaConfig.Models.CapabilityPatterns = map[string][]string{
		"vision":     {"*llava*", "*vision*", "*bakllava*"},
		"embeddings": {"*embed*", "nomic-embed-text", "mxbai-embed-large"},
		"code":       {"*code*", "codellama*", "deepseek-coder*", "qwen*coder*"},
	}

	// Context window patterns
	ollamaConfig.Models.ContextPatterns = []domain.ContextPattern{
		{Pattern: "*-32k*", Context: 32768},
		{Pattern: "*-16k*", Context: 16384},
		{Pattern: "*-8k*", Context: 8192},
		{Pattern: "*:32k*", Context: 32768},
		{Pattern: "*:16k*", Context: 16384},
		{Pattern: "*:8k*", Context: 8192},
		{Pattern: "llama3*", Context: 8192},
		{Pattern: "llama-3*", Context: 8192},
	}

	// Concurrency limits based on model size
	ollamaConfig.Resources.ConcurrencyLimits = []domain.ConcurrencyLimitPattern{
		{MinMemoryGB: 30, MaxConcurrent: 1},
		{MinMemoryGB: 15, MaxConcurrent: 2},
		{MinMemoryGB: 8, MaxConcurrent: 4},
		{MinMemoryGB: 0, MaxConcurrent: 8},
	}

	// Timeout scaling
	ollamaConfig.Resources.TimeoutScaling = domain.TimeoutScaling{
		BaseTimeoutSeconds: 30,
		LoadTimeBuffer:     true,
	}

	profiles[domain.ProfileOllama] = NewConfigurableProfile(ollamaConfig)

	// LM Studio built-in profile
	lmStudioConfig := &domain.ProfileConfig{
		Name:        domain.ProfileLmStudio,
		Version:     "1.0",
		DisplayName: "LM Studio",
		Description: "LM Studio local inference server",
	}
	lmStudioConfig.Routing.Prefixes = []string{"lmstudio", "lm-studio", "lm_studio"}
	lmStudioConfig.API.OpenAICompatible = true
	lmStudioConfig.API.AnthropicSupport = &domain.AnthropicSupportConfig{
		Enabled:      true,
		MessagesPath: "/v1/messages",
		MinVersion:   "0.4.1",
	}
	lmStudioConfig.API.Paths = []string{
		DefaultModelsUri, // both health check and models
		constants.PathV1ChatCompletions,
		constants.PathV1Completions,
		"/v1/embeddings",
		"/api/v0/models",
		"/v1/responses",
		"/v1/messages",
	}
