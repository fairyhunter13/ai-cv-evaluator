---
trigger: always_on
---

Define clear API contracts, input validation, and consistent error responses.

# Endpoints (Required)
- POST `/upload`
  - Content-Type: `multipart/form-data`.
  - Fields: `cv` (file), `project` (file).
  - Accept: .txt, .pdf, .docx
  - Behavior:
    - Detect MIME by content.
    - Reject > 10MB files (configurable).
    - Extract text:
      - txt: raw read
      - pdf: Apache Tika server (HTTP) for text extraction
      - docx: Apache Tika server (HTTP) for text extraction
    - Persist text + metadata; return `{ "cv_id": "...", "project_id": "..." }`.

- POST `/evaluate`
  - Body:
    {
      "cv_id": "string",
      "project_id": "string",
      "job_description": "string",
      "study_case_brief": "string"
    }
  - Validate presence and size limits for text fields.
  - Enqueue job; return `{ "id": "<job-id>", "status": "queued" }`.

- GET `/result/{id}`
  - Queued response: `{ "id": "456", "status": "queued" }`
  - Processing response: `{ "id": "456", "status": "processing" }`
  - Completed response:
    {
      "id": "456",
      "status": "completed",
      "result": {
        "cv_match_rate": 0.82,
        "cv_feedback": "...",
        "project_score": 7.5,
        "project_feedback": "...",
        "overall_summary": "..."
      }
    }
  - Failed response (optional, recommended):
    {
      "id": "456",
      "status": "failed",
      "error": { "code": "SCHEMA_INVALID", "message": "validation failed" }
    }

# Error Model
- Error response:
  { "error": { "code": "string", "message": "string", "details": {} } }
- HTTP status codes: 400 validation, 413 too large, 415 unsupported media type, 429 rate limit, 500 internal, 503 upstream AI failure.
- Include `X-Request-Id` in responses.

# OpenAPI
- Maintain `api/openapi.yaml` describing endpoints, schemas, and the error model.
- Optional: serve `/openapi.yaml` or `/docs`.

# Security & Validation
- Sanitize extracted text (strip control chars).
- Enforce per-field length limits.
- CORS policy: allowlist origins for GET; tighten POST in prod.

# Additional API Standards
- Content negotiation:
  - All responses use `application/json; charset=utf-8`.
  - Return 406 if an unsupported `Accept` header is sent.
- Idempotency:
  - Provide optional `Idempotency-Key` header for POST `/evaluate` to dedupe retried client requests.
  - Store key→job mapping with TTL; return the original response for duplicates.
- Rate limiting:
  - Apply token bucket or fixed window for POST endpoints; include `X-RateLimit-Remaining`/`X-RateLimit-Reset` headers when appropriate.
- Pagination & filtering (future endpoints):
  - Use `page` and `page_size` with bounds; default page_size ≤ 50.
  - Return pagination metadata: `{ "page": 1, "page_size": 20, "total": 123 }`.
- Request validation:
  - Validate required fields, lengths, and allowed enum values (e.g., file types) with a central validator.
  - Map validation errors to 400 with field-level details.
- Headers:
  - Correlation: require/propagate `X-Request-Id`.
  - Caching: allow `GET /result/{id}` to use `ETag`/`If-None-Match` for completed results to reduce bandwidth.

# Response Content Policy (No Chain-of-Thought)
- API responses must never include chain-of-thought or step-by-step reasoning.
- Only return the structured fields defined in the schemas. `cv_feedback` and `project_feedback` should be concise (1–3 sentences); `overall_summary` must be 3–5 sentences. No numbered steps.
- Server-side validation rejects and retries on responses that leak CoT; persistent failures map to the existing error taxonomy (e.g., `SCHEMA_INVALID` / upstream errors) without exposing the raw model output.
- See also: `04-ai-llm-and-rag-pipeline.md` → Chain-of-Thought (CoT) Handling.
- File uploads:
  - Enforce content sniffing and extension checks; reject archives/executables.
  - Limit total multipart size and per-file size; stream to memory or temp files based on size.
  - Normalize line endings and strip non-printable control characters from extracted text.
- Consistent error taxonomy:
  - `INVALID_ARGUMENT`, `NOT_FOUND`, `CONFLICT`, `RATE_LIMITED`, `UPSTREAM_TIMEOUT`, `UPSTREAM_RATE_LIMIT`, `SCHEMA_INVALID`, `INTERNAL`.
  - Map to appropriate HTTP codes and log with structured fields.
- Examples and docs:
  - Provide concise examples in `README.md` for each endpoint including curl snippets.
  - Ensure examples exactly match OpenAPI schemas and actual responses.

# Contract-First and OpenAPI Practices
- Maintain `api/openapi.yaml` as the source of truth.
- Validate handlers in e2e tests using OpenAPI (e.g., `kin-openapi` or generated clients via `oapi-codegen`).
- Keep response shapes (including error envelope) in sync with the spec.
- Version the API (`/v1`) and document breaking-change policy.

# Definition of Done (API)
- Implement exact shapes per `project.md`.
- Validation and error codes implemented.
- OpenAPI kept in sync with code.
