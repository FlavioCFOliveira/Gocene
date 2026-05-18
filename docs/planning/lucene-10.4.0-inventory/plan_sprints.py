#!/usr/bin/env python3
"""
Group the BACKLOG tasks created in Phase 2 into coherent sprints (Phase 3
draft). Emits a TSV describing the proposed sprints; does NOT call rmp yet.

Heuristic:
  - Bucket by (module, top sub-package, kind=prod|test).
  - Merge tiny buckets within the same module up to MIN_PER_SPRINT.
  - Split large buckets into chunks of <= MAX_PER_SPRINT.
"""

import sys
from pathlib import Path

TASK_IDS = Path("/tmp/gocene-inventory/task_ids.tsv")
OUT = Path("/tmp/gocene-inventory/sprints_draft.tsv")

MIN_PER_SPRINT = 30
MAX_PER_SPRINT = 120


def subpackage_key(java_path: str) -> str:
    """Return a stable key describing the top sub-package of a java_path.

    Examples:
      core/src/java/org/apache/lucene/index/IndexWriter.java -> 'index'
      core/src/test/org/apache/lucene/search/TestFoo.java     -> 'test/search'
      analysis/common/src/java/org/apache/lucene/analysis/hunspell/Stemmer.java -> 'hunspell'
      analysis/common/src/test/org/apache/lucene/analysis/hunspell/Foo.java -> 'test/hunspell'
    """
    parts = java_path.split("/")
    is_test = "src/test" in java_path
    # Drop the prefix module/src/{java,java21,test}/
    try:
        idx = parts.index("lucene") if "lucene" in parts else -1
        if idx >= 0 and idx + 1 < len(parts):
            sub = parts[idx + 1]
            if sub == "internal" and idx + 2 < len(parts):
                sub = "internal/" + parts[idx + 2]
            return ("test/" if is_test else "") + sub
    except ValueError:
        pass
    return ("test/" if is_test else "") + "misc"


def main() -> int:
    rows = [line.split("\t") for line in TASK_IDS.read_text().splitlines()[1:]]
    # task_id, title, module, java_path, type

    # Bucket per (module, subpkg)
    buckets: dict[tuple[str, str], list[list[str]]] = {}
    for r in rows:
        key = (r[2], subpackage_key(r[3]))
        buckets.setdefault(key, []).append(r)

    # Order modules following the dependency-friendly history used in
    # sprints 1..54: util -> store -> codec -> index -> search -> analysis
    # -> derivatives. The remaining ones we order alphabetically.
    module_order = [
        "core",
        "backward-codecs",
        "codecs",
        "analysis/common",
        "analysis/icu",
        "analysis/kuromoji",
        "analysis/nori",
        "analysis/morfologik",
        "analysis/opennlp",
        "analysis/phonetic",
        "analysis/smartcn",
        "analysis/stempel",
        "queries",
        "queryparser",
        "suggest",
        "facet",
        "grouping",
        "highlighter",
        "join",
        "memory",
        "misc",
        "expressions",
        "sandbox",
        "monitor",
        "classification",
        "spatial-extras",
        "spatial3d",
        "replicator",
    ]
    # Subpackage order within core (rough dependency order)
    subpkg_order_core = [
        "document", "util", "store", "geo", "internal", "codecs",
        "index", "search", "analysis", "search/comparators", "search/knn",
        "search/similarities", "search/spans", "search/uhighlight",
    ]

    def subpkg_rank(module: str, sub: str) -> tuple[int, str]:
        # prod before test for the same subpackage
        is_test = sub.startswith("test/")
        base = sub[len("test/"):] if is_test else sub
        if module == "core":
            try:
                rank = subpkg_order_core.index(base.split("/")[0])
            except ValueError:
                rank = 100
        else:
            rank = 0
        return (rank, 1 if is_test else 0, base)

    sprints: list[dict] = []
    for mod in module_order:
        mod_buckets = [(sub, lst) for (m, sub), lst in buckets.items() if m == mod]
        mod_buckets.sort(key=lambda x: subpkg_rank(mod, x[0]))
        if not mod_buckets:
            continue

        # Pack into sprint(s). Try to keep MIN_PER_SPRINT <= size <= MAX_PER_SPRINT.
        current: list[tuple[str, list]] = []
        current_count = 0

        def flush(reason: str):
            nonlocal current, current_count
            if not current:
                return
            subs = ",".join(sub for sub, _ in current)
            ids = []
            for _, lst in current:
                ids.extend(r[0] for r in lst)
            sprints.append({
                "module": mod,
                "subpkgs": subs,
                "task_count": current_count,
                "task_ids": ids,
                "reason": reason,
            })
            current = []
            current_count = 0

        for sub, lst in mod_buckets:
            # If this single bucket exceeds MAX, split it
            if len(lst) > MAX_PER_SPRINT:
                # First flush whatever we have
                flush("max-before-large")
                # Then chunk this bucket
                chunks = [lst[i:i + MAX_PER_SPRINT] for i in range(0, len(lst), MAX_PER_SPRINT)]
                for ci, chunk in enumerate(chunks, 1):
                    sprints.append({
                        "module": mod,
                        "subpkgs": f"{sub} (chunk {ci}/{len(chunks)})",
                        "task_count": len(chunk),
                        "task_ids": [r[0] for r in chunk],
                        "reason": "large-bucket-split",
                    })
                continue
            if current_count + len(lst) <= MAX_PER_SPRINT:
                current.append((sub, lst))
                current_count += len(lst)
            else:
                flush("max-cap")
                current.append((sub, lst))
                current_count = len(lst)
            if current_count >= MIN_PER_SPRINT:
                # Optionally flush to keep sprints small enough
                pass
        flush("module-end")

    # Now post-process: if a sprint has < MIN_PER_SPRINT AND the next sprint
    # is in the same module, do nothing (history shows small modules became
    # their own sprint). We accept some sprints < MIN if they cover an entire
    # small module.

    # Write draft
    with OUT.open("w") as f:
        f.write("sprint_idx\tmodule\tsubpkgs\ttask_count\ttask_ids_first10\ttotal_ids\n")
        for i, s in enumerate(sprints, 1):
            f.write(f"{i}\t{s['module']}\t{s['subpkgs']}\t{s['task_count']}\t"
                    f"{','.join(s['task_ids'][:10])}\t{len(s['task_ids'])}\n")

    # Also write task->sprint mapping (for execution later)
    mapping_path = Path("/tmp/gocene-inventory/sprint_assignments.tsv")
    with mapping_path.open("w") as f:
        f.write("sprint_idx\ttask_id\n")
        for i, s in enumerate(sprints, 1):
            for tid in s["task_ids"]:
                f.write(f"{i}\t{tid}\n")

    print(f"Sprints drafted: {len(sprints)}")
    print(f"Total tasks accounted for: {sum(s['task_count'] for s in sprints)}")
    print()
    print(f"{'#':>3}  {'tasks':>5}  module/subpkgs")
    print("-" * 80)
    for i, s in enumerate(sprints, 1):
        sub = s["subpkgs"]
        if len(sub) > 60:
            sub = sub[:57] + "..."
        flag = ""
        if s["task_count"] < MIN_PER_SPRINT:
            flag = " (small)"
        if s["task_count"] > MAX_PER_SPRINT:
            flag = " (LARGE!)"
        print(f"{i:>3}  {s['task_count']:>5}  {s['module']}: {sub}{flag}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
