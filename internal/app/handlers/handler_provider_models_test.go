package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// TestProviderSpecificModelEndpoints tests the provider-specific model listing endpoints
func TestProviderSpecificModelEndpoints(t *testing.T) {
	// Setup logger
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	styledLogger := logger.NewPlainStyledLogger(log)

	// Create test endpoints
	ollamaURL, _ := url.Parse("http://ollama:11434")
	lmstudioURL, _ := url.Parse("http://lmstudio:1234")
	openaiURL, _ := url.Parse("http://openai:8080")
	vllmURL, _ := url.Parse("http://vllm:8000")
	lemonadeURL, _ := url.Parse("http://lemonade:8000")

	endpoints := []*domain.Endpoint{
		{
			Name:      "ollama-1",
			URL:       ollamaURL,
			URLString: ollamaURL.String(),
			Status:    domain.StatusHealthy,
			Type:      "ollama",
		},
		{
			Name:      "lmstudio-1",
			URL:       lmstudioURL,
			URLString: lmstudioURL.String(),
			Status:    domain.StatusHealthy,
			Type:      "lm-studio",
		},
		{
			Name:      "openai-1",
			URL:       openaiURL,
			URLString: openaiURL.String(),
			Status:    domain.StatusHealthy,
			Type:      "openai",
		},
		{
			Name:      "vllm-1",
			URL:       vllmURL,
			URLString: vllmURL.String(),
			Status:    domain.StatusHealthy,
			Type:      "vllm",
		},
		{
			Name:      "lemonade-1",
			URL:       lemonadeURL,
			URLString: lemonadeURL.String(),
			Status:    domain.StatusHealthy,
			Type:      "lemonade",
		},
	}

	// Create registry and register models
	unifiedRegistry := registry.NewUnifiedMemoryModelRegistry(styledLogger, nil, nil, nil)
	ctx := context.Background()

	// Register endpoints
	for _, ep := range endpoints {
		unifiedRegistry.RegisterEndpoint(ep)
	}

	// Register models for each provider
	ollamaModels := []*domain.ModelInfo{{Name: "llama3:latest"}, {Name: "mistral:latest"}}
	lmstudioModels := []*domain.ModelInfo{{Name: "TheBloke/Llama-2-7B-Chat-GGUF"}}
	openaiModels := []*domain.ModelInfo{{Name: "gpt-3.5-turbo"}}
	vllmModels := []*domain.ModelInfo{{Name: "meta-llama/Llama-2-7b-hf"}}
	lemonadeModels := []*domain.ModelInfo{{Name: "Qwen2.5-0.5B-Instruct-CPU"}}

	require.NoError(t, unifiedRegistry.RegisterModelsWithEndpoint(ctx, endpoints[0], ollamaModels))
	require.NoError(t, unifiedRegistry.RegisterModelsWithEndpoint(ctx, endpoints[1], lmstudioModels))
	require.NoError(t, unifiedRegistry.RegisterModelsWithEndpoint(ctx, endpoints[2], openaiModels))
	require.NoError(t, unifiedRegistry.RegisterModelsWithEndpoint(ctx, endpoints[3], vllmModels))
	require.NoError(t, unifiedRegistry.RegisterModelsWithEndpoint(ctx, endpoints[4], lemonadeModels))

	// Wait for async unification (RegisterModels kicks off unifyModelsAsync in
	// a goroutine) to complete for all 5 endpoints - poll for the expected
	// unified model count rather than guessing how long that takes.
	wantModels := len(ollamaModels) + len(lmstudioModels) + len(openaiModels) + len(vllmModels) + len(lemonadeModels)
	require.Eventually(t, func() bool {
		models, err := unifiedRegistry.GetUnifiedModels(ctx)
		return err == nil && len(models) >= wantModels
	}, 5*time.Second, 10*time.Millisecond, "async unification did not complete")

	// Create converter factory
	converterFactory := converter.NewConverterFactory()

	// Create application
	app := &Application{
		modelRegistry:    unifiedRegistry,
		repository:       &mockRepository{endpoints: endpoints},
		converterFactory: converterFactory,
		logger:           styledLogger,
	}

	tests := []struct {
		name           string
		endpoint       string
		handler        http.HandlerFunc
		expectedFormat string
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "ollama_native_format",
			endpoint:       "/olla/ollama/api/tags",
			handler:        app.ollamaModelsHandler,
			expectedFormat: "ollama",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.OllamaModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Len(t, response.Models, 2)
				// Check for ollama-specific fields
				assert.NotEmpty(t, response.Models[0].Name)
				assert.NotEmpty(t, response.Models[0].Model)
			},
		},
		{
			name:           "ollama_openai_format",
			endpoint:       "/olla/ollama/v1/models",
			handler:        app.ollamaOpenAIModelsHandler,
			expectedFormat: "openai",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.OpenAIModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Equal(t, "list", response.Object)
				assert.Len(t, response.Data, 2)
				// Check for openai-specific fields
				assert.NotEmpty(t, response.Data[0].ID)
				assert.Equal(t, "model", response.Data[0].Object)
			},
		},
		{
			name:           "lmstudio_openai_format",
			endpoint:       "/olla/lmstudio/v1/models",
			handler:        app.lmstudioOpenAIModelsHandler,
			expectedFormat: "openai",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.OpenAIModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Equal(t, "list", response.Object)
				assert.Len(t, response.Data, 1)
				assert.Equal(t, "TheBloke/Llama-2-7B-Chat-GGUF", response.Data[0].ID)
			},
		},
		{
			name:           "lmstudio_enhanced_format",
			endpoint:       "/olla/lmstudio/api/v0/models",
			handler:        app.lmstudioEnhancedModelsHandler,
			expectedFormat: "lmstudio",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.LMStudioModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Equal(t, "list", response.Object)
				assert.Len(t, response.Data, 1)
				// Check for lmstudio-specific fields
				assert.NotEmpty(t, response.Data[0].ID)
				assert.NotEmpty(t, response.Data[0].Type)
			},
		},
		{
			name:           "openai_format",
			endpoint:       "/olla/openai/v1/models",
			handler:        app.openaiModelsHandler,
			expectedFormat: "openai",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.OpenAIModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Equal(t, "list", response.Object)
				// openai provider accepts all OpenAI-compatible endpoints
				// so we should get models from ollama, lmstudio, openai, vllm, and lemonade
				assert.Len(t, response.Data, 6)
				// verify all expected models are present
				modelIDs := make([]string, len(response.Data))
				for i, model := range response.Data {
					modelIDs[i] = model.ID
				}
				assert.Contains(t, modelIDs, "gpt-3.5-turbo")
				assert.Contains(t, modelIDs, "llama3:latest")
				assert.Contains(t, modelIDs, "TheBloke/Llama-2-7B-Chat-GGUF")
				assert.Contains(t, modelIDs, "meta-llama/Llama-2-7b-hf")
				assert.Contains(t, modelIDs, "Qwen2.5-0.5B-Instruct-CPU")
			},
		},
		{
			name:           "vllm_format",
			endpoint:       "/olla/vllm/v1/models",
			handler:        app.genericProviderModelsHandler("vllm", "openai"),
			expectedFormat: "openai",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.OpenAIModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Equal(t, "list", response.Object)
				assert.Len(t, response.Data, 1)
				assert.Equal(t, "meta-llama/Llama-2-7b-hf", response.Data[0].ID)
			},
		},
		{
			name:           "lemonade_native_format",
			endpoint:       "/olla/lemonade/api/v1/models",
			handler:        app.genericProviderModelsHandler("lemonade", "openai"),
			expectedFormat: "openai",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.OpenAIModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Equal(t, "list", response.Object)
				assert.Len(t, response.Data, 1)
				assert.Equal(t, "Qwen2.5-0.5B-Instruct-CPU", response.Data[0].ID)
			},
		},
		{
			name:           "lemonade_openai_format",
			endpoint:       "/olla/lemonade/v1/models",
			handler:        app.genericProviderModelsHandler("lemonade", "openai"),
			expectedFormat: "openai",
			checkResponse: func(t *testing.T, body []byte) {
				var response converter.OpenAIModelResponse
				require.NoError(t, json.Unmarshal(body, &response))
				assert.Equal(t, "list", response.Object)
				assert.Len(t, response.Data, 1)
				assert.Equal(t, "Qwen2.5-0.5B-Instruct-CPU", response.Data[0].ID)
			},
		},
	}

