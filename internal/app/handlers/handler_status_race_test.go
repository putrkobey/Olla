package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestModelsStatusHandler_Concurrent verifies that concurrent requests to
// /internal/status/models produce valid, independent responses with no data
// races. The race detector must flag any shared-state violation on the slices
// and maps that were previously package-level globals.
func TestModelsStatusHandler_Concurrent(t *testing.T) {
	t.Parallel()

	endpoints := []*domain.Endpoint{
		{
			Name:      "ollama-local",
			Type:      "ollama",
			URLString: "http://localhost:11434",
			Status:    domain.StatusHealthy,
			Priority:  1,
		},
		{
			Name:      "lmstudio-local",
			Type:      "lm-studio",
			URLString: "http://localhost:1234",
			Status:    domain.StatusHealthy,
			Priority:  2,
		},
	}

	// Populate the model registry with a couple of models so the code paths
	// that build summaries and group by family are exercised.
	family := "llama"
	paramSize := "7B"
	quant := "Q4_K_M"
	modelInfo := &domain.ModelInfo{
