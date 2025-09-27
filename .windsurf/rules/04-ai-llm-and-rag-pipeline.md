---
trigger: always_on
---

Design and implement the AI pipeline with prompt design, chaining, RAG, and resilience.

# Note
- See also: `04a-llm-prompting-and-schema.md` and `04b-rag-retrieval-and-embedding.md` for detailed guidance.

# Provider Defaults (Chat vs Embeddings)
- Chat provider (default): `openrouter`.
- Embeddings provider (default): `openai`.

- Environment:
  - Chat (OpenRouter):
    - `AI_PROVIDER` (default `openrouter`)
    - `OPENROUTER_API_KEY` (required for live; if absent, use mock mode)
    - `OPENROUTER_BASE_URL` (default `https://openrouter.ai/api/v1`)
    - `CHAT_MODEL` (default `openai/gpt-4o-mini` via OpenRouter)
  - Embeddings (OpenAI):
    - `OPENAI_API_KEY` (preferred for embeddings; if absent, fallback to OpenRouter or mock)
    - `OPENAI_BASE_URL` (default `https://api.openai.com/v1`)
    - `EMBEDDINGS_MODEL` (default `text-embedding-3-small`)

- Endpoints (OpenAI-compatible):
  - Chat (OpenRouter): `POST ${OPENROUTER_BASE_URL}/chat/completions`
  - Embeddings (OpenAI): `POST ${OPENAI_BASE_URL}/embeddings`

- Headers:
  - Chat (OpenRouter): `Authorization: Bearer ${OPENROUTER_API_KEY}`, `Content-Type: application/json`. Optional: `HTTP-Referer`, `X-Title`.
  - Embeddings (OpenAI): `Authorization: Bearer ${OPENAI_API_KEY}`, `Content-Type: application/json`.


# Pipeline Stages
1) Extract structured info from CV (skills, experience, projects).
2) Compare against job description → produce match rate + CV feedback.
3) Evaluate project report against rubric → project score + feedback.
4) Aggregate into final structured result with overall summary.

# RAG (Retrieval Augmented Generation)
- Vector DB: Qdrant. Two collections:
  - `job_description` (payload: title, section, text)
  - `scoring_rubric` (payload: parameter, weight, description)
- Embeddings: default `text-embedding-3-small` (configurable).
- Build retrievers tuned for: CV scoring and project scoring.
- If `OPENAI_API_KEY` absent: deterministic mock embeddings + mock LLM fixtures.

# Prompting & Chaining
- Use system prompts enforcing JSON schema output; no freeform text.
- Two-pass approach:
  - Pass 1: raw scoring with retrieved context for each task.
  - Pass 2: consistency pass to normalize ranges and ensure required fields.
- Control randomness: temperature 0.2–0.4, top_p default.
- Validate outputs against JSON schema; retry invalid outputs with backoff.

# Chain-of-Thought (CoT) Handling
- Never request, collect, store, or return free-form step-by-step reasoning to clients.
- Prompts must explicitly instruct: "You may reason privately but return only JSON matching the schema. Do not include your reasoning or chain-of-thought in the output."
- If justification is needed, use the designated fields and lengths:
  - `cv_feedback`, `project_feedback`: concise, 1–3 sentences.
  - `overall_summary`: richer narrative, 3–5 sentences (per `project.md`).
  - No step-by-step reasoning.
- Validation must reject outputs that leak CoT (e.g., patterns like "Step 1", "First,", "I think", long numbered lists) outside allowed fields/lengths.
- Do not log raw prompts or completions in production. Log only token counts and status/latency metrics; optionally sample sanitized snippets in dev only.
- Tests should assert absence of CoT leakage in responses and that only the allowed JSON schema is returned.

# Resilience
- Timeouts on all external calls (10–15s typical).
- Retries with exponential backoff and jitter (`cenkalti/backoff`).
- Concurrency limits; token bucket rate limiting.
- Optional circuit breaker for persistent upstream failures.
- Structured error taxonomy (UpstreamTimeout, UpstreamRateLimit, SchemaInvalid, RetrieverEmpty, etc.).

# Scoring Aggregation (from `project.md`)
- CV Match: weights 40/25/20/15 → weighted average → convert to % (×20) → `cv_match_rate` in [0.0,1.0].
- Project Deliverable: weights 30/25/20/15/10 → weighted average (1–5) → normalize ×2 → `project_score` in [0.0,10.0].
- Final fields: `cv_match_rate`, `cv_feedback`, `project_score`, `project_feedback`, `overall_summary`.

# Asynchronous Job Handling
- `/evaluate` enqueues job with IDs of artifacts.
- Worker loads texts, runs retrievers + LLM chain, aggregates scores, persists result, marks complete.
- `/result/{id}` polls job status until completed.

# Prompt Templates & JSON Schemas
- Enforce strict JSON outputs; never accept freeform prose.
- CV evaluation output schema (example):
  ```json
  {
    "cv": {
      "technical_skills": {"score": 1, "feedback": ""},
      "experience_level": {"score": 1, "feedback": ""},
      "achievements": {"score": 1, "feedback": ""},
      "cultural_fit": {"score": 1, "feedback": ""}
    },
    "cv_match_rate": 0.0,
    "cv_feedback": ""
  }
  ```
- Project evaluation output schema (example):
  ```json
  {
    "project": {
      "correctness": {"score": 1, "feedback": ""},
      "code_quality": {"score": 1, "feedback": ""},
      "resilience": {"score": 1, "feedback": ""},
      "documentation": {"score": 1, "feedback": ""},
      "creativity": {"score": 1, "feedback": ""}
    },
    "project_score": 0.0,
    "project_feedback": "",
    "overall_summary": ""
  }
  ```
- Prompt snippets:
  - System prompt mandates schema, strict numbers 1–5, and concise feedback.
  - User prompt supplies retrieved context chunks (job description/rubric) and extracted CV/project text.
  - Second pass normalizes values and validates required fields.

# Chunking & Embedding Strategy
- Text extraction normalization: strip control chars, normalize whitespace, preserve headings.
- Chunk sizes 512–1024 tokens with 10–20% overlap; benchmark retrieval quality.
- Use per-corpus metadata (source, section, weight) for better filtering.
- Similarity search top-k tuned (e.g., k=4–8); re-rank if needed.

# Provider Abstraction & Keys
- Create `AIClient` interface with methods: `Embed(ctx, texts)`, `ChatJSON(ctx, prompt, schema)`.
- Implement providers: OpenAI and Mock; design the interface to be pluggable so additional providers can be swapped in without changing usecases.
- Configure via env: `OPENAI_API_KEY`, `EMBEDDINGS_MODEL`, `CHAT_MODEL`.
- Pluggable via DI to swap in mocks for tests and offline mode.

# Guardrails & Validation
- JSON schema validation for all LLM responses.
- Retry with exponential backoff on invalid JSON or upstream 5xx/429.
- Apply max tokens, temperature 0.2–0.4, and sensible system prompts.
- Timeouts per call; circuit breaker to avoid cascading failures.

# Cost & Rate Controls
- Limit concurrency for LLM calls; rate limit per provider.
- Aggregate per-request token usage metrics.
- Fallback to cached intermediate results when possible to reduce cost.

# Offline / Mock Mode
- If `OPENAI_API_KEY` is missing, use deterministic mocks:
  - Embeddings: stable hash → vector generator.
  - Chat: load canned responses from `test/testdata/ai_fixtures/`.
- Ensure integration and E2E tests operate in mock mode by default in CI.

# Evaluation & Quality
- Add golden tests for prompt I/O pairs; verify schema compliance and stability.
- Track distribution of scores over a sample set to detect drift.
- Log minimal prompt metadata (no PII) and token counts for analysis.

# Definition of Done (AI)
- Mock mode works offline.
- RAG returns relevant chunks; tests assert determinism.
- LLM outputs JSON passing schema validation; retry logic verified.
