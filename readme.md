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
