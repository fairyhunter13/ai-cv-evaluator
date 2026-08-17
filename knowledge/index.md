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

Four of the nine concern one subject: **nothing stored with a score says how it was produced.**
Start at [Candidate evaluation](computations/candidate-evaluation.md).

# Attested Computation

* [Candidate evaluation](computations/candidate-evaluation.md) - The scoring computation, its parameters, and the executor receipt it does not yet write - which is why no score in the results table can be attested.

# Constraint

* [A failed step changes the scorer, not just the latency](constraints/a-failed-step-changes-the-scorer.md) - Any of the four evaluation steps failing degrades to a single-prompt fast path with a different prompt, and nothing stored says which one ran.
* [Container limits are sized to a 1.9 GB server](constraints/container-limits-fit-a-1-9gb-server.md) - Production runs on 1.9 GB RAM; limits summed to ~8.7 GB and caused heavy swapping, and were rewritten to ~2.4 GB against measured actuals.

# Decision

* [Observability was deprecated in production, and the docs still advertise it](decisions/observability-was-deprecated-in-production.md) - In June 2026 the observability stack moved behind a compose profile and trace export was removed, but README and docs/observability.md still publish live Grafana, Prometheus and Jaeger URLs.
* [The lint gate had never run](decisions/the-lint-gate-had-never-run.md) - A v2 version key over v1 directives, plus a pinned binary that was installed and then not invoked, meant golangci-lint never executed on this module until 2026-08-15.

# Defect

* [make test-e2e runs zero tests and exits 0](defects/make-test-e2e-runs-nothing.md) - Every E2E target filters on -run TestE2E_Core_RateLimitFriendly$, a name no test has, so the README's E2E command passes vacuously.
* [Scores are fabricated from unrelated fields when the model omits them](defects/scores-are-fabricated-when-the-model-omits-them.md) - calculateCVMatchRateFromAnalysis and calculateProjectScoreFromAnalysis derive a score from skill counts and complexity words, and it is returned as if the model produced it.
* [The disk-full outage, and the five safeguards it bought](defects/the-disk-full-outage.md) - Image cleanup used double-quoted awk "{print $2}", so the shell expanded $2 to empty and nothing was ever pruned; 216 images filled the 40 GB disk.

# Policy

* [The project scoring rubric](policies/project-scoring-rubric.md) - Five weighted parameters scored 1-5 — correctness 30, code quality 25, resilience 20, documentation 15, creativity 10 — stated only inside a Go prompt literal.
