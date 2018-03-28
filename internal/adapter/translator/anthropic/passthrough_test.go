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
