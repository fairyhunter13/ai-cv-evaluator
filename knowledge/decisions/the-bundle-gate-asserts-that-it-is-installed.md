---
type: Decision
resource: internal/knowledgegate/gate_test.go
title: The bundle gate asserts that it is installed, not only that the bundle passes
description: Five arms fail when lint-knowledge stops passing -Werror, when the CI step that reaches it is excused, when the pre-commit hook is non-executable in the index, when the okf install loses its pin, or when the checker accepts everything.
tags: [okf, knowledge, gates, ci, makefile]
status: stable
generated: { by: claude/opus-5, at: 2026-08-20T21:00:00Z }
---

# What the gate is

`make lint-knowledge` runs `okf check -Werror knowledge` and blocks. `lint-all` reaches it, and
`lint-all` is what CI and `.githooks/pre-commit` run. One target, three callers.

# Why it is tested

The bundle passing `okf` says nothing about whether anything ran `okf`. Each way this gate can be
present and decide nothing has an arm in `internal/knowledgegate`:

- `lint-knowledge` stops passing `-Werror`, or grows a `|| true`. A broken link is a *warning*:
  plain `check` prints it and exits 0. This is the shape `lint-knowledge-strict` had before it was
  deleted.
- The CI step that reaches it becomes `continue-on-error`. Only that step is graded — the gosec
  SARIF upload and the FOSSA scan are excused deliberately, and grading every step in the file
  would make this test about somebody else's policy.
- `.githooks/pre-commit` is `chmod -x` **in the index**. It looks executable in this working tree
  and is planted non-executable in every clone, and git skips a hook it cannot execute without
  printing a word.
- The `okf` install loses its pin. An unpinned install lets this gate's verdict change with no
  commit in this repo.
- `okf` itself accepts everything. The last arm feeds it a concept with no `type` key and requires
  a non-zero exit — the only arm that would notice a checker that had stopped checking.

Every arm was proved by breaking the thing it names and watching it red.

A second CI step ran `make lint-knowledge` again under `continue-on-error: true`, left over from
when the strict half was advisory. It is deleted: `lint-all` already covers the bundle blocking, so
the step re-ran the same check and threw the answer away.
