#!/usr/bin/env python3
"""
Create the 36 PENDING sprints in rmp from the draft produced by plan_sprints.py
and assign the BACKLOG tasks to them.

The script:
  1. Re-runs the bucketing logic from plan_sprints (kept in sync there) to
     obtain the ordered sprint list.
  2. Applies the manual merge of the core 'test/index' chunks (sprint 58 + 59
     in the draft) so the final count is 36 instead of 37.
  3. For each sprint: `rmp sprint create -d <description>`, capturing id.
  4. `rmp sprint add-tasks` to move BACKLOG tasks -> SPRINT in batches.

Writes /tmp/gocene-inventory/sprints_created.tsv mapping draft_idx -> rmp id.
"""

import json
import subprocess
import sys
import time
from pathlib import Path

TASK_IDS = Path("/tmp/gocene-inventory/task_ids.tsv")
OUT = Path("/tmp/gocene-inventory/sprints_created.tsv")
ROADMAP = "gocene"

# Mirror constants from plan_sprints.py
MIN_PER_SPRINT = 30
MAX_PER_SPRINT = 120


def subpackage_key(java_path: str) -> str:
    parts = java_path.split("/")
    is_test = "src/test" in java_path
    try:
        idx = parts.index("lucene")
    except ValueError:
        idx = -1
    if idx >= 0 and idx + 1 < len(parts):
        sub = parts[idx + 1]
        if sub == "internal" and idx + 2 < len(parts):
            sub = "internal/" + parts[idx + 2]
        return ("test/" if is_test else "") + sub
    return ("test/" if is_test else "") + "misc"


def plan_sprints() -> list[dict]:
    rows = [line.split("\t") for line in TASK_IDS.read_text().splitlines()[1:]]
    buckets: dict[tuple[str, str], list[list[str]]] = {}
    for r in rows:
        key = (r[2], subpackage_key(r[3]))
        buckets.setdefault(key, []).append(r)

    module_order = [
        "core", "backward-codecs", "codecs",
        "analysis/common", "analysis/icu", "analysis/kuromoji",
        "analysis/nori", "analysis/morfologik", "analysis/opennlp",
        "analysis/phonetic", "analysis/smartcn", "analysis/stempel",
        "queries", "queryparser", "suggest", "facet", "grouping",
        "highlighter", "join", "memory", "misc", "expressions",
        "sandbox", "monitor", "classification",
        "spatial-extras", "spatial3d", "replicator",
    ]
    subpkg_order_core = [
        "document", "util", "store", "geo", "internal", "codecs",
        "index", "search", "analysis",
    ]

    def subpkg_rank(module: str, sub: str) -> tuple:
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

        current: list[tuple[str, list]] = []
        current_count = 0

        def flush():
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
            })
            current = []
            current_count = 0

        for sub, lst in mod_buckets:
            if len(lst) > MAX_PER_SPRINT:
                flush()
                chunks = [lst[i:i + MAX_PER_SPRINT] for i in range(0, len(lst), MAX_PER_SPRINT)]
                for ci, chunk in enumerate(chunks, 1):
                    sprints.append({
                        "module": mod,
                        "subpkgs": f"{sub} (chunk {ci}/{len(chunks)})",
                        "task_count": len(chunk),
                        "task_ids": [r[0] for r in chunk],
                    })
                continue
            if current_count + len(lst) <= MAX_PER_SPRINT:
                current.append((sub, lst))
                current_count += len(lst)
            else:
                flush()
                current.append((sub, lst))
                current_count = len(lst)
        flush()

    return sprints


def merge_core_test_index(sprints: list[dict]) -> list[dict]:
    """Fold chunk 2/2 of core test/index into chunk 1/2 to avoid a tiny
    7-task sprint."""
    out: list[dict] = []
    i = 0
    while i < len(sprints):
        s = sprints[i]
        if (s["module"] == "core" and "test/index (chunk 1/" in s["subpkgs"]
                and i + 1 < len(sprints) and "test/index (chunk 2/" in sprints[i + 1]["subpkgs"]):
            merged = {
                "module": "core",
                "subpkgs": "test/index",
                "task_count": s["task_count"] + sprints[i + 1]["task_count"],
                "task_ids": s["task_ids"] + sprints[i + 1]["task_ids"],
            }
            out.append(merged)
            i += 2
        else:
            out.append(s)
            i += 1
    return out


def description_for(s: dict) -> str:
    # rmp sprint description is capped at 500 chars
    head = f"Sprint 55+: Port Lucene 10.4.0 — {s['module']}: {s['subpkgs']}."
    body = f" {s['task_count']} tasks (BACKLOG)."
    note = (
        " Scope: each task ports the linked Java class to Go (impl + tests in"
        " same task per Gocene historical template)."
    )
    desc = (head + body + note)[:500]
    return desc


def rmp_sprint_create(desc: str) -> int:
    cmd = ["rmp", "sprint", "create", "-r", ROADMAP, "-d", desc]
    res = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    if res.returncode != 0:
        raise RuntimeError(f"sprint create failed: {res.stderr.strip()}")
    return int(json.loads(res.stdout)["id"])


def rmp_sprint_add(sprint_id: int, task_ids: list[str]) -> None:
    BATCH = 100
    for i in range(0, len(task_ids), BATCH):
        chunk = task_ids[i:i + BATCH]
        cmd = ["rmp", "sprint", "add-tasks", "-r", ROADMAP,
               str(sprint_id), ",".join(chunk)]
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
        if res.returncode != 0:
            raise RuntimeError(
                f"sprint add-tasks failed (sprint={sprint_id}, "
                f"batch_start={i}, n={len(chunk)}): {res.stderr.strip()}"
            )


def main() -> int:
    sprints = merge_core_test_index(plan_sprints())
    print(f"Final sprint count: {len(sprints)}")
    OUT.write_text("draft_idx\trmp_id\tmodule\tsubpkgs\ttask_count\n")
    out_f = OUT.open("a")

    total = sum(s["task_count"] for s in sprints)
    print(f"Tasks to assign: {total}")

    t0 = time.time()
    for i, s in enumerate(sprints, 1):
        desc = description_for(s)
        sid = rmp_sprint_create(desc)
        rmp_sprint_add(sid, s["task_ids"])
        out_f.write(f"{i}\t{sid}\t{s['module']}\t{s['subpkgs']}\t{s['task_count']}\n")
        out_f.flush()
        elapsed = time.time() - t0
        print(f"  [{i}/{len(sprints)}] rmp sprint {sid}: {s['module']} ({s['task_count']} tasks)  elapsed={elapsed:.1f}s")
    out_f.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
