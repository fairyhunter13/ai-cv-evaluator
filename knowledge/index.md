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

Four of the ten concern one subject: **nothing stored with a score says how it was produced.**
Start at [Candidate evaluation](computations/candidate-evaluation.md).

# Subdirectories

* [computations](computations/index.md) - The scoring computation, its parameters, and the executor receipt it does not yet write - which is why no score in the results table can be attested.
* [constraints](constraints/index.md) - What the scorer's environment fixes: container limits sized to a 1.9 GB server, and a failed step that changes which scorer runs.
* [decisions](decisions/index.md) - What this repo gates and what it still advertises: a lint gate that had never run, a bundle gate that asserts its own installation, and observability the docs still promise.
* [defects](defects/index.md) - Three failures that each reported success: a green E2E target running zero tests, scores fabricated from unrelated fields, and the disk-full outage.
* [policies](policies/index.md) - Five weighted parameters scored 1-5 — correctness 30, code quality 25, resilience 20, documentation 15, creativity 10 — stated only inside a Go prompt literal.
* [references](references/index.md) - Deterministic verification code for computation receipts.
