# ADR-0003: AI Provider Choice

**Date:** 2025-09-28  
**Status:** Accepted  

## Context

The system requires LLM services for CV evaluation and project scoring. Key requirements:
- JSON-structured outputs for evaluation results
- High-quality reasoning for CV-job matching
- Reliable API availability and performance
- Cost-effective pricing for development and production
- Support for prompt engineering and parameter tuning
- Fallback options for resilience

## Decision

We will use **OpenRouter** as our primary AI provider for chat/completions (default model: `openrouter/auto` unless overridden), with **OpenAI** as the embeddings provider. End-to-end tests use live providers only; no mock/stub modes.

## Consequences

### Positive
- **Model Diversity**: Access to multiple models through single API
- **Cost Optimization**: Choose cost-effective models per use case
- **Fallback Options**: Built-in redundancy across providers
- **Consistent API**: Single integration point for multiple models
- **Rate Limit Management**: Better distributed rate limiting
- **Development Friendly**: Easy switching between models for testing

### Negative
- **External Dependency**: Requires internet connectivity and third-party service
- **API Costs**: Pay-per-use model increases operational costs
- **Rate Limits**: Subject to provider rate limiting
- **Latency**: Network calls add latency to evaluation pipeline

### Risks
- Service outages affect evaluation pipeline
- Rate limit exhaustion blocks new evaluations
- Cost overruns if usage spikes unexpectedly
- Model deprecation may require prompt rewriting

## Alternatives Considered

### Option A: OpenAI Direct
- **Pros**: Best model quality, reliable service, extensive documentation
- **Cons**: Higher costs, single provider risk, stricter rate limits
- **Rejected**: Lack of fallback options and higher cost

### Option B: Anthropic Claude
- **Pros**: Excellent reasoning, long context windows, safety focus
- **Cons**: Limited availability, higher costs, newer API ecosystem
- **Rejected**: Less mature Go SDK ecosystem

### Option C: Local Models (Ollama/LLaMA)
- **Pros**: No external costs, full control, privacy benefits
- **Cons**: Requires significant hardware, lower quality, complex deployment
- **Rejected**: Hardware requirements exceed VPS capabilities

### Option D: Azure OpenAI
- **Pros**: Enterprise features, SLA guarantees, regional deployment
- **Cons**: Complex setup, higher costs, Microsoft ecosystem lock-in
- **Rejected**: Over-engineered for project requirements

## Implementation Strategy

1. **Primary (Chat)**: OpenRouter with default `openrouter/auto`; allow overriding `CHAT_MODEL` when needed.
2. **Embeddings**: OpenAI `text-embedding-3-small` by default (configurable via `EMBEDDINGS_MODEL`).
3. **Testing**: E2E tests run against live providers; unit tests may mock interfaces locally.
4. **Monitoring**: Track costs, latency, and error rates per provider.
5. **Circuit Breaker**: Consider provider fallback strategies as future improvement.
