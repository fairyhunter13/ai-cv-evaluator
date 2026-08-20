---
type: Attested Computation
resource: internal/adapter/queue/redpanda/integrated_evaluation_handler.go
title: Candidate evaluation
description: The scoring computation, its parameters, and the executor receipt it does not yet write - which is why no score in the results table can be attested.
tags: [scoring, attestation, provenance]
status: draft
runtime: go
computation: PerformIntegratedEvaluation
executor:
  resource: cmd/worker
generated: {by: claude/opus-5, at: 2026-08-17T00:00:00Z}
---

# Computation

`POST /v1/evaluate` enqueues an `EvaluateTaskPayload` on the Redpanda topic `evaluate`. The worker's
`HandleEvaluate` calls `PerformIntegratedEvaluation`, which makes three sequential
`ChatJSONWithRetry` calls — CV match, project deliverables, refinement — then validates and stores.
The rubric the second call applies is [The project scoring rubric](../policies/project-scoring-rubric.md).

# Parameters

These are the values that determine a score. All are fixed in code except the model, which rotates.

| Parameter | Value |
|---|---|
| `temperature` | `0.2` for chat, `0.1` for chain-of-thought cleaning |
| `seed`, `top_p` | not set |
| `max_tokens` | chosen by prompt length: 512, then 1024 above 4k, 1536 above 8k, 2048 above 16k |
| model / provider | rotated across two Groq and two OpenRouter accounts with per-model fallback |
| retries | 3 handler attempts (linear 2s/4s), plus client-side exponential backoff under `AI_BACKOFF_*` |
| job timeout | 5 min; step 2 carries its own 5-min sub-timeout |
| RAG | Qdrant `job_description` top-3 and `scoring_rubric` top-2, best-effort |
| output ranges | `cv_match_rate ∈ [0,1]`, `project_score ∈ [1,10]`, silently clamped |

# Executor receipt

**There is none, and that is the point of this concept.** The `results` table stores `job_id`,
`cv_match_rate`, `cv_feedback`, `project_score`, `project_feedback`, `overall_summary`,
`created_at`. Nothing else is persisted with a score:

- not the model id, provider or account that produced it;
- not the prompt version — `test/testdata/golden/prompt_system.txt` reads like a pinned prompt but
  is referenced by zero Go code, and the live prompts are inline literals;
- not which path ran — the three-call chain, the
  [fast path](../constraints/a-failed-step-changes-the-scorer.md), or the
  [fabricated derivation](../defects/scores-are-fabricated-when-the-model-omits-them.md);
- not the temperature, max_tokens or attempt count actually used;
- not a git sha or build version — `ldflags` are only `-s -w` and no `main.version` exists.

`slog` carries most of this and logs are ephemeral; trace export was removed from the deployment in
June 2026. The Prometheus histograms `evaluation_cv_match_rate` and `evaluation_project_score` are
aggregate only. `artifacts/` is `rm -rf`'d at the start of every E2E target.

# Attester

No attester exists. What a deterministic, LLM-free one could check **today**, from a stored result
alone:

- `0 ≤ cv_match_rate ≤ 1` and `1 ≤ project_score ≤ 10` before clamping, not after;
- the three text fields are non-empty and are not the `"No feedback provided"` /
  `"No summary provided"` defaults;
- the response body is a bare JSON object with exactly the five expected keys;
- `jobs.status = completed` implies a `results` row, which the write order already guarantees;
- a failure code is drawn from the closed set in `classifyFailureCode`.

What it **cannot** check until the receipt above exists: reproducibility, which model produced the
score, and whether the score was assessed or derived. Those are the ones worth having — an
evaluator that cannot attest its own scores is asking to be trusted on the same terms it refuses
to extend to the candidates it grades.
