package inspector

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/logger"
)

func BenchmarkChain_WithAndWithoutBodyInspector(b *testing.B) {
	ctx := context.Background()
	logCfg := &logger.Config{Level: "error", PrettyLogs: false}
	log, _, err := logger.New(logCfg)
	require.NoError(b, err)
	styledLog := &mockStyledLogger{underlying: log}

	// Create profile factory
	profileFactory, err := profile.NewFactoryWithDefaults()
	require.NoError(b, err)

	requestBody := `{"model": "llama3.1:8b", "messages": [{"role": "user", "content": "Hello world"}]}`

	b.Run("WithPathInspectorOnly", func(b *testing.B) {
		// Create chain with only path inspector
		factory := NewFactory(profileFactory, styledLog)
		chain := factory.CreateChain()
		pathInspector := factory.CreatePathInspector()
		chain.AddInspector(pathInspector)

		b.ResetTimer()
