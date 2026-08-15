---
description: Run Go CI tests with coverage gate (>=80%)
auto_execution_mode: 3
---

Follow these steps to run CI tests locally and keep coverage healthy:

1. Ensure dependencies are ready
   - make deps
   - optional: make tools

// turbo
2. Run CI tests with coverage gate (>=80%)
   - Command: make ci-test

3. If tests fail or coverage <80%
   - Inspect failing packages and the coverage summary at `coverage/coverage.func.txt`
   - Open the HTML report for local review: `go tool cover -html=coverage/coverage.unit.out -o coverage/coverage.html` then view `coverage/coverage.html`
   - Fix the failing tests and increase coverage by adding or adjusting unit tests near their implementation (avoid top-level `test/` for unit tests; E2E lives in `test/e2e/`)
   - Prefer table-driven tests and cover edge/error paths
   - Repeat these fixes and rerun step 2 until tests pass and coverage ≥80%

4. Commit artifacts (optional)
   - Commit new/updated tests and minimal code fixes only
   - Do not commit `coverage/*.html` outputs

Notes
- The `ci-test` target already enforces a minimum 80% total statement coverage by parsing `coverage/coverage.unit.out`
- E2E tests are separate (see `make test-e2e` or `make run-e2e-tests`); this workflow is for unit tests only
