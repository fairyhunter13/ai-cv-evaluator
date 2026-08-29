---
type: Decision
resource: internal/knowledgegate/gate_test.go
title: The bundle gate asserts that it is installed, not only that the bundle passes
description: Four arms fail. Two are lint-knowledge dropping -Werror, and the CI step reaching it being excused. The third is the okf install losing its pin, and the fourth is a checker that accepts everything. A fifth arm graded the pre-commit hook, which was deleted on 2026-08-29.
tags: [okf, knowledge, gates, ci, makefile]
status: stable
generated: { by: claude/opus-5, at: 2026-08-20T21:00:00Z }
---

# What the gate is

`make lint-knowledge` runs `okf check -Werror knowledge` and blocks. `lint-all` reaches it, and
`lint-all` is what CI runs. `.githooks/pre-commit` ran it too until 2026-08-29.

# Why it is tested

The bundle passing `okf` says nothing about whether anything ran `okf`. Each way this gate can be
present and decide nothing has an arm in `internal/knowledgegate`:

- `lint-knowledge` stops passing `-Werror`, or grows a `|| true`. A broken link is a *warning*:
  plain `check` prints it and exits 0. This is the shape `lint-knowledge-strict` had before it was
  deleted.
- The CI step that reaches it becomes `continue-on-error`. Only that step is graded — the gosec
  SARIF upload and the FOSSA scan are excused deliberately, and grading every step in the file
  would make this test about somebody else's policy.
- The `okf` install loses its pin. An unpinned install lets this gate's verdict change with no
  commit in this repo.
- `okf` itself accepts everything. The last arm feeds it a concept with no `type` key and requires
  a non-zero exit — the only arm that would notice a checker that had stopped checking.

Every arm was proved by breaking the thing it names and watching it red.

A fifth arm held that `.githooks/pre-commit` was `100755` in the index, because a hook chmod -x'd
there is planted non-executable in every clone and git skips it without printing a word. That hook
and that arm went together on 2026-08-29, under the ruling that six rules keep a mechanism. A gate
on the gate is the first thing that ruling removes.

A second CI step ran `make lint-knowledge` again under `continue-on-error: true`, left over from
when the strict half was advisory. It is deleted: `lint-all` already covers the bundle blocking, so
the step re-ran the same check and threw the answer away.
