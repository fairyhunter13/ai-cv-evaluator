# Provider Documentation

This project integrates multiple external AI providers for chat completions, embeddings, and rate limiting.

- **Groq** – primary chat/completions provider when `GROQ_API_KEY` / `GROQ_API_KEY_2` are configured.
- **OpenRouter** – fallback chat provider backed by a curated set of free models, used when Groq is unavailable or rate limited.
- **OpenAI** – embeddings provider used for RAG (vector DB seeding and search) when `OPENAI_API_KEY` is set.

The following documents mirror the upstream provider documentation that this service relies on:

- **Groq API reference**: [`api-reference.md`](./api-reference.md)
- **Groq rate limits**: [`rate-limits.md`](./rate-limits.md)
- **OpenRouter models (GET /models)**: [`get-models.md`](./get-models.md)

These files are vendor docs kept alongside the codebase so you can quickly look up request/response shapes, available models, and rate-limit semantics while working on the evaluator.
