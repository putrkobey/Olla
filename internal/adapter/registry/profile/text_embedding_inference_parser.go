package profile

import (
	"errors"
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// TEIInfoResponse is the shape of the /info endpoint served by Hugging Face
// Text Embeddings Inference. Unlike OpenAI-style listing endpoints it describes
// the single model the instance serves rather than a list of models.
type TEIInfoResponse struct {
	ModelType       map[string]any `json:"model_type,omitempty"`
	ModelID         string         `json:"model_id"`
	ServedModelName string         `json:"served_model_name"`
}

// teiModelTypes maps the keys TEI uses in model_type's externally tagged object
// ({"embedding":{...}}, {"reranker":{...}}, {"classifier":{...}}) to Olla model
// types. The embedding kind uses "embeddings", the type LM Studio and Ollama
// already use, so one capability set covers every embedding backend. Anything
// else means a TEI version serving a kind we do not know about.
var teiModelTypes = map[string]string{
	"embedding":  "embeddings",
	"reranker":   "reranker",
	"classifier": "classifier",
}

type textEmbeddingInferenceParser struct{}

func (p *textEmbeddingInferenceParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	var response TEIInfoResponse
	if len(data) > 0 {
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse Text Embeddings Inference response: %w", err)
		}
	}

	// served_model_name is the name the instance actually serves requests under;
	// model_id is the on-disk checkpoint path and only a fallback.
	name := response.ServedModelName
