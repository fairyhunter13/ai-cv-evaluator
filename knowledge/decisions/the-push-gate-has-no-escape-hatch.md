---
type: Decision
resource: .githooks/pre-push
title: The push gate has no escape hatch the commit gate has
description: pre-commit reaches the checker only through make lint-all, exits 0 when make is absent and is skipped outright by SKIP_PRE_COMMIT_LINT=1 — the right shape for a fast local loop and the wrong shape for a gate, so pre-push calls the checker directly.
tags: [okf, knowledge, gate, hooks, make]
generated:
  by: claude/opus-5
  at: 2026-08-21T09:30:00Z
---

# The choice

`.githooks/pre-push` calls `okfrules check -Werror knowledge` directly, preferring `bin/okfrules`
and falling back to PATH. It has no environment variable that skips it and no branch that exits 0
when a tool is missing. `git push --no-verify` is the only way past it.

# What it replaced, and why that failed

`.githooks/pre-commit` was the only local arm, and it reaches the checker at three removes:

- it runs `make lint-all`, so `lint-knowledge` is reached only if the Makefile still routes there;
- it prints `make not found; skipping lint-all` and **exits 0** when `make` is absent;
- it honours `SKIP_PRE_COMMIT_LINT=1`.

All three are correct for what that hook is — a fast local loop over backend, frontend, infra,
docs and knowledge at once, which has to stay skippable. None of them is correct for a gate. A
commit gate whose two outcomes are "pass" and "skipped, exit 0" measures the machine it ran on.

The make path stays. This is a second arm at a later moment, not a replacement.

# Why it prefers bin/okfrules

`make tools` installs the pinned checker into `bin/`, which is where the Makefile's own
`lint-knowledge` finds it. Preferring the same binary means the hook and the make target cannot
disagree about which checker's verdict counts. A PATH copy is the fallback for a checkout that
has not run `make tools`.

# Why the hook asserts things about itself

`internal/knowledgegate/gate_test.go` asserts `.githooks/pre-push` is tracked and mode `100755` in
the *index* — one chmod -x'd there is planted non-executable in every future clone and git skips it
without printing anything — and that its executable lines carry none of the three escapes above.
The hook's own refusal probe covers the rest: it feeds the checker a concept with no `type` key
and blocks if that is accepted, which was verified by pointing `bin/okfrules` at a stub that exits
0 on everything.
