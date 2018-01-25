<div align="center">
    <img src="assets/images/banner.png" width="480" height="249" alt="Olla - Smart LLM Load Balancer & Proxy" />
  <p>
    <a href="https://github.com/putrkobey/Olla/blob/master/LICENSE"><img src="https://img.shields.io/github/license/putrkobey/Olla" alt="License"></a>
    <a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/putrkobey/Olla" alt="Go"></a>
    <a href="https://github.com/putrkobey/Olla/actions/workflows/ci.yml"><img src="https://github.com/putrkobey/Olla/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI"></a>
    <a href="https://goreportcard.com/report/github.com/putrkobey/Olla"><img src="https://goreportcard.com/badge/github.com/putrkobey/Olla" alt="Go Report Card"></a>
    <a href="https://github.com/putrkobey/Olla/releases/latest"><img src="https://img.shields.io/github/release/putrkobey/Olla" alt="Latest Release"></a> <br />
    <a href="https://github.com/ggerganov/llama.cpp"><img src="https://img.shields.io/badge/llama.cpp-native-lightgreen.svg" alt="llama.cpp: Native Support"></a>
    <a href="https://github.com/vllm-project/vllm"><img src="https://img.shields.io/badge/vLLM-native-lightgreen.svg" alt="vLLM: Native Support"></a>
    <a href="https://github.com/sgl-project/sglang"><img src="https://img.shields.io/badge/SGLang-native-lightgreen.svg" alt="SGLang: Native Support"></a>
    <a href="https://github.com/BerriAI/litellm"><img src="https://img.shields.io/badge/LiteLLM-native-lightgreen.svg" alt="LiteLLM: Native Support"></a>
    <a href="https://github.com/InternLM/lmdeploy"><img src="https://img.shields.io/badge/LM Deploy-native-lightgreen.svg" alt="LM Deploy: Native Support"></a> <br/> 
    <a href="https://github.com/waybarrios/vllm-mlx/"><img src="https://img.shields.io/badge/vLLM--MLX-native-lightgreen.svg" alt="vLLM-MLX: Native Support"></a>
    <a href="https://docs.docker.com/ai/model-runner/"><img src="https://img.shields.io/badge/Docker Model Runner-native-lightgreen.svg" alt="Docker Model Runner: Native Support"></a><br/>
    <a href="https://ollama.com"><img src="https://img.shields.io/badge/Ollama-native-lightgreen.svg" alt="Ollama: Native Support"></a>
    <a href="https://lmstudio.ai/"><img src="https://img.shields.io/badge/LM Studio-native-lightgreen.svg" alt="LM Studio: Native Support"></a>
    <a href="https://github.com/lemonade-sdk/lemonade"><img src="https://img.shields.io/badge/LemonadeSDK-native-lightgreen.svg" alt="LemonadeSDK: Native Support"></a>
    <a href="https://github.com/jundot/omlx"><img src="https://img.shields.io/badge/oMLX-native-lightgreen.svg" alt="oMLX: Native Support"></a>
  </P>
  <p>
    <div align="center">
    <img src="./docs/content/assets/demos/olla-v1.0.x-demo.gif" height="" width="" /><br/>
    <small>Recorded with <a href="https://vhs.charm.sh/">VHS</a> - see <a href="./docs/vhs/demo.tape">demo tape</a></small><br/><br/>
    </div>
    <a href="https://thushan.github.io/olla/"><img src="https://img.shields.io/badge/📖_Documentation-0078D4?style=for-the-badge&logoColor=white" height="32" alt="Documentation"></a>
    &nbsp;
    <a href="https://github.com/putrkobey/Olla/issues"><img src="https://img.shields.io/badge/🐛_Issues-D73502?style=for-the-badge&logoColor=white" height="32" alt="Issues"></a>
    &nbsp;
    <a href="https://github.com/putrkobey/Olla/releases"><img src="https://img.shields.io/badge/🚀_Releases-6f42c1?style=for-the-badge&logoColor=white" height="32" alt="Releases"></a>
  </p>
</div>

[Olla](https://tensorfoundry.io/products/olla) is a high-performance, low-overhead, low-latency proxy and load balancer for managing LLM infrastructure. It intelligently routes LLM requests across self-hosted inference nodes with a [wide variety](https://thushan.github.io/olla/integrations/overview/) of natively supported endpoints and extensible enough to support others. Olla provides model discovery and unified model catalogues within each provider, enabling seamless routing to available models on compatible endpoints.

Olla works alongside API gateways like [LiteLLM](https://github.com/BerriAI/litellm) or orchestration platforms like [GPUStack](https://github.com/gpustack/gpustack), focusing on making your **existing** LLM infrastructure reliable through intelligent routing and failover. You can choose between two proxy engines: **Sherpa** for simplicity and maintainability or **Olla** for maximum performance with advanced features like circuit breakers and connection pooling.

![Olla Single OpenAI](assets/diagrams/features.excalidraw.png)

Single CLI application and config file is all you need to go [Olla](https://tensorfoundry.io/products/olla)!

For large GPU deployments, enterprise and data centre use, see [TensorFoundry FoundryOS](https://tensorfoundry.io/products/foundryos). For an inference control plane, consider [Alloy](https://tensorfoundry.io/products/alloy).

## Key Features

- **🔄 Smart Load Balancing**: [Priority-based routing](https://thushan.github.io/olla/concepts/load-balancing/) with automatic failover and connection retry
- **📌 Sticky Sessions**: [KV-cache-aware affinity routing](https://thushan.github.io/olla/concepts/sticky-sessions/) that pins multi-turn conversations to the same backend
- **🔍 Smart Model Unification**: [Per-provider unification + OpenAI-compatible cross-provider routing](https://thushan.github.io/olla/concepts/model-unification/)
- **⚡ Dual Proxy Engines**: [Sherpa (simple) and Olla (high-performance)](https://thushan.github.io/olla/concepts/proxy-engines/)
- **🎯 Advanced Filtering**: [Profile and model filtering](https://thushan.github.io/olla/configuration/filters/) with glob patterns for precise control
- **💊 Health Monitoring**: [Continuous endpoint health checks](https://thushan.github.io/olla/concepts/health-checking/) with circuit breakers and automatic recovery
- **🔁 Intelligent Retry**: Automatic retry on connection failures with immediate transparent endpoint failover
- **🔧 Self-Healing**: Automatic model discovery refresh when endpoints recover
- **📊 Request Tracking**: Detailed response headers and [statistics](https://thushan.github.io/olla/api-reference/overview/#response-headers)
- **⚡🔄 Anthropic Messages API**: [Passthrough for backends with native support; automatic translation for others](https://thushan.github.io/olla/integrations/api-translation/anthropic/)
- **🔐 Backend Authentication**: [Outbound auth](https://thushan.github.io/olla/configuration/endpoint-auth/) for local backends requiring API keys, bearer tokens, or basic auth
- **🛡️ Production Ready**: Rate limiting, request size limits, optional CORS for browser clients, configurable proxy/server timeouts, graceful shutdown
- **⚡ High Performance**: Sub-millisecond endpoint selection with lock-free atomic stats
- **🎯 LLM-Optimised**: Streaming-first design with optimised timeouts for long inference
- **⚙️ High Performance**: Designed to be very [lightweight & efficient](https://thushan.github.io/olla/configuration/practices/performance/), runs on less than 50Mb RAM.
- **📈 Admin Dashboard**: [Read-only fleet overview](https://thushan.github.io/olla/configuration/dashboard/) embedded in the binary at `/internal/ui/`, on by default and loopback-only

## Herd Dashboard

Olla ships with an embedded (v0.0.29+), read-only dashboard at `/internal/ui/` - fleet health, per-endpoint latency and the discovered model inventory, served straight from the binary. No extra port, no separate web server, on by default and loopback-only.

<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/content/assets/images/dashboard/overview-dark.png">
    <img src="docs/content/assets/images/dashboard/overview-light.png" alt="Olla admin dashboard - fleet overview with live request sparkline, per-endpoint health and latency" width="820">
  </picture>
  <br/>
  <small>Read-only &middot; light &amp; dark &middot; nothing leaves your network &middot; <a href="https://thushan.github.io/olla/configuration/dashboard/">dashboard docs</a></small>
</div>

Start Olla and open [http://localhost:40114/internal/ui/](http://localhost:40114/internal/ui/) to see your herd.

## Supported Backends

Olla proxies the following self-hosted inference backends. The **Anthropic API** column shows whether `/olla/anthropic/` requests are forwarded as-is (native passthrough) or translated from Anthropic format to OpenAI format on the fly.

| Backend | Docs | Anthropic API |
|---------|------|---------------|
| [Ollama](https://ollama.com) | [Integration](https://thushan.github.io/olla/integrations/backend/ollama/) | ⚡ Passthrough (v0.14.0+) |
| [LM Studio](https://lmstudio.ai/) | [Integration](https://thushan.github.io/olla/integrations/backend/lmstudio/) | ⚡ Passthrough (v0.4.1+) |
| [vLLM](https://github.com/vllm-project/vllm) | [Integration](https://thushan.github.io/olla/integrations/backend/vllm/) | ⚡ Passthrough (v0.11.1+) |
| [vLLM-MLX](https://github.com/waybarrios/vllm-mlx/) | [Integration](https://thushan.github.io/olla/integrations/backend/vllm-mlx/) | ⚡ Passthrough |
| [llama.cpp](https://github.com/ggerganov/llama.cpp) | [Integration](https://thushan.github.io/olla/integrations/backend/llamacpp/) | ⚡ Passthrough (b4847+) |
| [Docker Model Runner](https://docs.docker.com/ai/model-runner/) | [Integration](https://thushan.github.io/olla/integrations/backend/docker-model-runner/) | ⚡ Passthrough |
| [oMLX](https://github.com/jundot/omlx) | [Integration](https://thushan.github.io/olla/integrations/backend/omlx/) | ⚡ Passthrough (v0.4.2+) |
| [SGLang](https://github.com/sgl-project/sglang) | [Integration](https://thushan.github.io/olla/integrations/backend/sglang/) | ⚡ Passthrough (v0.5.9+) |
| [LMDeploy](https://github.com/InternLM/lmdeploy) | [Integration](https://thushan.github.io/olla/integrations/backend/lmdeploy/) | ⚡ Passthrough (v0.13.0+) |
| [Lemonade SDK](https://github.com/lemonade-sdk/lemonade) | [Integration](https://thushan.github.io/olla/integrations/backend/lemonade/) | 🔄 Translation |
| [LiteLLM](https://github.com/BerriAI/litellm) | [Integration](https://thushan.github.io/olla/integrations/backend/litellm/) | 🔄 Translation |
| OpenAI-compatible (generic) | [Integration](https://thushan.github.io/olla/integrations/overview/) | 🔄 Translation |
| [Text Embeddings Inference](https://github.com/huggingface/text-embeddings-inference) | [Integration](https://thushan.github.io/olla/integrations/backend/text-embeddings-inference/) | N/A (no chat or text generation) |

> **⚡ Passthrough**: Olla forwards Anthropic-format requests directly; the backend handles them natively, with no translation overhead. \
> **🔄 Translation**: Olla converts Anthropic ↔ OpenAI format automatically; any OpenAI-compatible backend works transparently.

LiteLLM is the recommended bridge when you need Olla to reach hosted cloud APIs (OpenAI, Anthropic, Bedrock, etc.). Learn more 
about how they compare [Olla vs LiteLLM](https://tensorfoundry.io/blog/olla-vs-litellm) and where each fits, where each wins etc.

## Platform Support

Olla runs on multiple platforms and architectures:

| Platform | AMD64 | ARM64 | Notes |
|----------|-------|-------|-------|
| Linux    | ✅ | ✅ | Full support including Raspberry Pi 4+ |
| macOS    | ✅ | ✅ | Intel and Apple Silicon (M1/M2/M3/M4) |
| Windows  | ✅ | ✅ | Windows 10/11 and Windows on ARM |
| Docker   | ✅ | ✅ | Multi-architecture images (amd64/arm64) |

## Quick Start

### Installation

```bash
# Download latest release (auto-detects your platform)
bash <(curl -s https://raw.githubusercontent.com/putrkobey/Olla/main/install.sh)
```
```bash
# Docker (automatically pulls correct architecture)
docker run -t \
  --name olla \
  -p 40114:40114 \
  ghcr.io/putrkobey/Olla:latest

# Or explicitly specify platform (e.g., for ARM64)
docker run --platform linux/arm64 -t \
  --name olla \
  -p 40114:40114 \
  ghcr.io/putrkobey/Olla:latest
```

You can still use the `go install` method, but you won't get the bundled front-end, but you can see 
