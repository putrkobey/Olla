package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestBaseConverter_FindProviderAlias(t *testing.T) {
	base := NewBaseConverter("ollama")

	tests := []struct {
		name     string
		model    *domain.UnifiedModel
		wantName string
		wantOk   bool
	}{
		{
			name: "finds ollama alias",
			model: &domain.UnifiedModel{
				Aliases: []domain.AliasEntry{
					{Name: "llama3:latest", Source: "ollama"},
					{Name: "llama3:70b", Source: "generated"},
				},
			},
			wantName: "llama3:latest",
			wantOk:   true,
		},
		{
			name: "no ollama alias",
			model: &domain.UnifiedModel{
				Aliases: []domain.AliasEntry{
					{Name: "microsoft/phi-4", Source: "lmstudio"},
				},
			},
			wantName: "",
			wantOk:   false,
		},
		{
			name: "empty aliases",
			model: &domain.UnifiedModel{
				Aliases: []domain.AliasEntry{},
			},
			wantName: "",
			wantOk:   false,
		},
		{
			name: "nil aliases",
			model: &domain.UnifiedModel{
				Aliases: nil,
			},
			wantName: "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotOk := base.FindProviderAlias(tt.model)
			assert.Equal(t, tt.wantName, gotName)
			assert.Equal(t, tt.wantOk, gotOk)
		})
	}
}

func TestBaseConverter_FindProviderEndpoint(t *testing.T) {
	base := NewBaseConverter("ollama")

	tests := []struct {
		name         string
		model        *domain.UnifiedModel
		providerName string
		wantNil      bool
		wantEndpoint string
	}{
		{
			name: "finds matching endpoint",
			model: &domain.UnifiedModel{
