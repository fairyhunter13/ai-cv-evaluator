---
type: Defect
resource: Makefile
title: make test-e2e runs zero tests and exits 0
description: Every E2E target filters on -run TestE2E_Core_RateLimitFriendly$, a name no test has, so the README's E2E command passes vacuously.
tags: [e2e, ci, testing]
status: stable
generated: {by: claude-opus-5, at: 2026-08-17}
---

# What is wrong

`test-e2e` delegates to `test-e2e-core`, which runs
`go test -tags=e2e -run "TestE2E_Core_RateLimitFriendly$" ./test/e2e/...`. The only E2E test in the
tree is `TestE2E_Core_SingleJob` (`test/e2e/core_e2e_test.go:38`). `go test` exits 0 when a `-run`
pattern matches nothing, so the target is green and has always been green. The same filter appears
in `test-e2e-ci` and `run-e2e-core`.

`test-e2e-single` is the one target that runs a real test — it filters on
`TestE2E_Core_SingleJob`.

# Why nothing caught it

`.github/workflows/ci.yml` never runs the Go E2E suite at all; its only end-to-end job is a
Playwright SSO/Grafana spec. So the vacuous pass is not even observed by CI — it is observed by a
human running the command the README documents.

# What a fix has to do

Renaming the filter to the real test is not sufficient on its own. A target whose test selector can
silently match nothing will do this again; the durable form asserts a non-zero test count, or drops
`-run` and lets the build tag do the selecting.

# The same shape in the knowledge gate

`lint-knowledge` had it too, found 2026-08-17: it ran `okf check knowledge` only when `okf` was on
PATH and printed "skipping" otherwise, and `make tools` never installed `okf`. Every hosted CI run
logged the skip and passed, so the bundle was gated only on developer machines that happened to have
the binary. Fixed by installing `okf` in `tools` and exiting 1 when it is still absent. A gate whose
tool is optional is not a gate, which is the same lesson as a selector that can match nothing.

# And the version it installed was floating

Making `okf` mandatory left one thing unfixed: `tools` installed it `@latest`, so the verdict
`lint-knowledge` reports could change between two runs of the same commit, decided by a push to a
different repo. Pinned to `v0.1.0` 2026-08-17. The three failures on this page are one failure —
a green that does not mean what a reader takes it to mean — reached by a missing test, a missing
tool, and an unnamed version. The fourth instance is
[the lint gate that had never run](../decisions/the-lint-gate-had-never-run.md).
