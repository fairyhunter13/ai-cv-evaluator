---
description: Tidy up and standardize documentation per .cursor/rules/docs.mdc (audit, consolidation, naming, links, index)
auto_execution_mode: 3
---

This workflow audits and tidies docs according to `/.cursor/rules/docs.mdc`.

Scope and exclusions
- **In-scope**: everything under `docs/` and any `*.md` outside `docs/` (except root `README.md`).
- **Excluded**: `docs/project.md` and everything under `docs/rfc/`.
- **No destructive auto-actions**: audit steps produce reports under `artifacts/docs-audit/`. You then apply fixes manually.

1. Prepare audit folder
// turbo
   - Command: mkdir -p artifacts/docs-audit

2. List stray markdown files outside `docs/` (should be moved)
// turbo
   - Command: bash -lc 'find . -type f -name "*.md" ! -path "./docs/*" ! -path "./vendor/*" ! -path "./admin-frontend/node_modules/*" ! -name "README.md" | sort | tee artifacts/docs-audit/stray_md.txt'

3. Enumerate in-scope docs (excluding `docs/project.md` and `docs/rfc/`)
// turbo
   - Command: bash -lc 'find ./docs -type f -name "*.md" ! -path "./docs/rfc/*" ! -path "./docs/project.md" | sort | tee artifacts/docs-audit/docs_in_scope.txt'

4. Audit headings (for navigation and similarity checks)
// turbo
   - Command: bash -lc 'rm -f artifacts/docs-audit/headings.txt; while IFS= read -r f; do echo "### $f" >> artifacts/docs-audit/headings.txt; sed -n "s/^#\{1,6\}[[:space:]]*//p" "$f" | tr "[:upper:]" "[:lower:]" | sed "s/[[:space:]]\{1,\}/ /g" >> artifacts/docs-audit/headings.txt; echo >> artifacts/docs-audit/headings.txt; done < artifacts/docs-audit/docs_in_scope.txt'

5. Duplicate content candidates by identical heading sets (heuristic)
// turbo
   - Command: bash -lc 'rm -f artifacts/docs-audit/dup_candidates.tsv artifacts/docs-audit/potential_duplicates.txt; while IFS= read -r f; do h=$(sed -n "s/^#\{1,6\}[[:space:]]*//p" "$f" | tr "[:upper:]" "[:lower:]" | sed "s/[[:space:]]\{1,\}/ /g"); if [ -n "$h" ]; then s=$(printf "%s" "$h" | shasum | awk "{print $1}"); printf "%s\t%s\n" "$s" "$f" >> artifacts/docs-audit/dup_candidates.tsv; fi; done < artifacts/docs-audit/docs_in_scope.txt; sort -o artifacts/docs-audit/dup_candidates.tsv artifacts/docs-audit/dup_candidates.tsv; awk -F "\t" '{c[$1]++} END {for (k in c) if (c[k]>1) print k}' artifacts/docs-audit/dup_candidates.tsv | while read -r k; do echo "== $k =="; awk -F "\t" -v key="$k" '$1==key {print $2}' artifacts/docs-audit/dup_candidates.tsv; echo; done | tee artifacts/docs-audit/potential_duplicates.txt'

6. Naming convention audit
   - Top-level of `docs/` (excluding `README.md` and `docs/project.md`) should be UPPER_SNAKE (e.g., `ARCHITECTURE.md`).
   - Subdirectories (excluding `docs/rfc/`) should be lower_snake (e.g., `getting_started.md`).
// turbo
   - Command: bash -lc 'find ./docs -maxdepth 1 -type f -name "*.md" ! -name "README.md" ! -path "./docs/project.md" | while read -r f; do bn=$(basename "$f"); echo "$bn" | grep -Eq "^[A-Z0-9_]+\.md$" || echo "$f"; done | tee artifacts/docs-audit/naming_top_level_violations.txt'
// turbo
   - Command: bash -lc 'find ./docs -mindepth 2 -type f -name "*.md" ! -path "./docs/rfc/*" | while read -r f; do bn=$(basename "$f"); echo "$bn" | grep -Eq "^[a-z0-9_]+\.md$" || echo "$f"; done | tee artifacts/docs-audit/naming_subdir_violations.txt'

7. Basic internal link audit (absolute `docs/...` and simple `../xxx.md` only)
// turbo
   - Command: bash -lc 'touch artifacts/docs-audit/broken_links.txt; while IFS= read -r f; do base=$(dirname "$f"); grep -oE "\]\((\.\./|docs/)[^)]+\.md\)" "$f" | sed -E "s/^.*\(([^)]+)\).*$/\1/" | while read -r l; do case "$l" in docs/*) [ -f "./$l" ] || echo "$f -> $l" >> artifacts/docs-audit/broken_links.txt ;; ../*) [ -f "$base/$l" ] || echo "$f -> $l" >> artifacts/docs-audit/broken_links.txt ;; esac; done; done < artifacts/docs-audit/docs_in_scope.txt; wc -l artifacts/docs-audit/broken_links.txt'

8. Verify `docs/README.md` as central navigation hub
// turbo
   - Command: bash -lc 'ls -1d ./docs/*/ 2>/dev/null | sed "s#^\./docs/##;s#/$##" | sort | tee artifacts/docs-audit/docs_dirs.txt; test -f ./docs/README.md && echo "README present" || echo "README missing"'

9. Review reports and apply fixes (manual)
   - **Move** any file listed in `artifacts/docs-audit/stray_md.txt` into an appropriate `docs/` subdirectory. Update links accordingly.
   - **Do not modify** `docs/project.md` or anything under `docs/rfc/`.
   - **Consolidate** duplicates suggested in `artifacts/docs-audit/potential_duplicates.txt` into a single source of truth. Remove or archive redundant files; update references.
   - **Rename** files that violate naming rules listed in `artifacts/docs-audit/naming_*_violations.txt`.
   - **Fix links** reported in `artifacts/docs-audit/broken_links.txt`.
   - **Update** `docs/README.md` to include/refresh top-level navigation reflecting current structure.

10. Re-run steps 1–8 until the reports are clean

11. Commit
   - Use a message like: "docs: tidy structure, consolidate duplicates, fix links, update index"
   - Ensure docs changes accompany any related code/config changes per synchronization rules.

Notes
- This workflow follows the rules in `/.cursor/rules/docs.mdc` (organization, synchronization, anti-duplication, quality).
- You may extend audits with additional tools (e.g., link checkers) if available, but avoid affecting `docs/project.md` and `docs/rfc/`.
