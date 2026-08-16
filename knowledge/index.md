---
okf_version: "0.2"
---

Durable knowledge for this service, as an OKF v0.2 bundle. Read what touches your task before
starting; write it back in the same commit as the code.

This repo has no ADR tree and no `CLAUDE.md`, so rationale had been accumulating in commit bodies.
What lands here is what has no other home. `README.md` and the six files under `docs/` keep what
they already cover — architecture and blue/green, day-2 operations, data retention, SSO rate
limiting, frontend development — and `internal/adapter/ai/real/README.md` keeps the AI backoff
write-up. Nothing here restates them.

Four of the eight concern one subject: **nothing stored with a score says how it was produced.**
Start at [Candidate evaluation](computations/candidate-evaluation.md).

# Attested Computation

- [Candidate evaluation](computations/candidate-evaluation.md)

# Constraint

- [A failed step changes the scorer, not just the latency](constraints/a-failed-step-changes-the-scorer.md)
- [Container limits are sized to a 1.9 GB server](constraints/container-limits-fit-a-1-9gb-server.md)

# Decision

- [Observability was deprecated in production, and the docs still advertise it](decisions/observability-was-deprecated-in-production.md)
- [The lint gate had never run](decisions/the-lint-gate-had-never-run.md)

# Defect

- [make test-e2e runs zero tests and exits 0](defects/make-test-e2e-runs-nothing.md)
- [Scores are fabricated from unrelated fields when the model omits them](defects/scores-are-fabricated-when-the-model-omits-them.md)
- [The disk-full outage, and the five safeguards it bought](defects/the-disk-full-outage.md)

# Policy

- [The project scoring rubric](policies/project-scoring-rubric.md)
