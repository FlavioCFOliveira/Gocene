# Knowledge Model — Gocene

Canonical description of the Label-Property Graph held in the `gocene` roadmap and
reached exclusively through `rmp graph`. **This file must always conform to the live
graph**; it is regenerated from measurements, never from memory.

- **Roadmap:** `gocene` (resolved from `./.rmp`, key `roadmap`).
- **Engine:** `rmp` 1.17.0 — the graph is a **server/client pair**. `rmp graph serve`
  is the only process that opens the store; `rmp graph client` is the only way to run a
  statement, and it needs a live server for **every** one. The old subcommands
  (`create` / `query` / `update` / `delete` / `search`) were removed and now exit **127**.
- **Socket:** `~/.roadmaps/gocene/graph.sock` (the derived default — `.rmp` declares no
  `socket` key, so both ends must omit `--socket`).
- **Lucene reference:** Apache Lucene 10.5.0 tree at `/tmp/lucene` (tag `releases/lucene/10.5.0`).
- **Live size:** **0 nodes, 0 edges, 0 labels, 0 predicates, 0 property keys.**

> **The graph is empty, and this model is empty because the graph is empty.**
> There is no label dictionary, no predicate dictionary, no constraint and no index,
> because the store holds nothing to describe. Nothing below is a target or a plan.
> Populating the graph requires the **bootstrap** procedure in § 6, which begins with
> a survey and **stops for the user's approval** before anything is materialised.

## 1. State

Verified against the live server on 2026-09-08:

```cypher
MATCH (n) RETURN count(n)          -- 0
MATCH ()-[r]->() RETURN count(r)   -- 0
CALL db.labels()                   -- 0 rows
CALL db.relationshipTypes()        -- 0 rows
CALL db.propertyKeys()             -- 0 rows
SHOW CONSTRAINTS                   -- 0 rows
SHOW INDEXES                       -- 0 rows
```

The store at `~/.roadmaps/gocene/graph/` was discarded and recreated on 2026-09-08,
so the graph starts from nothing: no data, no schema, and no residual label,
relationship-type or property-key registrations. Nothing of any earlier graph was
retained, and no copy of it exists.

## 2. Label dictionary

**Empty.** No label is defined and no node exists. `CALL db.labels()` returns 0 rows.

## 3. Predicate dictionary

**Empty.** No predicate is defined and no edge exists. `CALL db.relationshipTypes()`
returns 0 rows.

## 4. Constraints

**None declared, none enforced.** `SHOW CONSTRAINTS` returns 0 rows. There is no
violated target to record either: a constraint constrains a label, and no label
exists.

The engine supports exactly two constraint kinds, both on a single node property,
and both become available once labels exist:

| Kind | DDL | Reported type |
|---|---|---|
| Uniqueness | `CREATE CONSTRAINT <n> FOR (x:L) REQUIRE x.p IS UNIQUE` | `UNIQUE` |
| Presence | `CREATE CONSTRAINT <n> FOR (x:L) REQUIRE x.p IS NOT NULL` | `NOT_NULL` |

`UNIQUE` is genuinely enforced — a violating write is rejected — which makes it the
strongest available defence against the pattern-`MERGE` duplication trap. Composite
`NODE KEY`, `ASSERT exists(...)` and type constraints (`IS :: STRING`) are
unsupported and fail with an opaque internal error.

## 5. Recommended indexes

**None.** An index is recommended from the equality lookups actually issued, ranked
by measured label size × selectivity, and proven with `EXPLAIN`. With no data there
is nothing to measure and no plan to compare, so any recommendation here would be
invention.

The engine's index is **single-property, node-only and hash — therefore
equality-only**. A range predicate (`>`, `<`) ignores it and falls back to a label
scan. `TEXT` / `RANGE` / `POINT` / `FULLTEXT` / `LOOKUP`, composite indexes and
relationship-property indexes are all unsupported.

## 6. Bootstrap — the required next step

Population must follow the **bootstrap** procedure of the `knowledge-authority`
skill, in this order:

1. **Survey exhaustively.** Walk the Gocene working tree and the Apache Lucene
   10.5.0 reference tree at `/tmp/lucene` until both structures are understood as a
   whole — from measurement, not from memory.
2. **Propose a shape.** Draft the labels, predicates and properties that fit what
   the survey found, each with its identity key and that key's type, plus the
   `UNIQUE` constraints those keys imply and the indexes justified by measurement.
   The shape must satisfy the four fidelity requirements of `CLAUDE.md` § 5.1:
   faithful representation of Apache Lucene 10.5.0, faithful representation of
   Gocene, a `PORTED_TO` relation authoritative at every granularity at which
   porting actually happens, and whatever further structure the port depends on.
3. **Stop for approval — mandatory.** Present the proposed model and wait. Do not
   materialise the schema and do not populate the graph until the user approves.
4. **Only then** write this file, create the constraints and indexes, and populate.

Every node and edge carries `gitCommit` (the full commit hash when the element was
last confirmed) and `gitDate` (that commit's ISO date); stamp both on every write.

Until step 4 completes, every factual question about the codebase falls back to
reading files, and each such read owes the graph an update once the graph exists.
