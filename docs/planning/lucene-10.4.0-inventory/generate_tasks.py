#!/usr/bin/env python3
"""
Generate rmp task-create commands for the Lucene 10.4.0 port plan.

Reads /tmp/gocene-inventory/inventory.tsv and emits tasks for:

  - Each prod class with status STUB         -> type=REFACTOR
  - Each prod class with status MISSING      -> type=TASK
  - Each standalone test (no prod sibling) with status MISSING -> type=CHORE

All tasks follow the established Gocene template (Title/FR/TR/AC), with
`lucene-authority,go-elite-engineer` as specialists.

If invoked with --dry-run the script only prints the rmp commands; with
--execute it runs them and captures the returned task IDs in
/tmp/gocene-inventory/task_ids.tsv.
"""

import argparse
import json
import re
import subprocess
import sys
import time
from pathlib import Path

INVENTORY = Path("/tmp/gocene-inventory/inventory.tsv")
TASK_IDS_OUT = Path("/tmp/gocene-inventory/task_ids.tsv")
PROGRESS_LOG = Path("/tmp/gocene-inventory/task_creation.log")
LUCENE_ROOT = Path("/tmp/lucene/lucene")  # Lucene module sources live under <repo>/lucene/<module>/...
ROADMAP = "gocene"
SPECIALISTS = "lucene-authority,go-elite-engineer"
MAX_FIELD = 4000  # rmp caps FR/TR/AC at 4096; we stay under


def to_snake_case(name: str) -> str:
    s = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", name)
    s = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", s)
    return s.lower()


def extract_class_javadoc(java_path: Path) -> str:
    """Extract the Javadoc block immediately preceding the first top-level
    type declaration (class | interface | enum | record). Returns the
    cleaned text (no leading * / indentation, no @tags)."""
    try:
        text = java_path.read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return ""
    # Find the first declaration of a top-level type
    decl_re = re.compile(
        r"^\s*(?:public\s+|final\s+|abstract\s+|sealed\s+|non-sealed\s+|static\s+|protected\s+|private\s+)*"
        r"(?:class|interface|enum|record)\s+",
        re.MULTILINE,
    )
    m = decl_re.search(text)
    if not m:
        return ""
    head = text[: m.start()]
    # Find last /** ... */ before this declaration
    jd_re = re.compile(r"/\*\*(.*?)\*/", re.DOTALL)
    last = None
    for jm in jd_re.finditer(head):
        last = jm
    if not last:
        return ""
    raw = last.group(1)
    lines = []
    for line in raw.splitlines():
        line = line.strip()
        if line.startswith("*"):
            line = line[1:].lstrip()
        # Drop @tag lines (e.g. @param, @return)
        if line.startswith("@"):
            continue
        lines.append(line)
    out = "\n".join(lines).strip()
    # Strip HTML-ish tags and curly Javadoc references for readability
    out = re.sub(r"\{@link\s+([^}]+)\}", r"`\1`", out)
    out = re.sub(r"\{@code\s+([^}]+)\}", r"`\1`", out)
    out = re.sub(r"<[^>]+>", "", out)
    # Squeeze runs of blank lines
    out = re.sub(r"\n\s*\n+", "\n\n", out)
    return out


def build_prod_task(row: dict, sibling_test_path: str | None) -> dict:
    fqdn = row["fqdn"]
    status = row["status"]
    java_path = LUCENE_ROOT / row["java_path"]
    summary = extract_class_javadoc(java_path)
    if not summary:
        summary = "(No top-level Javadoc found in the reference Java source.)"

    if status == "STUB":
        action = "Replace the existing Gocene stub for"
        ttype = "REFACTOR"
        stub_note = (
            "\n\nNote: a stub for this class already exists in the Gocene tree at "
            f"`{row['go_match_path']}` and must be replaced by a complete, "
            "byte-for-byte compatible implementation."
        )
    else:
        action = "Port"
        ttype = "TASK"
        stub_note = ""

    title = f"{action} `{fqdn}`"

    fr_lines = [
        f"Provide a Go implementation of `{fqdn}` that is byte-for-byte compatible with the Apache Lucene 10.4.0 reference. The Go port must preserve the public contract and observable behavior of the Java original.",
        "",
        "Reference Javadoc (Lucene 10.4.0):",
        summary,
    ]
    fr = "\n".join(fr_lines)
    if stub_note:
        fr += stub_note
    fr = fr[:MAX_FIELD]

    tr_lines = [
        f"Source (Lucene 10.4.0): {row['java_path']}",
    ]
    if sibling_test_path:
        tr_lines.append("Reference test peer(s) in Lucene 10.4.0:")
        tr_lines.append(f"  - {sibling_test_path}")
    else:
        tr_lines.append(
            "Reference test peer(s) in Lucene 10.4.0: none located by the "
            f"`Test{row['classname']}` / `{row['classname']}Test` convention."
        )
    tr_lines.append("")
    tr_lines.append(
        f"Target package guess: `{row['go_match_path']}` "
        f"(snake-case basename: `{row['go_basename_guess']}.go`)."
        if row["go_match_path"]
        else f"Target package: derive from the Java package "
        f"`{row['package']}` (snake-case basename: `{row['go_basename_guess']}.go`)."
    )
    tr = "\n".join(tr_lines)[:MAX_FIELD]

    ac = (
        "- Go implementation lives under the corresponding package in the Gocene module.\n"
        "- Unit tests in Go reproduce the Java test peer(s) listed in TR (one Go test per Java test peer where present).\n"
        "- Test results in Go are validated against the outputs of the Java test peer(s) executed against Apache Lucene 10.4.0.\n"
        "- Where the class produces serialized output (codec/index/store), the byte stream is identical to the Java reference for the same inputs.\n"
        "- `go build ./...` passes; no `panic(\"not implemented\")` or TODO markers remain in the implementation body."
    )

    return {
        "title": title[:255],
        "type": ttype,
        "fr": fr,
        "tr": tr,
        "ac": ac,
        "specialists": SPECIALISTS,
    }


def build_test_task(row: dict) -> dict:
    fqdn = row["fqdn"]
    java_path = LUCENE_ROOT / row["java_path"]
    summary = extract_class_javadoc(java_path)
    if not summary:
        summary = "(No top-level Javadoc found in the reference Java test source.)"

    title = f"Port test `{fqdn}`"

    fr_lines = [
        f"Port the Apache Lucene 10.4.0 test `{fqdn}` into the Gocene tree. The Go port must exercise the same behaviors and edge cases as the Java original, with assertion semantics preserved.",
        "",
        "Reference Javadoc (Lucene 10.4.0):",
        summary,
    ]
    fr = "\n".join(fr_lines)[:MAX_FIELD]

    tr_lines = [
        f"Source (Lucene 10.4.0 test): {row['java_path']}",
        "",
        f"Target: a Go test file living in the Gocene package that hosts the classes exercised by `{fqdn}`. Use idiomatic Go testing (table-driven where appropriate). Where the Java test relies on `LuceneTestCase` utilities (random `Directory`, `Analyzer`, etc.), use existing Gocene test helpers or introduce equivalents.",
    ]
    tr = "\n".join(tr_lines)[:MAX_FIELD]

    ac = (
        "- Go test file exists in the appropriate Gocene package and compiles.\n"
        "- Every `@Test` method in the Java source has a counterpart in the Go test (1:1 or merged where Go conventions prefer).\n"
        "- `go test ./<package>/...` passes, exercising the same assertion intents as the Java reference.\n"
        "- Where the Java test uses fixture files under `src/test-files/...`, the corresponding fixtures are vendored into the Gocene tree under a sibling test-data directory."
    )

    return {
        "title": title[:255],
        "type": "CHORE",
        "fr": fr,
        "tr": tr,
        "ac": ac,
        "specialists": SPECIALISTS,
    }


def _safe(value: str) -> str:
    """rmp's CLI parser rejects flag values that start with '-' (it treats them
    as the next flag). Prefix one space to escape, which the tool stores
    verbatim but does not affect rendered Markdown."""
    if value and value.startswith("-"):
        return " " + value
    return value


def rmp_create_task(task: dict) -> int:
    cmd = [
        "rmp", "task", "create",
        "-r", ROADMAP,
        "-t", _safe(task["title"]),
        "--type", task["type"],
        "-fr", _safe(task["fr"]),
        "-tr", _safe(task["tr"]),
        "-ac", _safe(task["ac"]),
        "-sp", _safe(task["specialists"]),
    ]
    res = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    if res.returncode != 0:
        raise RuntimeError(f"rmp create failed: stderr={res.stderr.strip()} stdout={res.stdout.strip()}")
    obj = json.loads(res.stdout)
    return int(obj["id"])


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--execute", action="store_true",
                    help="Actually call rmp task create (else dry-run prints).")
    ap.add_argument("--modules", help="Comma-separated module names to limit work.")
    ap.add_argument("--limit", type=int, default=0, help="Stop after N tasks (0 = no limit).")
    args = ap.parse_args()

    inv = INVENTORY.read_text().splitlines()
    header = inv[0].split("\t")
    idx = {h: i for i, h in enumerate(header)}
    rows = [dict(zip(header, ln.split("\t"))) for ln in inv[1:]]

    module_filter = None
    if args.modules:
        module_filter = set(args.modules.split(","))

    # Build sibling map: (pkg, classname) -> [test rows]
    test_by_pkg_class: dict[tuple[str, str], list[dict]] = {}
    for r in rows:
        if r["kind"] != "test":
            continue
        test_by_pkg_class.setdefault((r["package"], r["classname"]), []).append(r)

    prod_test_keys: set[tuple[str, str]] = set()
    for r in rows:
        if r["kind"] != "prod":
            continue
        for cand in (f"Test{r['classname']}", f"{r['classname']}Test",
                     f"{r['classname']}Tests", f"Test{r['classname']}s"):
            prod_test_keys.add((r["package"], cand))

    # Order: prod first (so we can later link test-only tasks to prod task IDs);
    # but for the historical template we do not need cross-links.
    work: list[tuple[str, dict, dict]] = []  # kind_label, source_row, task_dict

    for r in rows:
        if r["kind"] != "prod":
            continue
        if r["status"] == "IMPLEMENTED":
            continue
        if module_filter and r["module"] not in module_filter:
            continue
        # Find a sibling test
        sibling_path = ""
        for cand in (f"Test{r['classname']}", f"{r['classname']}Test",
                     f"{r['classname']}Tests", f"Test{r['classname']}s"):
            ts = test_by_pkg_class.get((r["package"], cand))
            if ts:
                sibling_path = ts[0]["java_path"]
                break
        if not sibling_path:
            parent_pkg = ".".join(r["package"].split(".")[:-1])
            for cand in (f"Test{r['classname']}", f"{r['classname']}Test",
                         f"{r['classname']}Tests", f"Test{r['classname']}s"):
                ts = test_by_pkg_class.get((parent_pkg, cand))
                if ts:
                    sibling_path = ts[0]["java_path"]
                    break
        task = build_prod_task(r, sibling_path or None)
        work.append(("prod", r, task))

    for r in rows:
        if r["kind"] != "test":
            continue
        if r["status"] == "IMPLEMENTED":
            continue
        if module_filter and r["module"] not in module_filter:
            continue
        key = (r["package"], r["classname"])
        if key in prod_test_keys:
            continue  # covered by the sibling prod task's AC
        # Also skip if this test name pairs with a prod via parent_pkg
        # (lightweight check: skip if the test name strips to a prod classname
        # already in scope).
        # Build standalone CHORE
        task = build_test_task(r)
        work.append(("test", r, task))

    print(f"Total tasks to create: {len(work)}")
    counts = {"prod_STUB": 0, "prod_MISSING": 0, "test_standalone": 0}
    for kind, src, task in work:
        if kind == "prod" and src["status"] == "STUB":
            counts["prod_STUB"] += 1
        elif kind == "prod" and src["status"] == "MISSING":
            counts["prod_MISSING"] += 1
        elif kind == "test":
            counts["test_standalone"] += 1
    print(f"  prod_STUB (REFACTOR):     {counts['prod_STUB']}")
    print(f"  prod_MISSING (TASK):      {counts['prod_MISSING']}")
    print(f"  test_standalone (CHORE):  {counts['test_standalone']}")

    if args.limit:
        work = work[: args.limit]

    if not args.execute:
        # Dry-run: print first few task title+type only
        for kind, src, task in work[:8]:
            print(f"  [{task['type']:8s}] {task['title']}")
        if len(work) > 8:
            print(f"  ... (+{len(work) - 8} more)")
        return 0

    # Execute
    if TASK_IDS_OUT.exists():
        # Resume: load existing IDs
        seen_titles = {}
        for line in TASK_IDS_OUT.read_text().splitlines():
            parts = line.split("\t")
            if len(parts) >= 2:
                seen_titles[parts[1]] = parts[0]
    else:
        TASK_IDS_OUT.write_text("task_id\ttitle\tmodule\tjava_path\ttype\n")
        seen_titles = {}

    PROGRESS_LOG.write_text("")
    log = PROGRESS_LOG.open("a")
    out = TASK_IDS_OUT.open("a")

    total = len(work)
    t0 = time.time()
    last_report = t0
    for i, (kind, src, task) in enumerate(work, 1):
        if task["title"] in seen_titles:
            continue
        try:
            tid = rmp_create_task(task)
        except Exception as exc:
            log.write(f"FAIL\t{task['title']}\t{exc}\n")
            log.flush()
            continue
        out.write(f"{tid}\t{task['title']}\t{src['module']}\t{src['java_path']}\t{task['type']}\n")
        out.flush()
        # Progress report every 5 seconds or every 100 tasks
        now = time.time()
        if (now - last_report > 5) or (i % 100 == 0) or (i == total):
            elapsed = now - t0
            rate = i / elapsed if elapsed > 0 else 0
            eta = (total - i) / rate if rate > 0 else 0
            print(f"  [{i}/{total}] {rate:.1f} tasks/s, eta {eta:.0f}s, last id={tid}")
            last_report = now
    out.close()
    log.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
