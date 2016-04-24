package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// fleetEndpoint describes one backend in a routing-matrix fleet. modelsJSON is
// fed through the shipped profile's real parser, so typing and tagging come from
// the same code path production discovery uses.
type fleetEndpoint struct {
	name       string
	typ        string
	modelsJSON string
	// offline hosts keep their discovered models, as a real endpoint that has
	// stopped answering health checks does, but are not routable
	offline bool
	// busy hosts are left out of the healthy list, as GetHealthyEndpoints does
	busy bool
}

const (
	ollamaTagsEmbedAndVision = `{"models":[
		{"name":"all-minilm:latest","details":{"family":"bert","parameter_size":"23M"},"capabilities":["embedding"]},
		{"name":"llava:13b","details":{"family":"llava"},"capabilities":["completion","vision"]}]}`
	ollamaTagsVisionAndTools = `{"models":[
		{"name":"qwen3-vl:8b","details":{"family":"qwen3vl"},"capabilities":["completion","vision","tools","thinking"]},
		{"name":"gpt-oss:20b","capabilities":["completion","tools"]}]}`
	ollamaTagsPlainChat = `{"models":[{"name":"llama3.1:8b","capabilities":["completion"]}]}`
	vllmChatModel       = `{"data":[{"id":"Qwen/Qwen3-32B","object":"model"}]}`
	teiInfoBGESmall     = `{"model_id":"BAAI/bge-small-en-v1.5","model_type":{"embedding":{"pooling":"cls"}}}`
	vllmBGE             = `{"data":[{"id":"BAAI/bge-m3","object":"model"}]}`
	vllmRerankModel     = `{"data":[{"id":"BAAI/bge-reranker-v2-m3","object":"model"}]}`
	vllmQwenVL          = `{"data":[{"id":"Qwen/Qwen2.5-VL-7B-Instruct","object":"model"}]}`
	lmStudioNomic       = `{"data":[{"id":"text-embedding-nomic-embed-text-v1.5","object":"model","type":"embeddings","publisher":"nomic-ai","arch":"nomic-bert","compatibility_type":"gguf","quantization":"Q4_K_M","state":"not-loaded","max_context_length":2048}]}`
	teiInfoReranker     = `{"model_id":"/data/bge-reranker-base","served_model_name":"BAAI/bge-reranker-base","model_type":{"reranker":{}}}`
	teiInfoClassify     = `{"model_id":"/data/roberta-go-emotions","served_model_name":"SamLowe/roberta-base-go_emotions","model_type":{"classifier":{"id2label":{"0":"joy"}}}}`
	teiInfoSnowflake    = `{"model_id":"Snowflake/snowflake-arctic-embed-l-v2.0","model_type":{"embedding":{"pooling":"cls"}}}`
)

// TestProxyRouting_MixedFleetMatrix guards the regression where the capability
// stage filtered endpoints to those hosting ANY model carrying the required tag,
// regardless of which model the caller named. A tag on one backend's model then
// evicted every other backend, so strict routing 503ed (or optimistic fallback
// misrouted) a perfectly hosted, untagged model. A named model must reach a host
// of that model; model-less requests are still steered by capability.
func TestProxyRouting_MixedFleetMatrix(t *testing.T) {
	t.Parallel()

	profileFactory, chain, styledLog := mixedFleetDeps(t)

	const (
		embeddingsPath = "/olla/proxy/v1/embeddings"
		chatPath       = "/olla/proxy/v1/chat/completions"
		visionContent  = `[{"type":"text","text":"what is this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]`
	)
	embedBody := func(model string) string {
		if model == "" {
			return `{"input":"hello"}`
		}
		return `{"model":"` + model + `","input":"hello"}`
	}
	const (
		rerankBody = `{"query":"what is deep learning","texts":["deep learning is a subset of ml","paris is in france"]}`
		inputsBody = `{"inputs":"hello"}`
	)
	visionBody := func(model string) string {
		if model == "" {
			return `{"messages":[{"role":"user","content":` + visionContent + `}]}`
		}
		return `{"model":"` + model + `","messages":[{"role":"user","content":` + visionContent + `}]}`
	}

	ollamaEmbed := fleetEndpoint{name: "ollama-1", typ: domain.ProfileOllama, modelsJSON: ollamaTagsEmbedAndVision}
	vllmEmbed := fleetEndpoint{name: "vllm-1", typ: domain.ProfileVLLM, modelsJSON: vllmBGE}
	vllmChat := fleetEndpoint{name: "vllm-chat", typ: domain.ProfileVLLM, modelsJSON: vllmChatModel}
	vllmRerank := fleetEndpoint{name: "vllm-rerank", typ: domain.ProfileVLLM, modelsJSON: vllmRerankModel}
	vllmBGEHost := fleetEndpoint{name: "vllm-embed", typ: domain.ProfileVLLM, modelsJSON: vllmBGE}
	vllmVision := fleetEndpoint{name: "vllm-1", typ: domain.ProfileVLLM, modelsJSON: vllmQwenVL}
	tei := fleetEndpoint{name: "tei-1", typ: constants.ProviderTypeTextEmbeddingsInference, modelsJSON: teiInfoSnowflake}
	teiEmbed := fleetEndpoint{name: "tei-embed", typ: constants.ProviderTypeTextEmbeddingsInference, modelsJSON: teiInfoSnowflake}
	teiRerank := fleetEndpoint{name: "tei-rerank", typ: constants.ProviderTypeTextEmbeddingsInference, modelsJSON: teiInfoReranker}
	teiClassify := fleetEndpoint{name: "tei-classify", typ: constants.ProviderTypeTextEmbeddingsInference, modelsJSON: teiInfoClassify}
	teiTrio := []fleetEndpoint{teiEmbed, teiRerank, teiClassify}
	lmStudio := fleetEndpoint{name: "lmstudio-1", typ: constants.ProviderTypeLMStudio, modelsJSON: lmStudioNomic}

	toolsBody := func(model string) string {
		const tools = `"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]`
		if model == "" {
			return `{"messages":[{"role":"user","content":"hi"}],` + tools + `}`
		}
		return `{"model":"` + model + `","messages":[{"role":"user","content":"hi"}],` + tools + `}`
	}
	ollamaVision := fleetEndpoint{name: "ollama-1", typ: domain.ProfileOllama, modelsJSON: ollamaTagsVisionAndTools}
	ollamaPlain := fleetEndpoint{name: "ollama-1", typ: domain.ProfileOllama, modelsJSON: ollamaTagsPlainChat}
	ollamaPlain2 := fleetEndpoint{name: "ollama-2", typ: domain.ProfileOllama, modelsJSON: ollamaTagsPlainChat}
	vllmChatHost := fleetEndpoint{name: "vllm-chat", typ: domain.ProfileVLLM, modelsJSON: vllmChatModel}

	tests := []struct {
		name         string
		path         string
		body         string
		wantEndpoint string // empty means every host in the fleet stays a candidate
		fleet        []fleetEndpoint
	}{
		{"ollama-tagged embedding reaches ollama beside vllm", embeddingsPath, embedBody("all-minilm:latest"), "ollama-1", []fleetEndpoint{ollamaEmbed, vllmEmbed}},
		{"named vllm embedding reaches vllm beside tagged ollama", embeddingsPath, embedBody("BAAI/bge-m3"), "vllm-1", []fleetEndpoint{ollamaEmbed, vllmEmbed}},
		{"ollama-tagged embedding reaches ollama beside tei", embeddingsPath, embedBody("all-minilm:latest"), "ollama-1", []fleetEndpoint{ollamaEmbed, tei}},
		{"tei model reaches tei beside tagged ollama", embeddingsPath, embedBody("Snowflake/snowflake-arctic-embed-l-v2.0"), "tei-1", []fleetEndpoint{ollamaEmbed, tei}},
		{"tei model reaches tei beside vllm", embeddingsPath, embedBody("Snowflake/snowflake-arctic-embed-l-v2.0"), "tei-1", []fleetEndpoint{tei, vllmEmbed}},
		{"vllm embedding reaches vllm beside tei", embeddingsPath, embedBody("BAAI/bge-m3"), "vllm-1", []fleetEndpoint{tei, vllmEmbed}},
		{"lm studio embedding reaches lm studio beside vllm", embeddingsPath, embedBody("text-embedding-nomic-embed-text-v1.5"), "lmstudio-1", []fleetEndpoint{lmStudio, vllmEmbed}},
		{"vllm embedding reaches vllm beside lm studio", embeddingsPath, embedBody("BAAI/bge-m3"), "vllm-1", []fleetEndpoint{lmStudio, vllmEmbed}},
		{
			"hybrid ollama embedding model beside tei reaches ollama", embeddingsPath, embedBody("embeddinggemma:latest"), "ollama-1",
			[]fleetEndpoint{{name: "ollama-1", typ: domain.ProfileOllama, modelsJSON: `{"models":[{"name":"embeddinggemma:latest","capabilities":["embedding","completion"]}]}`}, tei},
		},
		{
			"capability-less old ollama embedding model beside tei reaches ollama", embeddingsPath, embedBody("nomic-embed-text:latest"), "ollama-1",
			[]fleetEndpoint{{name: "ollama-1", typ: domain.ProfileOllama, modelsJSON: `{"models":[{"name":"nomic-embed-text:latest"}]}`}, tei},
		},
		{"untagged vision model reaches its host beside llava host", chatPath, visionBody("Qwen/Qwen2.5-VL-7B-Instruct"), "vllm-1", []fleetEndpoint{ollamaEmbed, vllmVision}},
		{"llava reaches ollama beside untagged vision host", chatPath, visionBody("llava:13b"), "ollama-1", []fleetEndpoint{ollamaEmbed, vllmVision}},
		// TEI native bodies carry no model field: the path picks the kind. /predict is
		// served by classifiers and rerankers alike, so those fleets omit the reranker.
		{"model-less rerank goes to the reranker", "/olla/proxy/rerank", rerankBody, "tei-rerank", teiTrio},
		{"model-less predict goes to the classifier", "/olla/proxy/predict", inputsBody, "tei-classify", []fleetEndpoint{teiEmbed, teiClassify}},
		// TEI /predict also serves rerankers, so an embedder + reranker fleet must not fall back to the embedder (424)
		{"model-less predict reaches the reranker beside an embedder", "/olla/proxy/predict", inputsBody, "tei-rerank", []fleetEndpoint{teiEmbed, teiRerank}},
		{"model-less embed goes to the embedder", "/olla/proxy/embed", inputsBody, "tei-embed", teiTrio},
		{"model-less embed_all goes to the embedder", "/olla/proxy/embed_all", inputsBody, "tei-embed", teiTrio},
		{"tei provider route rerank goes to the reranker", "/olla/tei/rerank", rerankBody, "tei-rerank", teiTrio},
		{"tei provider route predict goes to the classifier", "/olla/tei/predict", inputsBody, "tei-classify", []fleetEndpoint{teiEmbed, teiClassify}},
		{"named embedding model reaches the embedder among tei kinds", embeddingsPath, embedBody("Snowflake/snowflake-arctic-embed-l-v2.0"), "tei-embed", teiTrio},
		{"model-less rerank beside vllm still reaches the reranker", "/olla/proxy/rerank", rerankBody, "tei-rerank", []fleetEndpoint{vllmEmbed, teiEmbed, teiRerank}},
		{"model-less embeddings steered to a real lm studio embedding model beside vllm", embeddingsPath, embedBody(""), "lmstudio-1", []fleetEndpoint{lmStudio, vllmChat}},
		{"model-less embeddings steered to tagged embedding host", embeddingsPath, embedBody(""), "ollama-1", []fleetEndpoint{ollamaEmbed, vllmChat}},
		// vLLM reports no model kind, so these hosts are tagged from the model name alone
		{"model-less embeddings steered to a name-tagged vllm embedder beside vllm chat", embeddingsPath, embedBody(""), "vllm-embed", []fleetEndpoint{vllmChat, vllmBGEHost}},
		{"model-less embeddings steered to a name-tagged vllm embedder beside an untagged ollama chat host", embeddingsPath, embedBody(""), "vllm-embed", []fleetEndpoint{{name: "ollama-1", typ: domain.ProfileOllama, modelsJSON: `{"models":[{"name":"llama3.1:8b"}]}`}, vllmBGEHost}},
		{"model-less v1 rerank steered to the vllm reranker beside a vllm embedder", "/olla/proxy/v1/rerank", rerankBody, "vllm-rerank", []fleetEndpoint{vllmBGEHost, vllmRerank}},
		{"model-less vision steered to vision-tagged host", chatPath, visionBody(""), "ollama-1", []fleetEndpoint{ollamaEmbed, vllmVision}},
		// Ollama reports vision and tools in /api/tags, so those models carry the tags
		{"model-less vision steered to an ollama vision model beside an untagged vllm chat host", chatPath, visionBody(""), "ollama-1", []fleetEndpoint{ollamaVision, vllmChatHost}},
		{"model-less vision keeps every host when no model anywhere has vision", chatPath, visionBody(""), "", []fleetEndpoint{ollamaPlain, vllmChatHost}},
		{"model-less tools steered to an ollama tools model beside an untagged vllm chat host", chatPath, toolsBody(""), "ollama-1", []fleetEndpoint{ollamaVision, vllmChatHost}},
		{"model-less tools keeps every host when no model anywhere declares tools", chatPath, toolsBody(""), "", []fleetEndpoint{ollamaPlain, vllmChatHost}},
		{"named vllm model with tools is not evicted by ollama tools models", chatPath, toolsBody("Qwen/Qwen3-32B"), "vllm-chat", []fleetEndpoint{ollamaVision, vllmChatHost}},
		// provider routes confine candidates with ProviderScope before the capability stage
		{"model-less tools on the ollama provider route reaches the ollama tools host", "/olla/ollama/v1/chat/completions", toolsBody(""), "ollama-1", []fleetEndpoint{ollamaVision, ollamaPlain2, vllmChatHost}},
		{"named tools model on the ollama provider route reaches its host", "/olla/ollama/v1/chat/completions", toolsBody("gpt-oss:20b"), "ollama-1", []fleetEndpoint{ollamaVision, vllmChatHost}},
		{"named vision request for a vllm model is not evicted by ollama vision models", chatPath, visionBody("Qwen/Qwen3-32B"), "vllm-chat", []fleetEndpoint{ollamaVision, vllmChatHost}},
	}

	strategies := []struct {
		name string
		cfg  config.ModelRoutingStrategy
	}{
		{"strict", config.ModelRoutingStrategy{Type: "strict"}},
		{"optimistic-compatible-only", config.ModelRoutingStrategy{Type: "optimistic", Options: config.ModelRoutingStrategyOptions{FallbackBehavior: constants.FallbackBehaviorCompatibleOnly}}},
		{"optimistic-all", config.ModelRoutingStrategy{Type: "optimistic", Options: config.ModelRoutingStrategyOptions{FallbackBehavior: constants.FallbackBehaviorAll}}},
	}

	for _, tt := range tests {
		for _, strategy := range strategies {
			t.Run(tt.name+"/"+strategy.name, func(t *testing.T) {
				t.Parallel()

				w, selected := runMixedFleet(t, profileFactory, chain, styledLog, strategy.cfg, tt.fleet, tt.path, tt.body)

				require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
				require.NotEmpty(t, selected, "proxy engine must receive candidate endpoints")
				if tt.wantEndpoint == "" {
					var got, want []string
					for _, ep := range selected {
						got = append(got, ep.Name)
					}
					for _, fe := range tt.fleet {
						want = append(want, fe.name)
					}
					assert.ElementsMatch(t, want, got)
					return
				}
				for _, ep := range selected {
					assert.Equal(t, tt.wantEndpoint, ep.Name)
				}
			})
		}
	}
}

// runMixedFleet registers fleet in a real unified registry through the shipped profile
// parsers, sends one request through the proxy handler and returns the response and the
// endpoints the proxy engine was handed. Offline hosts keep their discovered models.
func runMixedFleet(t *testing.T, profileFactory profile.ProfileFactory, chain *inspector.Chain, styledLog logger.StyledLogger,
	strategyCfg config.ModelRoutingStrategy, fleet []fleetEndpoint, path, body string) (*httptest.ResponseRecorder, []*domain.Endpoint) {
	t.Helper()

	ctx := context.Background()
	cfg := strategyCfg
	unified := registry.NewUnifiedMemoryModelRegistry(styledLog, nil, &cfg, nil)

	healthy := make([]*domain.Endpoint, 0, len(fleet))
	all := make([]*domain.Endpoint, 0, len(fleet))
	wantModels := 0
	for _, fe := range fleet {
		u, perr := url.Parse("http://" + fe.name + ":8080")
		require.NoError(t, perr)
		ep := &domain.Endpoint{Name: fe.name, URL: u, URLString: u.String(), Type: fe.typ, Status: domain.StatusHealthy}
		all = append(all, ep)
		switch {
		case fe.offline:
			ep.Status = domain.StatusOffline
		case fe.busy:
			ep.Status = domain.StatusBusy
		default:
			healthy = append(healthy, ep)
		}

		prof, perr := profileFactory.GetProfile(fe.typ)
		require.NoError(t, perr)
		models, perr := prof.ParseModelsResponse([]byte(fe.modelsJSON))
		require.NoError(t, perr)
		wantModels += len(models)

		unified.RegisterEndpoint(ep)
		require.NoError(t, unified.RegisterModelsWithEndpoint(ctx, ep, models))
	}
	require.Eventually(t, func() bool {
		models, gerr := unified.GetUnifiedModels(ctx)
		return gerr == nil && len(models) >= wantModels
	}, 5*time.Second, 10*time.Millisecond, "async unification did not complete")

	var selected []*domain.Endpoint
	app := &Application{
		Config:         &config.Config{Server: config.ServerConfig{RateLimits: config.ServerRateLimits{}}},
		logger:         styledLog,
		profileFactory: profileFactory,
		inspectorChain: chain,
		discoveryService: &mockDiscoveryServiceWithFunc{getHealthyEndpointsFunc: func(context.Context) ([]*domain.Endpoint, error) { return healthy, nil },
			getEndpointsFunc: func(context.Context) ([]*domain.Endpoint, error) { return all, nil },
		},
		modelRegistry: unified,
		proxyService: &mockProxyService{
			proxyFunc: func(_ context.Context, w http.ResponseWriter, _ *http.Request, eps []*domain.Endpoint, _ *ports.RequestStats, _ logger.StyledLogger) error {
				selected = eps
				w.WriteHeader(http.StatusOK)
				return nil
			},
		},
		StartTime: time.Now(),
	}

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	if strings.HasPrefix(path, "/olla/proxy/") {
		req = req.WithContext(context.WithValue(req.Context(), constants.ContextRoutePrefixKey, "/olla/proxy/"))
	}
	w := httptest.NewRecorder()

	if !strings.HasPrefix(path, "/olla/proxy/") {
		app.providerProxyHandler(w, req)
	} else {
		app.proxyHandler(w, req)
	}

	return w, selected
}

// mixedFleetDeps builds the shipped profiles and the path/body inspector chain the
// fleet tests route through.
func mixedFleetDeps(t *testing.T) (profile.ProfileFactory, *inspector.Chain, *mockStyledLogger) {
	t.Helper()
	styledLog := &mockStyledLogger{}

	profileFactory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err)

	inspectorFactory := inspector.NewFactory(profileFactory, styledLog)
	bodyInspector, err := inspectorFactory.CreateBodyInspector()
	require.NoError(t, err)
	chain := inspectorFactory.CreateChain()
	chain.AddInspector(inspectorFactory.CreatePathInspector())
	chain.AddInspector(bodyInspector)
	return profileFactory, chain, styledLog
}

// When a capability exists in the fleet but every host with it is down, a model-less
// request is refused with a clear 503. Falling back to "everything" sprayed it at hosts
// that serve a different kind: with the only embedder stopped, /embed went to the
// reranker and classifier and TEI answered 424 for most of 1500 requests.
func TestProxyRouting_CapabilityUnavailable(t *testing.T) {
