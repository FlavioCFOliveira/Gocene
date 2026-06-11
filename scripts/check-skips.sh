#!/usr/bin/env bash
# check-skips.sh — enforce no-skip policy and blocker token convention.
#
# Gocene policy (rmp #119): NEVER use t.Skip. A test gap must FAIL via
# t.Fatal with a descriptive reason naming the blocking capability or
# rmp task. This guard:
#   1. Rejects any remaining t.Skip/t.Skipf calls.
#   2. Validates that t.Fatal blocker calls include a descriptive reason.
#
# A "blocker" t.Fatal is one whose message starts with a keyword like
# "requires", "deferred", "not yet", "skip", "TODO", "blocked", or
# "rmp #". t.Fatal calls that are normal assertion failures (e.g.
# t.Fatalf("AddField: %v", err)) are not flagged.
#
# Usage: scripts/check-skips.sh   (run from repository root; CI: lint gate)

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

mapfile -t files < <(git ls-files '*_test.go')

# --- Rule 1: No t.Skip calls ---

skip_violations=0
for f in "${files[@]}"; do
  while IFS= read -r hit; do
    [ -n "$hit" ] || continue
    # Extract line number and content.
    line_no="${hit%%:*}"
    line_content="${hit#*:}"
    # Skip if the line is a comment (starts with // or is inside /* */).
    if echo "$line_content" | grep -qE '^\s*//'; then
      continue
    fi
    echo "t.Skip violation: $f:$hit"
    skip_violations=$((skip_violations + 1))
  done < <(grep -nE 't\.Skip(f)?\b' "$f" 2>/dev/null || true)
done

if [ "$skip_violations" -ne 0 ]; then
  echo ""
  echo "FAIL: $skip_violations t.Skip call(s) found." >&2
  echo "Gocene policy (rmp #119): NEVER use t.Skip. Use t.Fatal with a" >&2
  echo "descriptive reason naming the blocking capability or rmp task." >&2
  echo "See docs/skipped-tests-audit.md." >&2
  exit 1
fi

# --- Rule 2: t.Fatal blocker calls must have descriptive reasons ---

# A "blocker" t.Fatal starts with a keyword indicating deferred functionality.
# We look for t.Fatal calls whose message does NOT match the blocker pattern
# and flag them as potentially undocumented (these may be legitimate assertion
# failures, so this is a warning-only check).
blocker_pattern='t\.Fatal(f)?\(\s*"(requires|deferred|not yet|skip|TODO|blocked|rmp #|backlog #)'

blocker_warnings=0
for f in "${files[@]}"; do
  while IFS= read -r hit; do
    [ -n "$hit" ] || continue
    line_content="${hit#*:}"
    # Skip comment lines.
    if echo "$line_content" | grep -qE '^\s*(//|/\*)'; then
      continue
    fi
    # Check if this t.Fatal has a blocker-like reason.
    if ! echo "$hit" | grep -qE "$blocker_pattern"; then
      # Skip normal assertion patterns (format strings with %v, %w, %d, etc.)
      if echo "$hit" | grep -qE '%[vwdstfqx]'; then
        continue
      fi
      # Skip common test helpers.
      if echo "$hit" | grep -qE '(Fatalf|Errorf)\(.*"%[vwd]'; then
        continue
      fi
      echo "warning: possible undocumented blocker: $f:$hit"
      blocker_warnings=$((blocker_warnings + 1))
    fi
  done < <(grep -nE 't\.Fatal(f)?\b' "$f" 2>/dev/null || true)
done

if [ "$blocker_warnings" -ne 0 ]; then
  echo ""
  echo "WARNING: $blocker_warnings t.Fatal call(s) may lack blocker descriptions." >&2
  echo "Blocker t.Fatal messages should start with: requires, deferred, not yet," >&2
  echo "blocked, rmp #, or backlog #. See docs/skipped-tests-audit.md." >&2
  # Warning only — does not fail the build. CI lint gate is informative.
fi

echo "OK: no undocumented test skips."
