---
type: Log
title: ai-cv-evaluator knowledge history
---

# Bundle history

## 2026-08-21

- **Update**: the vocabularies the spec closes, measured against the cloned reference bundles rather than against a reading of it — `generated.by` is `claude/opus-5` in 9 concepts, the one spelling of it that is §7's `<producer>/<version>`; 9 date-only `generated.at` values become instants; 4 numeric `[^1]` footnotes are keyed to a `sources[].id` (§5.1), because a position misattributes silently the moment the list is reordered. okf `v0.4.0` is what measures them: its `-strict` arm carries `ActorConvention`, `StatusVocabulary` and `FootnoteLabelsJoinSources`, and this bundle is now zero on all three.
- **Update**: the checker's module path changes and nothing else does. `okfrules` and `okf` merged into one module at okf `v0.3.0` -- the fleet rules are now the package `okf/rules` and the binary is built from `cmd/okfrules` in the okf module -- so `make tools`, `lint-knowledge` and the gate test install from the new path. Same binary name, same `Standard()` rules, same verdict on this bundle: `check -Werror` is silent before and after. The old module's tags stay resolvable on the proxy, so nothing pinned to them breaks.
- **Update**: the checker pin moves to `okfrules` v0.2.1, where `NoIntraBundleWikilinks` joins `Standard()`. It was `-strict`-only while one fleet bundle still carried bare wikilinks; that conversion finished and all ten bundles measured zero, so the rule now runs in every pinned repo rather than only the two that build their own checker. `Strict()` adds `LogVerbs` alone.

## 2026-08-20

- **Update**: `make tools`, `lint-knowledge` and the gate test install and run `okfrules` (`@v0.2.0`) rather than stock `okf` — the same conformance check plus the rules the fleet keeps and OKF §11 forbids a conformant consumer from enforcing: a `resource:` naming a path that is gone, a `type` outside the skills' table, a `verified:` stamp naming no human. The pin arm below grades the new pin, so all three move in one commit.
- **Creation**: [the bundle gate asserts that it is installed](decisions/the-bundle-gate-asserts-that-it-is-installed.md) — five arms in `internal/knowledgegate` red when `lint-knowledge` drops `-Werror` or swallows its exit, when the CI step reaching it is excused, when `.githooks/pre-commit` is `chmod -x` in the index, when the `okf` install loses its pin, or when `okf` accepts a concept with no `type` key. Each proved by breaking it. The leftover `continue-on-error` CI step that re-ran `make lint-knowledge` and discarded the answer is deleted; `lint-all` already runs it blocking.
- **Update**: `lint-knowledge` folds in `-Werror` and `lint-knowledge-strict` is deleted. The strict target ended in `|| true`, so the CI step that ran it reported nothing a build could act on; a broken link is a *warning*, which plain `okf check` also prints and exits 0 on. One blocking target now, reached from `lint-all`, the pre-commit hook and CI. All ten personal bundles were measured warning-free first, so this costs zero reds, and it was proved by adding a dangling link and watching `make lint-knowledge` exit 2.

## 2026-08-17

- **Update**: index entries now carry the concept's own `description`. `make lint-knowledge-strict`
  runs the `-Werror` lane and always exits 0; CI runs it `continue-on-error` beside `lint-all`,
  because a forward link to an unwritten concept is legitimate and must not decide a build.
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
