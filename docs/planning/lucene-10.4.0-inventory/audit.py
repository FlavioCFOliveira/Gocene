#!/usr/bin/env python3
"""
Cross-reference every Lucene 10.4.0 in-scope Java file against the current
Gocene Go tree.

Output columns:
  module  kind  java_path  package  classname  fqdn  java_loc
  go_basename_guess  go_match_path  status  go_loc  go_stub_score
"""

import os
import re
import sys
from pathlib import Path

REPO_ROOT = Path("/home/flavio/dev/github.com/FlavioCFOliveira/Gocene")
JAVA_TSV = Path("/tmp/gocene-inventory/java_inventory.tsv")
GO_TSV = Path("/tmp/gocene-inventory/go_files.tsv")
OUT_TSV = Path("/tmp/gocene-inventory/inventory.tsv")


def to_snake_case(name: str) -> str:
    s = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", name)
    s = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", s)
    return s.lower()


def package_to_path_hint(pkg: str) -> str:
    if pkg.startswith("org.apache.lucene."):
        rest = pkg[len("org.apache.lucene."):]
    elif pkg.startswith("org.tartarus.snowball"):
        rest = "snowball" + pkg[len("org.tartarus.snowball"):].replace(".", "/")
    elif pkg.startswith("org.egothor.stemmer"):
        rest = "stemmer"
    else:
        rest = pkg.replace(".", "/")
    return rest.replace(".", "/")


PURE_STUB_RE = re.compile(
    r'panic\(\s*"[^"]*(not implemented|TODO|unimplemented|stub|not yet)',
    re.IGNORECASE,
)
TODO_RE = re.compile(r'//\s*(TODO|FIXME|STUB|XXX)[: ]')


def go_index_definitions(files: list[Path]) -> dict[str, list[Path]]:
    """Index Go files by exported type/func name they define.

    Returns map: identifier -> [paths defining it].
    """
    type_re = re.compile(r"^type\s+([A-Z][A-Za-z0-9_]*)\b")
    func_re = re.compile(r"^func\s+(?:\([^)]*\)\s*)?(?:New)?([A-Z][A-Za-z0-9_]*)\b")

    index: dict[str, set[Path]] = {}
    for p in files:
        full = REPO_ROOT / p
        try:
            text = full.read_text(encoding="utf-8", errors="ignore")
        except OSError:
            continue
        for line in text.splitlines():
            m = type_re.match(line)
            if m:
                index.setdefault(m.group(1), set()).add(p)
                continue
            m = func_re.match(line)
            if m and not p.name.endswith("_test.go"):
                index.setdefault(m.group(1), set()).add(p)
    return {k: sorted(v) for k, v in index.items()}


def score_go_file(path: Path, java_loc: int) -> tuple[int, int]:
    try:
        text = path.read_text(encoding="utf-8", errors="ignore")
    except OSError:
        return 0, 99
    loc = text.count("\n")
    score = 0
    if PURE_STUB_RE.search(text):
        score += 3
    score += len(TODO_RE.findall(text))
    if "_deferred.go" in path.name:
        score += 5
    if loc < 30 and java_loc > 80:
        score += 2
    if loc < 50 and java_loc > 200:
        score += 1
    return loc, score


def main() -> int:
    go_files_all = [Path(line.strip()) for line in GO_TSV.read_text().splitlines() if line.strip()]
    go_prod = [p for p in go_files_all if not p.name.endswith("_test.go")]
    go_test = [p for p in go_files_all if p.name.endswith("_test.go")]

    # Basename indexes (without .go and without _test)
    by_basename_prod: dict[str, list[Path]] = {}
    for p in go_prod:
        key = p.name[:-3]
        by_basename_prod.setdefault(key, []).append(p)

    by_basename_test: dict[str, list[Path]] = {}
    for p in go_test:
        key = p.name[:-3]
        if key.endswith("_test"):
            key = key[: -len("_test")]
        by_basename_test.setdefault(key, []).append(p)

    # Identifier indexes (find `type X` / `func NewX` definitions)
    ident_prod = go_index_definitions(go_prod)
    ident_test = go_index_definitions(go_test)

    rows: list[list[str]] = []
    rows.append([
        "module", "kind", "java_path", "package", "classname", "fqdn",
        "java_loc", "go_basename_guess", "go_match_path", "status",
        "go_loc", "go_stub_score",
    ])

    def best_candidate(candidates: list[Path], pkg_hint: str) -> Path | None:
        if not candidates:
            return None
        if len(candidates) == 1:
            return candidates[0]
        def share(p: Path) -> int:
            pp = str(p.parent).replace("\\", "/")
            n = 0
            while pp and pkg_hint and pp[0] == pkg_hint[0]:
                pp = pp[1:]; ph = pkg_hint[1:]; n += 1
                pkg_hint_local = ph  # type: ignore
            return n
        # simpler: max common prefix
        def cp(a: str, b: str) -> int:
            n = 0
            for x, y in zip(a, b):
                if x != y:
                    return n
                n += 1
            return n
        return max(candidates, key=lambda p: cp(str(p.parent).replace("\\", "/"), pkg_hint))

    with JAVA_TSV.open() as f:
        for line in f:
            parts = line.rstrip("\n").split("\t")
            if len(parts) != 7:
                continue
            module, kind, java_path, pkg, classname, fqdn, java_loc_s = parts
            try:
                java_loc = int(java_loc_s)
            except ValueError:
                java_loc = 0
            snake = to_snake_case(classname)
            pkg_hint = package_to_path_hint(pkg)

            if kind == "prod":
                by_base = by_basename_prod
                ident = ident_prod
            else:
                by_base = by_basename_test
                ident = ident_test

            # Stage 1: basename match
            candidates = list(by_base.get(snake, []))
            # Stage 2: identifier match (handles _deferred.go aggregators etc.)
            if not candidates and classname in ident:
                candidates = list(ident[classname])

            chosen = best_candidate(candidates, pkg_hint)

            if chosen is None:
                status = "MISSING"
                go_loc = 0
                stub_score = 0
                go_match = ""
            else:
                go_path = REPO_ROOT / chosen
                go_loc, stub_score = score_go_file(go_path, java_loc)
                if stub_score >= 2:
                    status = "STUB"
                else:
                    status = "IMPLEMENTED"
                go_match = str(chosen)

            rows.append([
                module, kind, java_path, pkg, classname, fqdn,
                str(java_loc), snake, go_match, status,
                str(go_loc), str(stub_score),
            ])

    OUT_TSV.write_text("\n".join("\t".join(r) for r in rows) + "\n")

    total = len(rows) - 1
    print(f"Total rows: {total}")
    for kind in ("prod", "test"):
        sub = [r for r in rows[1:] if r[1] == kind]
        n = len(sub)
        impl = sum(1 for r in sub if r[9] == "IMPLEMENTED")
        stub = sum(1 for r in sub if r[9] == "STUB")
        miss = sum(1 for r in sub if r[9] == "MISSING")
        print(f"{kind:5s}: {n:5d} total  IMPL={impl:5d}  STUB={stub:5d}  MISSING={miss:5d}")

    # Per-module breakdown for prod
    print("\nPer-module prod breakdown:")
    by_mod: dict[str, list[list[str]]] = {}
    for r in rows[1:]:
        if r[1] != "prod":
            continue
        by_mod.setdefault(r[0], []).append(r)
    for mod in sorted(by_mod):
        sub = by_mod[mod]
        impl = sum(1 for r in sub if r[9] == "IMPLEMENTED")
        stub = sum(1 for r in sub if r[9] == "STUB")
        miss = sum(1 for r in sub if r[9] == "MISSING")
        print(f"  {mod:25s}  total={len(sub):4d}  IMPL={impl:4d}  STUB={stub:4d}  MISSING={miss:4d}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
