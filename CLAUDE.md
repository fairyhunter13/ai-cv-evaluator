# ai-cv-evaluator

`knowledge/` is an OKF v0.2 bundle. Read the concepts that touch the task before starting; write
them back in the same commit as the code. The `okf-knowledge-bundle` skill owns how.
Gate: `.githooks/pre-push`, which calls the checker directly and has no way out. `make
lint-knowledge` (blocking, `-Werror`) is the fast local loop, in `make lint-all`,
`.githooks/pre-commit` and CI — it reaches the checker only through `make` and honours
`SKIP_PRE_COMMIT_LINT`.
