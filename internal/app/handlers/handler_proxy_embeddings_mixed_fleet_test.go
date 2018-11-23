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
