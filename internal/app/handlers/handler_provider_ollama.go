package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// ollamaModelsHandler returns models from all healthy ollama instances
// endpoint: GET /olla/ollama/api/tags
func (a *Application) ollamaModelsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	models, err := a.getProviderModels(ctx, "ollama")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response, err := a.convertModelsToProviderFormat(models, "ollama")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ollamaModelShowHandler handles model detail requests.
// endpoint: POST /olla/ollama/api/show
//
// aggregating model details across instances presents challenges:
