# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Peregrine** implements **Clopus-NG v2** - an autonomous monitoring and reasoning system built on Model-First Reasoning (MFR) principles from arXiv:2512.14474.

### Core Design Constraints

1. **Strict Model-First Reasoning** - No summarization, no context accumulation, no reasoning before modeling
2. **Prompt Caching** - Static system prompts cached at ngrok gateway layer; only suffix tokens are paid
3. **LLM Reliability** - Primary: external LLM via ngrok, Secondary: Ollama local (`gemma3:270m`), Tertiary: deterministic rules
4. **Isolated, Stateless Agents** - Zero cross-task memory, zero history leakage

## Architecture

### Two-Phase Agent Model

**Phase 1: Modeler Agent**
- Input: Raw logs, events, metrics, resource specs
- Output: Schema-validated JSON problem model (entities, state_variables, constraints, confidence)
- Restrictions: NO recommendations, NO explanations, NO actions
- If output deviates from schema → task rejected and retried

**Phase 2: Solver Agent**
- Input: Structured Problem Model (JSON only) - never sees raw logs
- Output: Action specification with target, patch, justification, risk level

### LLM Routing Priority

| Priority | Provider | Model |
|----------|----------|-------|
| 1 | External via ngrok | Claude/Gemini |
| 2 | Local Ollama | gemma3:270m |
| 3 | Rule Engine | Deterministic fallback |

### Capability Downgrade Rules

- Modeler/Solver: External preferred, Ollama allowed
- Complex Planning / Multi-step remediation: External required, Ollama rejected
- If Ollama returns confidence below threshold → escalate or defer

### Deterministic Fallback Patterns

Common cases (OOMKilled, ImagePullBackOff, CrashLoopBackOff) can use non-LLM rules to reduce LLM load.

## Key References

- Model-First Reasoning: https://arxiv.org/pdf/2512.14474
- ngrok Prompt Caching: https://ngrok.com/blog/prompt-caching
- Clopus Watcher: https://denislavgavrilov.com/p/clopus-watcher-an-autonomous-monitoring

## Tech Stack

- Go 1.21+ with gRPC
- Kubernetes for deployment
- ngrok AI Gateway for prompt caching
- Ollama for local LLM inference (gemma3:270m)
- Prometheus for metrics
- NATS for messaging (planned)

## Build Commands

```bash
# Install dependencies
make deps

# Generate protobuf code (requires protoc)
make proto

# Build the router binary
make build

# Run tests
make test

# Run locally with Ollama
make run-local

# Build Docker image
make docker-build
```

## Development

```bash
# Run with hot reload (requires air)
make dev

# Lint code
make lint

# Format code
make fmt
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

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| OLLAMA_ENDPOINT | Ollama server URL | http://localhost:11434 |
| OLLAMA_MODEL | Ollama model name | gemma3:270m |
| NGROK_ENDPOINT | ngrok AI Gateway URL | https://ai-gateway.ngrok.io |
| NGROK_API_KEY | ngrok API key | (required for ngrok) |
| NGROK_MODEL | External LLM model | claude-3-sonnet |
