package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// openaiModelsHandler returns models from endpoints that implement openai's api.
// these are typically local inference servers (localai, text-generation-webui, etc)
// that chose openai's api format for compatibility
// endpoint: GET /olla/openai/v1/models
func (a *Application) openaiModelsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	models, err := a.getProviderModels(ctx, "openai")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
