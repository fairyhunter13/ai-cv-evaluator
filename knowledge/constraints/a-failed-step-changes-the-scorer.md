---
type: Constraint
resource: internal/adapter/queue/redpanda/integrated_evaluation_handler.go
title: A failed step changes the scorer, not just the latency
description: Any of the four evaluation steps failing degrades to a single-prompt fast path with a different prompt, and nothing stored says which one ran.
tags: [scoring, llm, fallback]
status: stable
generated: {by: claude-opus-5, at: 2026-08-17}
---

# The behaviour

`PerformIntegratedEvaluation` runs four steps — CV match, project deliverables, refinement,
validation. Each one's error arm returns `performFastPathEvaluation(...)`, a single-prompt
evaluation with its own prompt text. A timeout in step 2 is therefore not a retry of step 2: it is
a different scorer, reading different instructions, producing the score that gets stored.

RAG is separately best-effort by design — a missing embedding or an unreachable Qdrant is swallowed
and the step proceeds prompt-only. That degradation is also invisible in the result.

# Why it is written down

The fallback is defensible; the silence is the constraint. Two candidates submitted an hour apart
can be scored by two structurally different prompt chains, with different RAG context, and the
`results` row is byte-identical in shape. Any comparison across jobs — a ranking, a threshold, a
regression check on prompt changes — assumes a fixed scorer that the code does not provide.

# What has to hold for a fix

Whatever records the path must be written on the same transaction as the score, not logged.
`slog` lines carry the model id and attempt counts today, and logs are ephemeral: the trace
collector was removed from the deployment in June 2026. See
[Candidate evaluation](../computations/candidate-evaluation.md).
