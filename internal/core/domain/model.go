package domain

import (
	"context"
	"fmt"
	"time"
)

// ModelDetails holds detailed model metadata for intelligent routing
// based off ollama model details, guessing others won't be this detailed
type ModelDetails struct {
	ParameterSize     *string    `json:"parameter_size,omitempty"`
	QuantizationLevel *string    `json:"quantization_level,omitempty"`
	Publisher         *string    `json:"publisher,omitempty"`
	Type              *string    `json:"type,omitempty"` // from LMStudio: llm, vlm
	Family            *string    `json:"family,omitempty"`
	Format            *string    `json:"format,omitempty"`
	ParentModel       *string    `json:"parent_model,omitempty"`
	State             *string    `json:"state,omitempty"`              // loaded / not-loaded (LMStudio gives this now)
	Digest            *string    `json:"digest,omitempty"`             // super important for comparison checks
	MaxContextLength  *int64     `json:"max_context_length,omitempty"` // Max context length in tokens (LMStudio gives this)
	ModifiedAt        *time.Time `json:"modified_at,omitempty"`
	Checkpoint        *string    `json:"checkpoint,omitempty"` // Lemonade: HuggingFace model path
	Recipe            *string    `json:"recipe,omitempty"`     // Lemonade: inference engine (oga-cpu, oga-npu, llamacpp, flm)
	Families          []string   `json:"families,omitempty"`
	// Capabilities are capabilities the backend declared for the model, already in
	// Olla's vocabulary (vision, function_calling). They are added to what the
	// model's kind and name imply.
	Capabilities []string `json:"capabilities,omitempty"`
}

type ModelInfo struct {
