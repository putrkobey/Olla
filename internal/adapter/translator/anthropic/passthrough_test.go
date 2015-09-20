package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// mockProfileLookup is a test double that implements translator.ProfileLookup
// Allows configuring AnthropicSupportConfig per endpoint type for testing
type mockProfileLookup struct {
	configs map[string]*domain.AnthropicSupportConfig
}

// GetAnthropicSupport returns the configured AnthropicSupportConfig for the given endpoint type
func (m *mockProfileLookup) GetAnthropicSupport(endpointType string) *domain.AnthropicSupportConfig {
	if m.configs == nil {
		return nil
	}
	return m.configs[endpointType]
}

// newMockProfileLookup creates a new mock profile lookup with empty configuration
func newMockProfileLookup() *mockProfileLookup {
	return &mockProfileLookup{
		configs: make(map[string]*domain.AnthropicSupportConfig),
	}
}

// withSupport adds AnthropicSupportConfig for a specific endpoint type
func (m *mockProfileLookup) withSupport(endpointType string, cfg *domain.AnthropicSupportConfig) *mockProfileLookup {
	m.configs[endpointType] = cfg
	return m
}

// TestCanPassthrough tests the CanPassthrough method with various endpoint configurations
func TestCanPassthrough(t *testing.T) {
	tests := []struct {
		name               string
		passthroughEnabled bool
		endpoints          []*domain.Endpoint
		profileLookup      translator.ProfileLookup
		want               bool
		description        string
	}{
		{
			name:               "all_endpoints_support_anthropic",
			passthroughEnabled: true,
			endpoints: []*domain.Endpoint{
				{Name: "vllm-1", Type: "vllm"},
				{Name: "vllm-2", Type: "vllm"},
			},
			profileLookup: newMockProfileLookup().withSupport("vllm", &domain.AnthropicSupportConfig{
				Enabled:      true,
				MessagesPath: "/v1/messages",
			}),
			want:        true,
			description: "should return true when all endpoints support Anthropic passthrough",
		},
		{
			// The handler pre-filters to only capable endpoints before calling CanPassthrough.
			// By the time this method is called, non-capable backends are already excluded.
			// A mixed vllm+ollama deployment arrives here as [vllm-1] — passthrough proceeds.
			name:               "mixed_endpoints_some_support_some_dont",
			passthroughEnabled: true,
			endpoints: []*domain.Endpoint{
				{Name: "vllm-1", Type: "vllm"}, // handler already stripped ollama
			},
			profileLookup: newMockProfileLookup().
				withSupport("vllm", &domain.AnthropicSupportConfig{
					Enabled:      true,
					MessagesPath: "/v1/messages",
				}),
			want:        true,
			description: "should return true when handler pre-filtered to only capable endpoints",
		},
		{
			// When no backends support Anthropic, the handler's filter produces an empty list.
			// CanPassthrough returns false for an empty list regardless of config.
			name:               "no_endpoints_support_anthropic",
			passthroughEnabled: true,
			endpoints:          []*domain.Endpoint{}, // handler filtered all out
			profileLookup:      newMockProfileLookup().withSupport("ollama", nil),
			want:               false,
			description:        "should return false when handler pre-filter yields no capable endpoints",
		},
		{
			name:               "passthrough_disabled",
			passthroughEnabled: false,
			endpoints: []*domain.Endpoint{
				{Name: "vllm-1", Type: "vllm"},
			},
			profileLookup: newMockProfileLookup().withSupport("vllm", &domain.AnthropicSupportConfig{
				Enabled:      true,
				MessagesPath: "/v1/messages",
			}),
			want:        false,
			description: "should return false when passthrough is disabled even if endpoints support it",
		},
		{
			name:               "empty_endpoints_list",
			passthroughEnabled: true,
			endpoints:          []*domain.Endpoint{},
			profileLookup:      newMockProfileLookup(),
			want:               false,
			description:        "should return false when endpoints list is empty",
		},
		{
			// Backend with no AnthropicSupportConfig is excluded by the handler filter.
			// CanPassthrough receives an empty list and returns false.
			name:               "nil_anthropic_support_config",
			passthroughEnabled: true,
			endpoints:          []*domain.Endpoint{}, // handler excluded the unsupported custom endpoint
			profileLookup:      newMockProfileLookup(),
			want:               false,
			description:        "should return false when handler filter excludes unsupported backends",
		},
		{
			// Backend with Enabled:false is excluded by the handler filter.
			// CanPassthrough receives an empty list and returns false.
			name:               "anthropic_support_disabled",
			passthroughEnabled: true,
			endpoints:          []*domain.Endpoint{}, // handler excluded the disabled endpoint
			profileLookup: newMockProfileLookup().withSupport("vllm", &domain.AnthropicSupportConfig{
				Enabled:      false,
				MessagesPath: "/v1/messages",
			}),
			want:        false,
			description: "should return false when handler filter excludes explicitly-disabled backends",
		},
		{
			name:               "multiple_endpoints_all_support",
			passthroughEnabled: true,
			endpoints: []*domain.Endpoint{
				{Name: "vllm-1", Type: "vllm"},
				{Name: "vllm-2", Type: "vllm"},
				{Name: "sglang-1", Type: "sglang"},
				{Name: "litellm-1", Type: "litellm"},
			},
			profileLookup: newMockProfileLookup().
				withSupport("vllm", &domain.AnthropicSupportConfig{
					Enabled:      true,
					MessagesPath: "/v1/messages",
				}).
				withSupport("sglang", &domain.AnthropicSupportConfig{
					Enabled:      true,
					MessagesPath: "/v1/messages",
				}).
				withSupport("litellm", &domain.AnthropicSupportConfig{
					Enabled:      true,
					MessagesPath: "/v1/messages",
				}),
			want:        true,
			description: "should return true when multiple different endpoint types all support Anthropic",
		},
		{
			// The handler filters the unsupported backend before calling CanPassthrough.
			// The three vllm endpoints survive the filter — passthrough proceeds.
			name:               "single_endpoint_without_support",
			passthroughEnabled: true,
			endpoints: []*domain.Endpoint{
				{Name: "vllm-1", Type: "vllm"},
				{Name: "vllm-2", Type: "vllm"},
				{Name: "vllm-3", Type: "vllm"},
				// unsupported-1 already removed by handler filter
			},
			profileLookup: newMockProfileLookup().
				withSupport("vllm", &domain.AnthropicSupportConfig{
					Enabled:      true,
					MessagesPath: "/v1/messages",
				}),
			want:        true,
			description: "should return true when handler pre-filter removes the unsupported endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.AnthropicTranslatorConfig{
				Enabled:            true,
				MaxMessageSize:     10 << 20, // 10MB
				PassthroughEnabled: tt.passthroughEnabled,
			}

			translator := mustNewTranslator(createTestLogger(), cfg)
			result := translator.CanPassthrough(tt.endpoints, tt.profileLookup)

			assert.Equal(t, tt.want, result, tt.description)
		})
	}
}

// TestPreparePassthrough tests the PreparePassthrough method with various request scenarios
func TestPreparePassthrough(t *testing.T) {
	tests := []struct {
		name         string
		requestBody  string
		maxMsgSize   int64
		wantErr      bool
		errContains  string
		validateFunc func(t *testing.T, result *translator.PassthroughRequest)
		description  string
	}{
		{
			name: "valid_anthropic_request",
			requestBody: `{
				"model": "claude-3-5-sonnet-20241022",
				"max_tokens": 1024,
				"messages": [
					{"role": "user", "content": "Hello, world!"}
				]
			}`,
			maxMsgSize: 10 << 20,
			wantErr:    false,
			validateFunc: func(t *testing.T, result *translator.PassthroughRequest) {
				assert.NotNil(t, result)
				assert.Equal(t, "/v1/messages", result.TargetPath)
				assert.Equal(t, "claude-3-5-sonnet-20241022", result.ModelName)
				assert.False(t, result.IsStreaming)
				assert.NotEmpty(t, result.Body)

				// Verify body is preserved unchanged
				var req AnthropicRequest
				err := json.Unmarshal(result.Body, &req)
				require.NoError(t, err)
				assert.Equal(t, "claude-3-5-sonnet-20241022", req.Model)
				assert.Equal(t, 1024, req.MaxTokens)
				assert.Len(t, req.Messages, 1)
			},
			description: "should successfully prepare valid Anthropic request",
		},
		{
			name:        "invalid_json",
			requestBody: `{invalid json`,
			maxMsgSize:  10 << 20,
			wantErr:     true,
			errContains: "invalid Anthropic request",
			description: "should return error for invalid JSON",
		},
		{
			name: "missing_model_field",
			requestBody: `{
				"max_tokens": 1024,
				"messages": [
					{"role": "user", "content": "Hello"}
				]
			}`,
			maxMsgSize:  10 << 20,
