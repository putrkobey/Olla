package unifier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func unifyOn(t *testing.T, u ports.ModelUnifier, endpointType, url string, names ...string) {
	t.Helper()
	models := make([]*domain.ModelInfo, 0, len(names))
	for _, n := range names {
		models = append(models, &domain.ModelInfo{Name: n})
	}
	_, err := u.UnifyModels(context.Background(), models, &domain.Endpoint{URLString: url, Name: url, Type: endpointType})
	require.NoError(t, err)
}

// Exact first, Ollama only: a bare name must never reach a model another
// platform serves, because that platform would be sent a name it does not know.
func TestCatalogResolveByName_OllamaDefaultTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		query  string
		wantID string // empty means no match
		setup  func(t *testing.T, u ports.ModelUnifier)
	}{
		{"bare resolves to ollama latest", "all-minilm", "all-minilm:latest", func(t *testing.T, u ports.ModelUnifier) {
			unifyOn(t, u, "ollama", "http://o:11434", "all-minilm:latest")
		}},
		{"upper case resolves", "ALL-MINILM", "all-minilm:latest", func(t *testing.T, u ports.ModelUnifier) {
			unifyOn(t, u, "ollama", "http://o:11434", "all-minilm:latest")
