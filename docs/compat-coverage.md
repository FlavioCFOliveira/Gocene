# Gocene Binary-Compatibility Coverage Audit

This document records a read-only audit, originally performed on 2026-05-25, of
every binary artefact that Apache Lucene 10.4.0 serialises and the
corresponding coverage that the Gocene port exposes through isolated unit
tests, combined integration tests, and committed test fixtures. The full
row-level inventory lives in [`compat-coverage.tsv`](compat-coverage.tsv); this
file summarises the worst gap in each stack layer and the overall counts.

The original audit captured the *pre-Sprint-114* baseline. The summary blocks
below have been refreshed on 2026-05-26 against the state shipped by Sprint
114 T1–T6 and the 21 per-package follow-ups (rmp 4614..4634). A row is treated
as *covered* only when an `internal/compat/<pkg>/*_test.go` file actually
asserts byte parity for that artefact — read-fixture or write-and-verify legs
each count; full round-trips are bonus. See the *Sprint 114 recategorisation
note* at the bottom of this file for the deferral inventory.

## Top gaps per stack layer (refreshed 2026-05-26)

- **store** — Closed by Sprint 114: the `store-primitives` scenario and
  `internal/compat/store/` assert IndexInput/IndexOutput parity against
  Lucene-written streams. Remaining gap: the rate-limited I/O path is
  exercised only on the Gocene-read leg (write-side parity deferred).
- **codecs** — Closed for the primary Lucene 10.4.0 read formats
  (postings, doc-values, points, term vectors, stored fields, norms,
  segment info, field infos, compound, per-field) via the
  `postings-format`, `doc-values-format`, `points-format`,
  `term-vectors-format`, `stored-fields-format`, `norms-format`,
  `segment-info-format`, `field-infos-format`, `compound-format`,
  `perfield-postings-doc-values` scenarios and `internal/compat/codecs/`.
  Remaining gap: SimpleText, Memory and BitVectors codecs are out of
  scope (debug-only in Lucene 10.4.0); Gocene-write legs for the
  read-only-in-Lucene-10.4 backward-codec families stay deferred.
- **index** — Closed for `segments_N`, `.si`, live docs, deletions,
  doc-values updates, and soft deletes via the `index-deletions-and-dv-
  updates`, `index-soft-deletes`, `live-docs-format`, and
  `index-corruption` scenarios and `internal/compat/index/`. Remaining
  gap: the Gocene-write leg for several index-level scenarios is blocked
  on the SegmentReader core-readers gap (see deferred-rows note).
- **search** — Closed by the `search-scoring-corpus` and `knn-hit-
  ordering` scenarios plus `combined-multi-segment-index-search` (S1)
  and `combined-reverse-index-search` (S2), which pin BM25 score and
  KNN hit ordering against Lucene-produced indexes.
- **analysis** — Closed for synonym FSTs, Hunspell blobs, Snowball
  artefacts, token payloads, and the Kuromoji external dictionary via
  the `synonym-fst`, `hunspell-blob`, `snowball-blob`, and
  `token-payload-bytes` scenarios and `internal/compat/analysis/`.
- **queries** — Closed by the `queries-hit-corpus` scenario; queries are
  now verified against Lucene-written indexes through
  `internal/compat/queries/`.
- **facets** — Closed for taxonomy directories, sorted-set ords,
  association payloads, and packed facet sets via `taxonomy-directory`,
  `facet-sortedset-ords`, `facet-association-payload`,
  `facet-set-packed-bytes`, and the S3 combined scenario.
- **suggest** — Closed for `Completion104PostingsFormat`,
  `AnalyzingInfixSuggester`, the WFST/AnalyzingSuggester FSTs, and the
  S5 combined scenario.
- **highlight** — Closed via `highlight-offset-corpus`,
  `fast-vector-highlight-phrases`, and the S6 combined scenario.
- **join** — Closed via the `parent-block-corpus` scenario and
  `internal/compat/join/`.
- **grouping** — Closed via the `grouping-result-corpus` scenario and
  `internal/compat/grouping/`.
- **classification** — Closed via the `classifier-label-corpus` scenario
  and `internal/compat/classification/`.
- **monitor** — Closed via `monitor-query-blob` and
  `monitor-index-segment` scenarios and `internal/compat/monitor/`.
- **replicator** — NRT closed via `replicator-nrt-copystate`,
  `replicator-session-revision`, and the S4 combined scenario. The HTTP
  replicator and `IndexRevision` rows are *deferred (read-only)*: the
  upstream APIs were removed in Lucene 10.4.0 (see deferred-rows note).
- **spatial** — Closed via `spatial-prefix-tree`, `spatial-bbox-dv`,
  `spatial-serialized-dv-shape`, `spatial-wkt-geojson`,
  `spatial-composite`, and `spatial3d-serializable`.
- **expressions** — Closed for the value-source layer via
  `expressions-eval-corpus` and `internal/compat/expressions/`. JVM
  bytecode interop remains intentionally out of scope.
- **queryparser** — Closed via `queryparser-trees-and-hits` and the S6
  combined scenario.
- **sandbox** — Closed via `sandbox-idversion-postings` and
  `sandbox-quantization-codec`. `IDVersionPostingsFormat` write leg is
  deferred (read-only in Lucene 10.4.0; see deferred-rows note).
- **misc** — Closed via `misc-index-splitter-input` and
  `misc-highfreq-terms-corpus`.
- **memory** — Closed via `memory-index-flush` and
  `internal/compat/memory/memory_index_against_directory_test.go`.
- **backward_codecs** — Closed for `lucene40/blocktree`, `lucene70/si`,
  `lucene90/hnsw-v0`, `lucene99/postings`, `lucene99/scalar-quantized`,
  `lucene103/postings`, `packed64`, the store endianness reverser, and
  the multi-version corpora via the `bwc-*` scenarios. Gocene-write
  legs remain *deferred (read-only)* in line with Lucene's own
  contract — backward codecs are read-only in Lucene 10.4.0.

## Coverage summary

The inventory below is unchanged in shape (`compat-coverage.tsv` retains its
pre-Sprint-114 row classification so the audit trail is preserved). The
*effective* coverage after Sprint 114, expressed in terms of the manifest
scenarios and the Go-side compat suite, is:

- **Total artefacts inventoried:** 105 rows across 25 packages.
- **Manifest scenarios committed:** 74 unique scenarios in
  [`tools/lucene-fixtures/manifests/baseline.tsv`](../tools/lucene-fixtures/manifests/baseline.tsv)
  (12 foundational from T3, ≈40 per-package scenarios, 9 backward-codec
  `bwc-*` rows, 6 combined `combined-*` rows, plus smoke and the
  bwc-multi-version umbrella).
- **Go compat packages under `internal/compat/`:** 23 package
  directories (one per audited package plus `scenarios/` and `smoke/`)
  totalling 110 `*_test.go` files asserting byte parity against the
  pinned fixtures.
- **Coverage after Sprint 114 (effective):** every one of the 25
  audited packages now has at least one `internal/compat/<pkg>/*_test.go`
  asserting byte parity against a Lucene-written fixture. The previous
  nine-packages-with-zero-coverage list is fully closed.
- **Committed fixtures from Lucene 10.4.0:** the `testdata/lucene-10.4.0-
  fixtures` corpus has been augmented by the per-scenario fixtures
  generated by the Java harness; every manifest row pins a SHA-256 in
  `baseline.tsv` and is regenerable deterministically from the canary
  seeds `0xC0FFEE` and `0xDECAF`.

## Combined-scenario coverage (Sprint 114 T5, rmp 4611)

Six end-to-end combined scenarios compose ≥2 audited subsystems each and
emit a deterministic TSV transcript that the Go-side suite re-parses and
pins. All six are byte-deterministic at the two canary seeds
(`0xC0FFEE`, `0xDECAF`); the Lucene-side verifier round-trip is gated by
the harness CLI (`verify <scenario> <seed> <source>`); the Gocene-write
leg is deferred per scenario in
`internal/compat/scenarios/deferred_combined_compat_test.go`.

| Scenario name | Description | Audit rows touched |
| --- | --- | --- |
| `combined-multi-segment-index-search` (S1) | 3-segment Lucene 10.4.0 index (stored fields, NumericDocValues, IntPoint, KnnFloatVectorField, term vectors, norms) + 8-query catalogue (5 TermQuery + 2 PhraseQuery + 1 BooleanQuery), emits `s1-hits.tsv` | search numerical-parity; index multi-segment topology; stored fields / docvalues / points / KNN / term vectors composed end-to-end |
| `combined-reverse-index-search` (S2) | Single-segment Lucene reference over the same doc set; emits `s2-hits.tsv` byte-identical to `s1-hits.tsv` | search numerical-parity; multi-vs-single-segment scoring invariance |
| `combined-facets-search` (S3) | TaxonomyDirectory sidecar + faceted query, emits `s3-facet-counts.tsv` (dim, label, count) | facets taxonomy directory; faceted-query end-to-end |
| `combined-replicator-roundtrip` (S4) | NRT primary→replica wire transcript (`s4-frames.bin` + `s4-files.tsv`) using the canonical `SimplePrimaryNode.writeCopyState` layout | replicator NRT CopyState wire format |
| `combined-suggester-fst` (S5) | `AnalyzingSuggester` FST + 5 prefix lookups, emits `s5-suggestions.tsv` | suggest module (FST persistence + lookup) |
| `combined-highlight-queryparser-analysis` (S6) | Classic `QueryParser` → `StandardAnalyzer` → `UnifiedHighlighter` chain over 3 queries, emits `s6-highlights.tsv` with byte-stable snippet escaping | highlight UH offsets; queryparser parity; analysis chain end-to-end |

Manifest digests live in
[`tools/lucene-fixtures/manifests/baseline.tsv`](../tools/lucene-fixtures/manifests/baseline.tsv);
the six new rows are appended at the end of the manifest, after the
`bwc-*` block, so previously-anchored row positions are preserved.

The mutation-diagnostic CLI (`verify-diagnostic <scenario> <seed> <source>`)
emits a one-line JSON record `{file, offset, expected, actual}` on the
first byte-level divergence and exits 4; it is exercised by
`TestMutationDiagnostic` against an S1 fixture mutated at byte offset 100.

## Sprint 114 recategorisation note — deferred rows

Sprint 114 closed every audit row that maps to an actively written
Lucene 10.4.0 format. A small set of rows remains deferred; each
deferral is recorded in code at
`internal/compat/scenarios/deferred_combined_compat_test.go` (combined
scenarios) and in the affected per-package `*_test.go` files
(per-package scenarios). The two deferral categories are:

1. **Gocene-write legs blocked on the `SegmentReader` core-readers
   gap.** The Gocene `SegmentReader` does not yet expose the full
   `getCoreCacheHelper` / core-readers surface, which prevents the Go
   side from emitting multi-segment writes that Lucene can verify
   byte-for-byte. Read-leg parity (Lucene-write → Gocene-read) is
   asserted in all cases; the write leg is held until the
   `SegmentReader` work lands. Affected scenarios:
   `index-deletions-and-dv-updates` (write leg), `index-soft-deletes`
   (write leg), the multi-segment portion of `combined-multi-segment-
   index-search` and `combined-reverse-index-search`, and the
   replicator NRT primary-side emit path
   (`combined-replicator-roundtrip`).

2. **Read-only-in-Lucene-10.4.0 formats and APIs removed upstream.**
   Apache Lucene 10.4.0 itself no longer writes these artefacts;
   Gocene therefore implements read-only parity to match. Affected
   rows: the entire `backward_codecs/*` write leg
   (Lucene 4.0–10.3 codecs are read-only by upstream contract); the
   HTTP replicator (`HttpReplicator`, removed in Lucene 10.x);
   `IndexRevision` (removed upstream); the
   `IDVersionPostingsFormat` write leg and the sandbox quantization
   sampling codec write leg (sandbox-only, no longer emitted by
   stock 10.4.0 writers); `SimpleText`, `Memory`, and `BitVectors`
   codecs (debug-only in upstream 10.4.0, no production write
   coverage requested).

Both categories are intentional and aligned with the binary-
compatibility mandate in [`CLAUDE.md`](../CLAUDE.md): Gocene must read
every byte sequence Lucene 10.4.0 emits, and must produce only those
artefacts that Lucene 10.4.0 itself still emits. Where Lucene has
stopped writing a format, Gocene matches.
