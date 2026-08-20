---
type: Attested Computation
resource: internal/adapter/queue/redpanda/integrated_evaluation_handler.go
title: Candidate evaluation
description: The scoring computation, its parameters, and the executor receipt it does not yet write - which is why no score in the results table can be attested.
tags: [scoring, attestation, provenance]
status: draft
runtime: go
computation: internal/adapter/queue/redpanda/integrated_evaluation_handler.go
parameters:
  - { name: job_id, type: string, required: true }
  - { name: cv_text, type: string, required: true }
  - { name: project_text, type: string, required: true }
executor:
  resource: cmd/worker
  receipt: [job_id, git_sha, model_id, provider, prompt_version, path_taken, temperature, max_tokens, attempts, raw_scores]
attester:
  resource: ../references/attesters/evaluation_receipt.py
generated: {by: claude/opus-5, at: 2026-08-21T00:00:00Z}
---

# Computation

`POST /v1/evaluate` enqueues an `EvaluateTaskPayload` on the Redpanda topic `evaluate`. The worker's
`HandleEvaluate` calls `PerformIntegratedEvaluation`, which makes three sequential
`ChatJSONWithRetry` calls — CV match, project deliverables, refinement — then validates and stores.
The rubric the second call applies is [The project scoring rubric](../policies/project-scoring-rubric.md).

# Parameters

These are the values that determine a score. All are fixed in code except the model, which rotates — which is why none of them is a `parameters` entry above: those are the typed holes a caller fills, and every value here is one the caller cannot reach.

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

`executor.receipt` above declares what a run must return. **Nothing returns it, and that is the
point of this concept** — the declaration is what makes the gap a failing check instead of a
paragraph. The `results` table stores `job_id`,
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

[`references/attesters/evaluation_receipt.py`](../references/attesters/evaluation_receipt.py) is
written against the receipt above, so today it fails every evaluation on the first check: the
receipt is empty. That is the honest verdict. What it checks once the fields exist:

- the receipt is for the job whose row is being attested;
- the model, prompt version and temperature are the sanctioned ones, and the attempt count is
  within the retry policy;
- `0 ≤ cv_match_rate ≤ 1` and `1 ≤ project_score ≤ 10` before clamping, not after, and the stored
  score is the pre-clamp one — a clamp that moved a score is a rejection, not a repair;
- the path taken is one of the three known ones, and a
  [derived](../defects/scores-are-fabricated-when-the-model-omits-them.md) score fails: it was
  never assessed;
- the three text fields are non-empty and are not the `"No feedback provided"` /
  `"No summary provided"` defaults.

Every one of those needs a field the receipt does not carry yet. An evaluator that cannot attest
its own scores is asking to be trusted on the same terms it refuses to extend to the candidates it
grades, and the check is written now so that stays a red test rather than a paragraph.
