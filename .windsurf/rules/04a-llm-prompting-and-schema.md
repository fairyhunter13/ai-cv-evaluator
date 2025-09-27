---
trigger: always_on
---

Guide LLM prompting and JSON-schema-constrained outputs to ensure deterministic, valid results.

# Provider Default (OpenRouter)
- Default provider: `openrouter`.
- Env keys: see `04-ai-llm-and-rag-pipeline.md` → Provider Defaults (Chat vs Embeddings).
- Endpoint (OpenAI-compatible): `POST ${OPENROUTER_BASE_URL}/chat/completions`
- Headers: `Authorization: Bearer ${OPENROUTER_API_KEY}`, `Content-Type: application/json`.

- Provider preference order (Chat):
  1) OpenRouter (when `OPENROUTER_API_KEY` set)
  2) Deterministic mock mode (CI/offline)


# Prompting Principles
- System prompt enforces: JSON only, no prose, numeric scales 1–5 for sub-scores, concise feedback.
- Use a 2-pass strategy:
  - Pass 1: raw scoring with retrieved context.
  - Pass 2: normalization and validation (fix ranges, fill required fields).
- Control randomness:
  - temperature: 0.2–0.4 (default 0.2)
  - top_p: default
  - max_tokens bounded for cost control

# Chain-of-Thought-Safe Prompting
- Do not request or output step-by-step reasoning; only return the JSON schema.
- Add this line to the system prompt: "You may reason privately but must only output valid JSON that matches the schema. Do not include chain-of-thought or internal reasoning in the output."
- Keep justifications concise using existing fields (`cv_feedback`, `project_feedback`, `overall_summary`) and limit them to 1–3 sentences.
- On validation, reject outputs that leak CoT (e.g., patterns like "Step 1", "First,", "I think", lengthy numbered lists) and retry with a clarifying instruction.

# JSON Schemas
- CV evaluation (subset):
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
- Project evaluation (subset):
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

# Validation & Retries
- Validate LLM JSON against schema; retry on invalid JSON or missing fields with exponential backoff + jitter.
- Use a small number of retries (e.g., 3) with increasing delays; do not retry on deterministic validation failures.

# Output Normalization
- Clamp sub-scores to [1,5]; coerce floats to ints if needed.
- Recompute aggregates:
  - CV match → weighted average (40/25/20/15) × 20 → `cv_match_rate` in [0,1].
  - Project → weighted average (30/25/20/15/10) × 2 → `project_score` in [0,10].
- Generate concise `overall_summary` (3–5 sentences).

# Mock Mode
- Without API keys: use deterministic mock responses based on stable input hashing.
- Store fixtures under `test/testdata/ai_fixtures/` for integration/E2E tests.

# Frugality & Cost Controls (Chat)
- Minimize token usage: keep prompts compact, remove irrelevant context, and cap `max_tokens`.
- Prefer lower-cost models by default; upgrade temporarily only when justified by tests.
- Cache intermediate LLM outputs (e.g., extracted CV structure) and reuse across stages.
- Avoid repeated calls on unchanged inputs; add stable hashing + memoization.
- Limit concurrency and disable streaming unless strictly required.
- Implement early-exit paths when sufficient confidence is reached.

# Definition of Done (LLM Prompting)
- LLM outputs are valid JSON per schema over a representative test set.
- Golden tests capture stability of prompt templates and outputs.
