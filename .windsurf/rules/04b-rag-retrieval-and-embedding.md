---
trigger: always_on
---

Define retrieval strategy, embeddings, chunking, and Qdrant collections for deterministic context injection.

# Provider Default (OpenAI Embeddings)
- Default embeddings provider: `openai`.
- Env keys: see `04-ai-llm-and-rag-pipeline.md` → Provider Defaults (Chat vs Embeddings).
- Endpoint (OpenAI-compatible): `POST ${OPENAI_BASE_URL}/embeddings`
- Headers: `Authorization: Bearer ${OPENAI_API_KEY}`, `Content-Type: application/json`.

- Provider preference order:
  1) OpenAI (when `OPENAI_API_KEY` set)
  2) OpenRouter (when `OPENROUTER_API_KEY` set)
  3) Deterministic mock mode (CI/offline)

# Frugality & Cost Controls (Embeddings)
- De-duplicate texts before embedding (hash-based) and cache vectors by content hash.
- Truncate long texts to model token limits; avoid re-embedding unchanged content.
- Batch requests conservatively (e.g., 8–16 items) and cap concurrency.
- Backoff on rate limits; fail fast with clear logs to prevent waste.
- Use minimal top-k retrieval that meets accuracy goals; reuse previous results when possible.

# Vector DB & Collections (Qdrant)
- Collections:
  - `job_description` (payload: title, section, text)
  - `scoring_rubric` (payload: parameter, weight, description)
- Idempotent creation at startup; consistent vector sizes and distance metric.
- Payload indexes for frequent filters; optional on-disk payloads for large corpora.

# Embeddings
- Default model: `text-embedding-3-small` (configurable via env).
- Batch requests with concurrency limits; expose metrics for latency and errors.
- Offline mode: deterministic embedding vectors via stable hashing (for CI).

# Chunking & Retrieval
- Normalize extracted text: strip control chars, preserve headings.
- Chunk size: 512–1024 tokens; 10–20% overlap.
- Top-k similarity search tuned per task (e.g., k=4–8); optional re-ranking.
- Per-corpus metadata (source, section, weight) to boost precision.

# Retrievers
- Separate retrievers for CV scoring and Project scoring; tune independently.
- Provide simple interfaces to swap implementations in tests.

# Definition of Done (RAG)
- Retrieval returns relevant chunks for both CV and project tasks in integration tests.
- Deterministic results in mock mode across runs.
