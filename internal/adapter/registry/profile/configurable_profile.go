package profile

import (
	"fmt"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/util"
	"github.com/putrkobey/Olla/internal/util/pattern"
)

const (
	defaultModelsFieldPath = "data"
	ollamaResponseFormat   = "ollama"
	ollamaModelsFieldPath  = "models"
)

// ConfigurableProfile bridges YAML config to the PlatformProfile interface.
// Much easier for users to write YAML than implement Go interfaces.
type ConfigurableProfile struct {
	config *domain.ProfileConfig

	// each platform returns models in its own special snowflake format
	modelParser ModelResponseParser
}

func NewConfigurableProfile(config *domain.ProfileConfig) *ConfigurableProfile {
	return &ConfigurableProfile{
		config:      config,
		modelParser: getParserForFormat(config.Request.ResponseFormat),
	}
}

func (p *ConfigurableProfile) GetName() string {
	return p.config.Name
}

func (p *ConfigurableProfile) GetVersion() string {
	return p.config.Version
}

func (p *ConfigurableProfile) GetPaths() []string {
	return p.config.API.Paths
}

func (p *ConfigurableProfile) GetPath(index int) string {
	if index < 0 || index >= len(p.config.API.Paths) {
		return ""
	}
	return p.config.API.Paths[index]
}

func (p *ConfigurableProfile) GetModelDiscoveryURL(baseURL string) string {
	return util.ResolveURLPath(baseURL, p.config.API.ModelDiscoveryPath)
}

func (p *ConfigurableProfile) GetHealthCheckPath() string {
	return p.config.API.HealthCheckPath
}

func (p *ConfigurableProfile) IsOpenAPICompatible() bool {
	return p.config.API.OpenAICompatible
}

func (p *ConfigurableProfile) GetRequestParsingRules() domain.RequestParsingRules {
	rules := p.config.Request.ParsingRules
	return domain.RequestParsingRules{
		ChatCompletionsPath: rules.ChatCompletionsPath,
		CompletionsPath:     rules.CompletionsPath,
		GeneratePath:        rules.GeneratePath,
		ModelFieldName:      rules.ModelFieldName,
		SupportsStreaming:   rules.SupportsStreaming,
	}
}

func (p *ConfigurableProfile) GetModelResponseFormat() domain.ModelResponseFormat {
	// Determine the models field path based on response format
	modelsFieldPath := defaultModelsFieldPath
	if p.config.Request.ResponseFormat == ollamaResponseFormat {
		modelsFieldPath = ollamaModelsFieldPath
	}

	return domain.ModelResponseFormat{
		ResponseType:    "object",
		ModelsFieldPath: modelsFieldPath,
	}
}

func (p *ConfigurableProfile) GetDetectionHints() domain.DetectionHints {
	return domain.DetectionHints{
		UserAgentPatterns: p.config.Detection.UserAgentPatterns,
		ResponseHeaders:   p.config.Detection.Headers,
		PathIndicators:    p.config.Detection.PathIndicators,
	}
}

func (p *ConfigurableProfile) ParseModelsResponse(data []byte) ([]*domain.ModelInfo, error) {
	if p.modelParser == nil {
		return nil, fmt.Errorf("no model parser configured for format: %s", p.config.Request.ResponseFormat)
	}
	return p.modelParser.Parse(data)
}

func (p *ConfigurableProfile) GetTimeout() time.Duration {
	if p.config.Characteristics.Timeout == 0 {
		return 2 * time.Minute
	}
	return p.config.Characteristics.Timeout
}

func (p *ConfigurableProfile) GetMaxConcurrentRequests() int {
	if p.config.Characteristics.MaxConcurrentRequests == 0 {
		return 10
	}
	return p.config.Characteristics.MaxConcurrentRequests
}

func (p *ConfigurableProfile) GetDefaultPriority() int {
	if p.config.Characteristics.DefaultPriority == 0 {
		return 50
	}
	return p.config.Characteristics.DefaultPriority
}

func (p *ConfigurableProfile) GetConfig() *domain.ProfileConfig {
	return p.config
}

func (p *ConfigurableProfile) ValidateEndpoint(endpoint *domain.Endpoint) error {
	if endpoint.URL == nil {
		return fmt.Errorf("%s endpoint requires URL", p.config.Name)
	}

	// ollama defaults to http, but we need to be explicit for safety
	if endpoint.URL.Scheme == "" {
		return fmt.Errorf("%s endpoint URL must include scheme (http:// or https://)", p.config.Name)
	}

	return nil
}

// InferenceProfile implementation

func (p *ConfigurableProfile) GetModelCapabilities(modelName string, registry domain.ModelRegistry) domain.ModelCapabilities {
	caps := domain.ModelCapabilities{
		ChatCompletion:   true, // Default for most models
		TextGeneration:   true, // Default for most models
		StreamingSupport: p.config.Request.ParsingRules.SupportsStreaming,
		MaxContextLength: 4096,
		MaxOutputTokens:  2048,
	}

	// Check capability patterns from config

	// Check for embeddings capability
	if patterns, ok := p.config.Models.CapabilityPatterns["embeddings"]; ok {
		for _, pat := range patterns {
			if pattern.MatchesGlob(modelName, pat) {
				caps.Embeddings = true
				caps.ChatCompletion = false
				caps.TextGeneration = false
				break
			}
		}
	}

	// Check for vision capability
	if patterns, ok := p.config.Models.CapabilityPatterns["vision"]; ok {
		for _, pat := range patterns {
			if pattern.MatchesGlob(modelName, pat) {
				caps.VisionUnderstanding = true
				break
			}
		}
	}

	// Check for code capability
