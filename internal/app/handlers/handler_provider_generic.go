package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// genericProviderModelsHandler returns a handler function for any provider's model listing
func (a *Application) genericProviderModelsHandler(providerType, format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Normalise provider type
		normalizedProvider := NormaliseProviderType(providerType)

		// Get models from provider
		models, err := a.getProviderModels(ctx, normalizedProvider)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Convert to requested format using the existing converter factory
		response, err := a.convertModelsToProviderFormat(models, format)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// genericModelShowHandler returns model details for any provider
func (a *Application) genericModelShowHandler(providerType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
