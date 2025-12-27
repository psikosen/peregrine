# Peregrine

An intelligent LLM routing system implementing **Clopus-NG v2** - an autonomous monitoring and reasoning system built on Model-First Reasoning (MFR) principles.

## Overview

Peregrine provides priority-based LLM routing with automatic fallback, capability-aware provider selection, and deterministic rule-based fallbacks for common patterns. It implements a strict two-phase agent model separating problem modeling from solution generation.

### Key Features

- **Priority-based routing** - External LLM → Local Ollama → Deterministic rules
- **Capability-aware selection** - Routes requests to providers that support required capabilities
- **Prompt caching** - Static system prompts cached at ngrok gateway layer
- **Health monitoring** - Automatic provider health checks with failure tracking
- **Metrics instrumentation** - Prometheus metrics for routing decisions and latencies

## Architecture

### Two-Phase Agent Model

**Phase 1: Modeler Agent**
- Input: Raw logs, events, metrics, resource specs
- Output: Schema-validated JSON problem model
- Restrictions: No recommendations, no explanations, no actions

**Phase 2: Solver Agent**
- Input: Structured Problem Model (JSON only)
- Output: Action specification with target, patch, justification, risk level

### LLM Provider Priority

| Priority | Provider | Use Case |
|----------|----------|----------|
| 1 | External via ngrok | Complex reasoning, high accuracy |
| 2 | Local Ollama | Fallback, privacy-sensitive |
| 3 | Rule Engine | Deterministic patterns (OOMKilled, ImagePullBackOff, etc.) |

## Getting Started

### Prerequisites

- Go 1.21+
- protoc (for gRPC code generation)
- Ollama (optional, for local LLM fallback)

### Installation

```bash
# Clone the repository
git clone https://github.com/peregrine/router.git
cd router

# Install dependencies
make deps

# Generate protobuf code
make proto

# Build
make build
```

### Running

```bash
# Run with default settings
make run

# Run with local Ollama
make run-local
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `OLLAMA_ENDPOINT` | Ollama server URL | `http://localhost:11434` |
| `OLLAMA_MODEL` | Ollama model name | `gemma3:270m` |
| `NGROK_ENDPOINT` | ngrok AI Gateway URL | `https://ai-gateway.ngrok.io` |
| `NGROK_API_KEY` | ngrok API key | (required for ngrok) |
| `NGROK_MODEL` | External LLM model | `claude-3-sonnet` |

## API

Peregrine exposes a gRPC API defined in `api/proto/router.proto`:

```protobuf
service LLMRouter {
  rpc Complete(CompleteRequest) returns (CompleteResponse);
  rpc GetHealth(HealthRequest) returns (HealthResponse);
  rpc ListProviders(ListProvidersRequest) returns (ListProvidersResponse);
}
```

### Complete Request

```protobuf
message CompleteRequest {
  string agent_role = 1;           // "modeler" or "solver"
  string system_prompt = 2;        // Static (cached)
  string user_prompt = 3;          // Dynamic (suffix tokens)
  double temperature = 4;
  int32 max_tokens = 5;
  map<string, string> metadata = 6;
  repeated string required_capabilities = 7;
}
```

## Project Structure

```
cmd/router/          # gRPC server entry point
internal/
  router/            # Core routing logic, health checks
  provider/          # LLM provider implementations (ngrok, ollama, rules)
  metrics/           # Prometheus instrumentation
  model/             # Request/response types, capabilities
api/proto/           # gRPC service definition
deployments/k8s/     # Kubernetes manifests
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Run with hot reload (requires air)
make dev

# Lint code
make lint

# Format code
make fmt
```

## References

- [Model-First Reasoning (arXiv:2512.14474)](https://arxiv.org/pdf/2512.14474)
- [ngrok Prompt Caching](https://ngrok.com/blog/prompt-caching)
- [Clopus Watcher](https://denislavgavrilov.com/p/clopus-watcher-an-autonomous-monitoring)

## License

MIT
