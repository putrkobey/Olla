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

