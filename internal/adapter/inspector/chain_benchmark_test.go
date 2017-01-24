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
		for range b.N {
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(requestBody)))
			req.Header.Set("Content-Type", "application/json")

			profile, err := chain.Inspect(ctx, req, "/v1/chat/completions")
			if err != nil {
				b.Fatal(err)
			}
			if profile.ModelName != "" {
				b.Fatal("Expected no model name with path inspector only")
			}
		}
	})

	b.Run("WithPathAndBodyInspectors", func(b *testing.B) {
		// Create chain with both inspectors
		factory := NewFactory(profileFactory, styledLog)
		chain := factory.CreateChain()
		pathInspector := factory.CreatePathInspector()
		bodyInspector, err := factory.CreateBodyInspector()
		if err != nil {
			b.Fatalf("Failed to create body inspector: %v", err)
		}
		chain.AddInspector(pathInspector)
		chain.AddInspector(bodyInspector)

		b.ResetTimer()
		for range b.N {
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(requestBody)))
			req.Header.Set("Content-Type", "application/json")

			profile, err := chain.Inspect(ctx, req, "/v1/chat/completions")
			if err != nil {
				b.Fatal(err)
			}
			if profile.ModelName != "llama3.1:8b" {
