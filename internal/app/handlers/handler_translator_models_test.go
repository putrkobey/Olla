package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestTranslatorModelsHandler_Success(t *testing.T) {
	// Create test application with models
	mockReg := &mockTranslatorRegistry{
		models: []*domain.UnifiedModel{
			{
				ID: "claude/3:opus",
				Aliases: []domain.AliasEntry{
					{Name: "claude-3-opus-20240229", Source: "anthropic"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{EndpointURL: "http://localhost:8080"},
				},
			},
			{
				ID: "claude/3:sonnet",
				Aliases: []domain.AliasEntry{
					{Name: "claude-3-sonnet-20240229", Source: "anthropic"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{EndpointURL: "http://localhost:8080"},
				},
			},
		},
	}

	app := &Application{
		logger:           &mockStyledLogger{},
		modelRegistry:    mockReg,
		discoveryService: &mockDiscoveryService{},
		repository:       &mockEndpointRepository{},
	}

	// Create Anthropic translator with test config
	testConfig := config.AnthropicTranslatorConfig{
		Enabled:        true,
		MaxMessageSize: 10 << 20, // 10MB
	}
	trans, err := anthropic.NewTranslator(app.logger, testConfig)
	if err != nil {
		t.Fatalf("failed to create translator: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/olla/anthropic/v1/models", nil)
	w := httptest.NewRecorder()

