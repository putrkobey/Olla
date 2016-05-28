package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// SGLang and LMDeploy ship with anthropic_support enabled. These tests run the
// shipped profiles through the real inspector chain so the mode header shows
// whether the profile took effect, rather than the request being quietly translated.

const (
	nativeAnthropicPlainBody      = `{"model":"native-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	nativeAnthropicToolChoiceBody = `{"model":"native-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"tool_choice":{"type":"none"}}`
	nativeAnthropicToolsBody      = `{"model":"native-model","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"get_weather","description":"w","input_schema":{"type":"object"}}]}`
)

// postNativeAnthropic sends body to a fleet of an Ollama host plus one host of
// typ, serving native-model only on the latter, and returns the recorder, the
// endpoint names offered to the proxy and the path it was asked to forward.
func postNativeAnthropic(t *testing.T, typ, body string, passthrough bool) (*httptest.ResponseRecorder, []string, string) {
	t.Helper()

	factory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err)
	log := &mockStyledLogger{}
	pathInspector := inspector.NewPathInspector(factory, log)
	chain := inspector.NewChain(log)
	chain.AddInspector(pathInspector)
	bodyInspector, err := inspector.NewBodyInspector(log)
	require.NoError(t, err)
	chain.AddInspector(bodyInspector)
