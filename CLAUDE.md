# ai-cv-evaluator

`knowledge/` is an OKF v0.2 bundle. Read the concepts that touch the task before starting; write
them back in the same commit as the code. The `okf-knowledge-bundle` skill owns how.
Gate: `.githooks/pre-push`, which calls the checker directly and has no way out. It is the only
hook left: `.githooks/pre-commit` was deleted on 2026-08-29. `make lint-knowledge` (blocking,
`-Werror`) is the fast local loop, in `make lint-all` and CI.
