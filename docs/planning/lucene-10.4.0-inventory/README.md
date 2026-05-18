# Lucene 10.4.0 — Port Inventory

This directory contains the snapshot of the cross-reference between the
Apache Lucene 10.4.0 source tree and the current Gocene Go tree. It is the
authoritative input for the planning of sprints from 55 onward, which port
the remaining classes and tests.

## Provenance

- Reference repository: `https://github.com/apache/lucene.git`
- Reference tag: `releases/lucene/10.4.0`
- Reference commit: `9983b7c`
- Generated on: 2026-05-18
- Tooling: see `audit.py`, `pair_tests.py`, `build_java_inventory.sh`

## In-scope modules (27)

`scope_modules.txt` lists the Lucene modules that were inventoried. Excluded
from the port are: `luke`, `demo`, `benchmark`, `benchmark-jmh`,
`distribution`, `distribution.tests`, `dev-docs`, `documentation`,
`test-framework`, `spatial-test-fixtures`.

## Files

Inventory (Phase 1):

- `java_inventory.tsv` — every Java source and test file in the in-scope
  modules, with module, kind (`prod`/`test`), path, package, classname,
  fully-qualified name, and line count.
- `inventory.tsv` — `java_inventory.tsv` enriched with the matching Go file
  (basename or identifier match), status (`IMPLEMENTED`/`STUB`/`MISSING`),
  Go LOC, and a stub score derived from heuristics (panic markers,
  `_deferred.go` aggregators, undersized files).
- `pairs.tsv` — for each Java production class, its sibling Java test class
  (where the convention `Test<X>.java` or `<X>Test.java` applies). Used to
  document which port tasks already cover a test peer in their AC.

Task & sprint generation (Phase 2 / Phase 3):

- `generate_tasks.py` — emits one rmp task per MISSING / STUB prod class
  and one CHORE per standalone Java test that is still MISSING. Extracts
  the class-level Javadoc from the Java source to seed the FR field.
- `task_ids.tsv` — rmp task id, title, module, java_path, type. Generated
  by `generate_tasks.py --execute` on 2026-05-18.
- `plan_sprints.py` — dry-run sprint planner; buckets tasks by (module,
  top sub-package, kind) and produces `sprints_draft.tsv`.
- `create_sprints.py` — creates the 36 PENDING sprints in rmp (folds the
  core test/index chunks 1/2 + 2/2) and assigns the tasks via
  `rmp sprint add-tasks`.
- `sprints_draft.tsv` — initial planner output (37 sprints).
- `sprints_created.tsv` — final mapping `draft_idx -> rmp_sprint_id -> module
   -> sub-packages -> task_count` for the 36 sprints actually created.
- `sprint_assignments.tsv` — `draft_idx -> task_id` for every assignment.

## Heuristics (audit.py)

A Go file is treated as `STUB` (i.e., still requires work) when any of the
following holds:

- The body contains `panic("not implemented" | "TODO" | "unimplemented" | "stub" | "not yet ...")`.
- The Go file is `*_deferred.go` (these are Sprint-28-era aggregators with
  empty type/constructor declarations).
- The Go file is much shorter than the Java original (LOC < 30 with Java LOC
  > 80, or LOC < 50 with Java LOC > 200).
- The file contains `// TODO`, `// FIXME`, `// STUB`, or `// XXX` markers.

A Java class with no matching Go file (neither by basename nor by exported
identifier) is `MISSING`.

## Counts (2026-05-18)

| Kind | Total | IMPLEMENTED | STUB | MISSING |
|------|------:|------------:|-----:|--------:|
| prod | 3140  | 2081        | 220  | 839     |
| test | 1757  | 2           | 0    | 1755    |

Tasks created in `rmp` on 2026-05-18 (Phase 2):

- 839 `TASK` — port classes with status MISSING.
- 220 `REFACTOR` — replace STUBs with full implementations.
- 732 `CHORE` — port standalone Java tests (tests whose Java filename
  does not pair with a prod class by the `Test<X>` / `<X>Test`
  convention). The other 1023 paired test ports are covered by the
  acceptance criteria of the sibling implementation task, in line with
  the Gocene historical template.

Total: **1791 tasks** (IDs 2740-4530), all in BACKLOG -> SPRINT after
Phase 3. The Gocene historical template (one task per Java class,
acceptance criteria includes porting the Java test peer when present)
was preserved for consistency with the 2283 tasks completed in sprints
1-54.

## Phase 3 — Sprint layout

36 new sprints were created (`rmp` ids **55-90**), bucketed by
(module, top sub-package, kind=prod|test). Sprint sizes range from 2
(memory) to 127 (core/test-index), most fall in 30-120. The order
respects the dependency rationale used in sprints 1-54: util/store/codec
foundations first, then index/search, then analysis, then derivative
modules.

`sprints_created.tsv` records the mapping `draft_idx -> rmp_sprint_id ->
module -> sub-packages -> task_count`. `sprint_assignments.tsv` records
the assignment `draft_idx -> task_id` for each of the 1791 tasks.

## Regenerating the inventory

The shell and Python scripts in this directory are self-contained. To
re-run them after fetching a different Lucene tag or evolving Gocene:

```bash
# 1. (Re)clone the Lucene reference if /tmp/lucene is missing or outdated.
git clone --depth=1 --branch releases/lucene/10.4.0 \
    https://github.com/apache/lucene.git /tmp/lucene

# 2. List Go files in the Gocene tree (run from the repo root).
find . -name '*.go' -not -path './.git/*' -not -path './.lucene_ref/*' \
       -not -path './cmd/*' | sed 's|^\./||' \
       > /tmp/gocene-inventory/go_files.tsv

# 3. Build the raw Java inventory.
mkdir -p /tmp/gocene-inventory
cp docs/planning/lucene-10.4.0-inventory/scope_modules.txt \
   docs/planning/lucene-10.4.0-inventory/build_java_inventory.sh \
   docs/planning/lucene-10.4.0-inventory/audit.py \
   docs/planning/lucene-10.4.0-inventory/pair_tests.py \
   /tmp/gocene-inventory/
chmod +x /tmp/gocene-inventory/build_java_inventory.sh
/tmp/gocene-inventory/build_java_inventory.sh

# 4. Audit and pair tests.
python3 /tmp/gocene-inventory/audit.py
python3 /tmp/gocene-inventory/pair_tests.py

# 5. Copy refreshed artefacts back into the repo (manual review encouraged).
cp /tmp/gocene-inventory/*.tsv docs/planning/lucene-10.4.0-inventory/
```

## Known limitations

- Heuristic matching: a Java class is matched to a Go file by snake_case
  basename or by exported identifier. Classes whose Go port lives under an
  unconventional filename and uses a renamed identifier may be reported as
  `MISSING` (false negative). Conversely, a Go file with the same name but
  unrelated content would be reported as `IMPLEMENTED` (false positive).
  Spot-checks during sprint planning should refine the few outliers.
- Inner / nested Java classes are not enumerated separately; the port task
  for the enclosing class implicitly covers them.
- Test pairing uses the conventional Java naming `Test<X>` / `<X>Test`.
  Tests that exercise multiple classes or have purely descriptive names
  (e.g., `TestSearchAfterSortedMultiTermStreams`) appear as standalone test
  tasks without a `prod` sibling.
