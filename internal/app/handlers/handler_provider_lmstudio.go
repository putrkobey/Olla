package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// lmstudioOpenAIModelsHandler provides models in openai format.
// lm studio supports multiple api formats for broader client compatibility
// endpoints: GET /olla/lmstudio/v1/models or /olla/lmstudio/api/v1/models
func (a *Application) lmstudioOpenAIModelsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
