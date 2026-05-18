#!/usr/bin/env python3
"""
Cross-link each Java production class with its Java test class (Test<X>.java or
<X>Test.java in the same package), and emit a CSV of pairs.

Inputs:
  /tmp/gocene-inventory/inventory.tsv

Output:
  /tmp/gocene-inventory/pairs.tsv  (module, classname, prod_status,
                                    test_classname, test_status_in_gocene,
                                    test_java_path)
"""

import sys
from pathlib import Path

INV = Path("/tmp/gocene-inventory/inventory.tsv")
OUT = Path("/tmp/gocene-inventory/pairs.tsv")


def main() -> int:
    rows = [line.rstrip("\n").split("\t") for line in INV.read_text().splitlines()]
    header = rows[0]
    data = rows[1:]
    idx = {h: i for i, h in enumerate(header)}

    # Index test rows by (package, classname)
    tests_by_key: dict[tuple[str, str], list[list[str]]] = {}
    for r in data:
        if r[idx["kind"]] != "test":
            continue
        key = (r[idx["package"]], r[idx["classname"]])
        tests_by_key.setdefault(key, []).append(r)

    out = ["\t".join([
        "module", "package", "prod_classname", "prod_status",
        "test_classname", "test_status", "test_java_path", "prod_java_path",
    ])]

    no_test = 0
    matched = 0
    for r in data:
        if r[idx["kind"]] != "prod":
            continue
        pkg = r[idx["package"]]
        cls = r[idx["classname"]]
        prod_status = r[idx["status"]]
        prod_path = r[idx["java_path"]]

        # Candidate test class names
        candidates = [f"Test{cls}", f"{cls}Test", f"{cls}Tests", f"Test{cls}s"]
        chosen = None
        for cand in candidates:
            if (pkg, cand) in tests_by_key:
                chosen = tests_by_key[(pkg, cand)][0]
                break
        if chosen is None:
            # Try parent package (analysis tests sometimes live one level up)
            parent_pkg = ".".join(pkg.split(".")[:-1])
            for cand in candidates:
                if (parent_pkg, cand) in tests_by_key:
                    chosen = tests_by_key[(parent_pkg, cand)][0]
                    break
        if chosen:
            matched += 1
            out.append("\t".join([
                r[idx["module"]], pkg, cls, prod_status,
                chosen[idx["classname"]], chosen[idx["status"]],
                chosen[idx["java_path"]], prod_path,
            ]))
        else:
            no_test += 1
            out.append("\t".join([
                r[idx["module"]], pkg, cls, prod_status,
                "", "", "", prod_path,
            ]))

    OUT.write_text("\n".join(out) + "\n")
    print(f"Pairs with test: {matched}")
    print(f"Prod without test: {no_test}")

    # Distribution among prod statuses
    statuses: dict[str, dict[str, int]] = {}
    for line in out[1:]:
        parts = line.split("\t")
        prod_status = parts[3]
        has_test = bool(parts[4])
        d = statuses.setdefault(prod_status, {"with_test": 0, "without_test": 0})
        d["with_test" if has_test else "without_test"] += 1
    for st in ("IMPLEMENTED", "STUB", "MISSING"):
        if st in statuses:
            d = statuses[st]
            print(f"  prod {st:11s}  with_test={d['with_test']:4d}  without_test={d['without_test']:4d}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
