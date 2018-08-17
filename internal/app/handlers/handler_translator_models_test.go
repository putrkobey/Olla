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

	// Call handler
	handler := app.translatorModelsHandler(trans)
	handler(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify response format
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify data field exists
	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("expected data field to be array")
	}

	// Verify we have models
	if len(data) != 2 {
		t.Errorf("expected 2 models, got %d", len(data))
	}

	// Verify the response carries Anthropic-compliant envelope fields.
	if _, ok := response["has_more"]; !ok {
		t.Error("expected has_more field in response envelope")
	}
	if _, ok := response["first_id"]; !ok {
		t.Error("expected first_id field in response envelope")
	}
	if _, ok := response["last_id"]; !ok {
		t.Error("expected last_id field in response envelope")
	}

	// Verify per-entry shape matches the Anthropic spec wire format.
	if len(data) > 0 {
		model := data[0].(map[string]interface{})

		// "type" must be "model" — the Anthropic SDK uses it as a deserialise discriminator.
		if modelType, ok := model["type"].(string); !ok || modelType != "model" {
			t.Errorf("expected type \"model\", got %q", model["type"])
		}

		// "display_name" replaces "name".
		if _, ok := model["display_name"]; !ok {
			t.Error("expected display_name field")
		}
		if _, present := model["name"]; present {
			t.Error("unexpected legacy name field; Anthropic spec uses display_name")
		}

		// "created_at" must be an RFC3339 string, not a unix int.
		createdAt, ok := model["created_at"].(string)
		if !ok {
			t.Errorf("expected created_at to be a string, got %T", model["created_at"])
		} else if _, parseErr := time.Parse(time.RFC3339, createdAt); parseErr != nil {
			t.Errorf("created_at %q is not valid RFC3339: %v", createdAt, parseErr)
		}
		if _, present := model["created"]; present {
