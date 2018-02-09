package unifier

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetConfig_ReflectsResetSingleton is the regression test for the
// double-cache bug: getConfig() used to keep its own sync.Once independent of
// LoadModelConfig's, so resetConfigSingleton() (which only clears the latter)
// never actually reached extraction - normalizeQuantization and friends kept
// serving whatever config they saw on first use for the rest of the test
// process. getConfig() now delegates straight to LoadModelConfig(), so a
// reset must be visible here too.
func TestGetConfig_ReflectsResetSingleton(t *testing.T) {
	// Latch getConfig() onto the embedded defaults first, same as any other
	// test in this process would.
	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)
	require.NotContains(t, getDefaultConfig().Quantization.Mappings, "ZZTESTMARKER", "sanity: pick a mapping key the embedded defaults don't already define")

	dir := t.TempDir()
	customYAML := `
quantization:
  mappings:
    ZZTESTMARKER: "custom-marker"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "models.yaml"), []byte(customYAML), 0644))
	// Select the config via OLLA_CONFIG_DIR rather than t.Chdir, so this test
	// stays isolated from others that use unqualified models.yaml paths.
	t.Setenv("OLLA_CONFIG_DIR", dir)

	resetConfigSingleton()

	assert.Equal(t, "custom-marker", normalizeQuantization("ZZTESTMARKER"), "getConfig() must observe the reset, not a stale cached config")
}

func TestNormalizeQuantization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// K-quants
		{"Q4_K_M uppercase", "Q4_K_M", "q4km"},
		{"Q4_K_M lowercase", "q4_k_m", "q4km"},
		{"Q4_K_S", "Q4_K_S", "q4ks"},
		{"Q3_K_L", "Q3_K_L", "q3kl"},
		{"Q3_K_M", "Q3_K_M", "q3km"},
		{"Q3_K_S", "Q3_K_S", "q3ks"},
		{"Q5_K_M", "Q5_K_M", "q5km"},
		{"Q5_K_S", "Q5_K_S", "q5ks"},
		{"Q6_K", "Q6_K", "q6k"},
		{"Q2_K", "Q2_K", "q2k"},

		// Regular quants
		{"Q4_0", "Q4_0", "q4"},
		{"Q4_1", "Q4_1", "q4_1"},
		{"Q5_0", "Q5_0", "q5"},
		{"Q5_1", "Q5_1", "q5_1"},
		{"Q8_0", "Q8_0", "q8"},

		// Float formats
		{"F16", "F16", "f16"},
		{"FP16", "FP16", "f16"},
		{"F32", "F32", "f32"},
		{"FP32", "FP32", "f32"},
		{"BF16", "BF16", "bf16"},

		// GPTQ/AWQ formats
		{"GPTQ_4BIT", "GPTQ_4BIT", "gptq4"},
		{"GPTQ-4BIT", "GPTQ-4BIT", "gptq4"},
		{"AWQ_4BIT", "AWQ_4BIT", "awq4"},
		{"AWQ-4BIT", "AWQ-4BIT", "awq4"},

		// Other formats
		{"INT8", "INT8", "int8"},
		{"INT4", "INT4", "int4"},

		// Unknown/unmapped
		{"CUSTOM", "CUSTOM", "custom"},
		{"", "", ""},

		// Mixed case
		{"q4_K_m", "q4_K_m", "q4km"},
		{"Q4_k_M", "Q4_k_M", "q4km"},

		// gpt-oss / Nemotron FP4 formats
		{"MXFP4", "MXFP4", "mxfp4"},
		{"NVFP4", "NVFP4", "nvfp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeQuantization(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractParameterSize(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedSize  string
		expectedCount int64
	}{
		// Standard formats
		{"7B", "7B", "7b", 7_000_000_000},
		{"7b lowercase", "7b", "7b", 7_000_000_000},
		{"13B", "13B", "13b", 13_000_000_000},
		{"70B", "70B", "70b", 70_000_000_000},
		{"175B", "175B", "175b", 175_000_000_000},

		// Decimal formats
		{"1.5B", "1.5B", "1.5b", 1_500_000_000},
		{"3.8B", "3.8B", "3.8b", 3_800_000_000},
		{"12.2B", "12.2B", "12.2b", 12_200_000_000},
		{"0.5B", "0.5B", "0.5b", 500_000_000},

		// Million parameters
		{"540M", "540M", "540m", 540_000_000},
		{"125M", "125M", "125m", 125_000_000},

		// With spaces
		{"7 B", "7 B", "7b", 7_000_000_000},
		{"1.5 B", "1.5 B", "1.5b", 1_500_000_000},

		// Edge cases
		{"", "", "", 0},
		{"unknown", "unknown", "unknown", 0},
		{"7", "7", "7b", 7_000_000_000},
		{"NaN", "NaN", "NaN", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, count := extractParameterSize(tt.input)
			assert.Equal(t, tt.expectedSize, size)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestInferCapabilitiesFromMetadata(t *testing.T) {
	tests := []struct {
		name                 string
		modelType            string
		modelName            string
		contextLength        int64
		metadata             map[string]interface{}
		expectedCapabilities []string
	}{
		{
			name:                 "LLM type",
			modelType:            "llm",
			modelName:            "llama-3.2-1b",
			expectedCapabilities: []string{"text-generation", "chat", "completion"},
		},
		{
			name:                 "VLM type",
			modelType:            "vlm",
			modelName:            "llava-v1.6",
			expectedCapabilities: []string{"text-generation", "vision", "multimodal", "image-understanding"},
		},
		{
			name:                 "Embeddings type",
			modelType:            "embeddings",
			modelName:            "all-minilm",
			expectedCapabilities: []string{"embeddings", "similarity", "vector-search"},
		},
		{
			name:                 "Code model by name",
			modelType:            "llm",
			modelName:            "codellama-34b",
			expectedCapabilities: []string{"text-generation", "chat", "completion", "code-generation", "programming", "code-completion"},
		},
		{
			name:                 "Instruct model",
