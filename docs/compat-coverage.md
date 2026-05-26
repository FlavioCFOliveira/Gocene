# Gocene Binary-Compatibility Coverage Audit

This document records a read-only audit performed on 2026-05-25 of every binary
artefact that Apache Lucene 10.4.0 serialises and the corresponding coverage
that the Gocene port exposes through isolated unit tests, combined integration
tests, and committed test fixtures. The full row-level inventory lives in
[`compat-coverage.tsv`](compat-coverage.tsv); this file summarises the worst
gap in each stack layer and the overall counts.

## Top gaps per stack layer

- **store** — IndexInput/IndexOutput primitives lack any corpus of raw
  Lucene-emitted streams; parity is asserted only against internally produced
  bytes.
- **codecs** — The Lucene104 postings format (`.doc`/`.pos`/`.pay`/`.tim`/
  `.tip`/`.tmd`) is not read back from the committed `.cfs` fixture; SimpleText,
  Memory, BitVectors and Lucene104 scalar-quantized HNSW codecs have no tests
  at all.
- **index** — Live-docs (`.liv`) and incremental doc-values updates have no
  Lucene-written fixtures; cross-engine coverage is concentrated on
  `segments_N` and `.si` only.
- **search** — No numerical-parity corpus exists to verify that Gocene scores
  match Lucene scores; KNN search over the fixture HNSW bytes is not exercised
  end-to-end.
- **analysis** — Synonym FST blobs, compiled Hunspell dictionaries and Word2Vec
  archives are never round-tripped against Lucene-produced binaries.
- **queries** — No binary artefacts; coverage gap is the absence of integration
  tests over Lucene-written indexes (only Gocene-write paths are verified).
- **facets** — Taxonomy directory index files have no Lucene fixture;
  association payload byte layout is asserted by self-roundtrip only.
- **suggest** — `Completion104PostingsFormat` and `AnalyzingInfixSuggester`
  sidecar indexes have zero coverage (no tests, no fixtures).
- **highlight** — Term-vector consumption is not verified against
  Lucene-written term-vector files.
- **join** — Runtime-only module; no integration test reads a Lucene-written
  parent-block-join segment.
- **grouping** — No tests beyond block-grouping smoke; no Lucene-written
  grouping corpus exercised.
- **classification** — Runtime-only module; lacks an integration test that
  classifies a Lucene-written corpus.
- **monitor** — `MonitorQuerySerializer` wire format and the persisted monitor
  query index have no round-trip against Lucene-generated blobs.
- **replicator** — NRT `CopyState`/`FileMetaData` and HTTP replicator protocol
  have no captured Java-peer frames used as fixtures.
- **spatial** — `SerializedDVStrategy` shape blobs are not decoded from
  Lucene-produced bytes; `composite` strategy has no tests at all.
- **expressions** — No persisted artefact in the Java sense (JVM bytecode is
  not portable to Go), but interop with Lucene-compiled expressions is
  untested.
- **queryparser** — No binary artefacts; query-grammar parity is asserted only
  through Gocene-internal cases.
- **sandbox** — `IDVersionPostingsFormat` and quantization sampling codec are
  pure ports without tests, fixtures, or writer parity.
- **misc** — `IndexSplitter` / `IndexMergeTool` have never been run against a
  Lucene-written input; `SweetSpotSimilarity` and `HighFreqTerms` have no
  tests.
- **memory** — `MemoryIndex` is transient and not persisted, but no byte-level
  parity test compares its internal layout to Lucene's `MemoryIndex` during a
  merge scenario.
- **backward_codecs** — `backward_index` houses skeleton multi-version
  compatibility tests but no actual Lucene-written index ZIPs are committed,
  so older codec coverage remains aspirational.

## Coverage summary

- **Total artefacts inventoried:** 105 rows across 25 packages.
- **Isolated tests:** 63 `yes`, 28 `partial`, 14 `no`.
- **Combined / integration tests:** 30 `yes`, 26 `partial`, 49 `no`
  (Sprint 114 T5 flipped the UnifiedHighlighter offset retrieval row from
  `no` → `partial` via the S6 combined-scenario coverage).
- **Committed fixtures from Lucene 10.4.0:** 7 `yes`, 98 `no` (the seven `yes`
  rows all share the single `testdata/lucene-10.4.0-fixtures` corpus, which
  covers `segments_1`, `_0.si`, `_0.cfs`, `_0.cfe` and the artefacts embedded
  in the compound file).

Nine packages have zero artefacts with an isolated `yes` test:
`classification`, `expressions`, `grouping`, `highlight`, `join`, `monitor`,
`queries`, `search`, and `spatial3d`. These are mostly runtime-only modules,
but their lack of fixture-based coverage means we cannot prove byte-level
behavioural parity for any input they consume.

The single largest leverage point is the fixtures corpus: extending
`testdata/lucene-10.4.0-fixtures` to include taxonomy directories, completion
postings, replicator wire frames, synonym/Hunspell compiled blobs, and a
multi-version backward-compatibility ZIP would convert a large fraction of the
`partial` rows above into `yes`.

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
