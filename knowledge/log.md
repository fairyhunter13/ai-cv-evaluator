---
type: Log
title: ai-cv-evaluator knowledge history
---

# Bundle history

## 2026-08-17

- **Update**: `defects/make-test-e2e-runs-nothing.md` extended again — `tools` installed `okf`
  `@latest`, so the gate ran but its verdict was not reproducible. Pinned to `v0.1.0`.
- `defects/make-test-e2e-runs-nothing.md` extended: `lint-knowledge` skipped whenever `okf` was
  absent, and `make tools` never installed it, so the bundle gate had never run on hosted CI.
- **Creation**: nine concepts, each harvested from a commit body or from a live code path that
  documents what it does but not why it matters. Sixteen candidates were surveyed; seven were left
  alone because they already carry their reason at the site that needs it — `otelpgx`'s
  provider-capture note in `conn.go`, the `HTTPRequestsByID` cardinality warning, and the banned
  free models in `freemodels/service.go` all explain themselves in place.
- `computations/candidate-evaluation.md` is `status: draft` because it describes a receipt the
  service does not write. It carries no `executor.receipt` and no `attester.resource`; both are
  what a fix would add. The concept exists to say, in one place, that no score in `results` can be
  attested — the four score-provenance concepts hang off it.
