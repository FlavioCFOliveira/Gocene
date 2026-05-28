#!/usr/bin/env bash
# check-skips.sh — fail on undocumented test skips (rmp #4722).
#
# A t.Skip is a tracked debt; every skip MUST carry a non-empty reason so the
# skipped-test audit (docs/skipped-tests-audit.md) stays self-maintaining and
# new, silent skips cannot creep in. This guard rejects bare t.Skip() and
# empty-message t.Skip("")/t.Skipf("").
#
# Usage: scripts/check-skips.sh   (run from the repository root; CI: skip-guard)
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# Bare skip or empty-message skip. Whitespace-only messages are also rejected.
pattern='t\.Skipf?\(\s*("")?\s*\)'

# Limit to tracked Go test files (ignore agent worktrees, vendor, etc.).
mapfile -t files < <(git ls-files '*_test.go')

violations=0
for f in "${files[@]}"; do
  while IFS= read -r hit; do
    [ -n "$hit" ] || continue
    echo "undocumented skip: $f:$hit"
    violations=$((violations + 1))
  done < <(grep -nE "$pattern" "$f" 2>/dev/null || true)
done

if [ "$violations" -ne 0 ]; then
  echo ""
  echo "FAIL: $violations undocumented t.Skip call(s) found." >&2
  echo "Every t.Skip/t.Skipf must carry a non-empty reason naming the blocking" >&2
  echo "capability or rmp task. See docs/skipped-tests-audit.md." >&2
  exit 1
fi

echo "OK: no undocumented test skips."
