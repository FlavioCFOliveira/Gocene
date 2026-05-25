# Changelog

All notable changes to Gocene are documented in this file.

## [Unreleased] — v1.0 preparation

### Additions

- **Lucene 10.4.0 postings round-trip** (`codecs/lucene104`): full
  `Lucene104PostingsFormat` writer + reader with `BlockPostingsEnum`,
  `ImpactsEnum` for BMW/MaxScore scoring, and a block-tree wire-up via
  `Lucene103SegmentTermsEnum`. All format-level tests pass without `t.Skip`.
- **Lucene99HnswVectorsReader**: replaced 116-line stub with complete
  `.vem`/`.vex` reader. Fixed the pre-existing bug where `finish()` in
  `lucene99HnswFieldWriter` never initialised the graph builder — HNSW graphs
  are now built correctly via in-memory scorer suppliers.
- **AddIndexesFromReader** no longer increments an atomic counter; it now
  registers `pendingImportedSegments` matching the `AddIndexes` pipeline.
- **backward_codecs/lucene40/blocktree**: fixed test build failure caused by
  a stale `(any, error)` return type in `noopPostingsReader.Impacts`; all 26
  backward-codec packages now build and pass.
- **Package documentation**: added `codecs/doc.go` with a full godoc
  describing the SPI-based codec registry and nine format types.
- **Runnable examples**: `analysis/example_test.go`,
  `index/example_test.go`, `search/example_test.go` — canonical usage flows
  enforced by the Go test harness.

### Removed

- Stale README known-limitation bullet "Merge infrastructure incomplete"
  (the `AddIndexes` variants it described are now working and tested).

---

## [v0.1.0-alpha] — 2026-05-24

### Summary

First tagged release of Gocene — an idiomatic Go port of Apache Lucene targeting
byte-level wire compatibility with the JVM implementation.

**Lucene source reference:** Apache Lucene 10.4.0
(tag `releases/lucene/10.4.0`, commit `9983b7c`)

---

### Ported packages

| Go package | Lucene source module | Status |
|---|---|---|
| `analysis` | `lucene/analysis` | Partial — core pipeline complete; Snowball/Hunspell/morph deferred |
| `analysis/br` | `lucene/analysis` | Ported |
| `analysis/charfilter` | `lucene/analysis` | Ported |
| `analysis/cjk` | `lucene/analysis` | Ported |
| `analysis/ckb` | `lucene/analysis` | Ported |
| `analysis/classic` | `lucene/analysis` | Ported |
| `analysis/commongrams` | `lucene/analysis` | Ported |
| `analysis/compound` | `lucene/analysis` | Ported |
| `analysis/core` | `lucene/analysis` | Ported |
| `analysis/egothor` | `lucene/analysis` | Ported |
| `analysis/email` | `lucene/analysis` | Ported |
| `analysis/en` | `lucene/analysis` | Ported |
| `analysis/es` | `lucene/analysis` | Ported |
| `analysis/hunspell` | `lucene/analysis` | Ported |
| `analysis/icu` | `lucene/analysis` | Ported (no ICU4J .brk dictionaries) |
| `analysis/ko` | `lucene/analysis` | Ported |
| `analysis/kuromoji` | `lucene/analysis` | Ported |
| `analysis/miscellaneous` | `lucene/analysis` | Ported |
| `analysis/morfologik` | `lucene/analysis` | Ported |
| `analysis/morph` | `lucene/analysis` | Ported |
| `analysis/pattern` | `lucene/analysis` | Ported |
| `analysis/phonetic` | `lucene/analysis` | Ported |
| `analysis/query` | `lucene/analysis` | Ported |
| `analysis/shingle` | `lucene/analysis` | Ported |
| `analysis/smartcn` | `lucene/analysis` | Ported |
| `analysis/snowball` | `lucene/analysis` | Ported |
| `analysis/stempel` | `lucene/analysis` | Ported |
| `analysis/synonym` | `lucene/analysis` | Ported |
| `analysis/util` | `lucene/analysis` | Ported |
| `analysis/wikipedia` | `lucene/analysis` | Ported |
| `backward_codecs` | `lucene/backward-codecs` | Ported (Lucene 4.0–10.3) |
| `bufferpool` | `lucene/core` | Ported |
| `classification` | `lucene/classification` | Ported |
| `codecs` | `lucene/core` | Ported (ForUtil, PForUtil, BKD, HNSW, BlockTree, Compressing, Lucene90–104) |
| `collation` | `lucene/analysis` | Ported |
| `document` | `lucene/core` | Ported |
| `expressions` | `lucene/expressions` | Ported |
| `facets` | `lucene/facet` | Ported |
| `geo` | `lucene/core` | Ported |
| `grouping` | `lucene/grouping` | Ported |
| `highlight` | `lucene/highlighter` | Ported |
| `index` | `lucene/core` | Ported (core indexing infrastructure) |
| `internal/hppc` | `lucene/core` | Ported (internal use only) |
| `join` | `lucene/join` | Ported |
| `memory` | `lucene/memory` | Ported |
| `misc` | `lucene/misc` | Ported |
| `monitor` | `lucene/monitor` | Ported |
| `payloads` | `lucene/analysis` | Ported |
| `queries` | `lucene/queries` | Partial — function/docvalues/valuesource foundation ported |
| `queryparser` | `lucene/queryparser` | Ported |
| `replicator` | `lucene/replicator` | Ported |
| `replicator/nrt` | `lucene/replicator` | Ported |
| `sandbox` | `lucene/sandbox` | Ported |
| `search` | `lucene/core` | Ported (core search, BM25, similarities, KNN) |
| `search/comparators` | `lucene/core` | Ported |
| `search/knn` | `lucene/core` | Ported |
| `snowball` | `lucene/analysis` | Ported |
| `spatial` | `lucene/spatial-extras` | Ported |
| `spatial3d` | `lucene/spatial3d` | Ported |
| `store` | `lucene/core` | Ported (MMap, NIO, ByteBuffers, NRT cache) |
| `suggest` | `lucene/suggest` | Ported |
| `util` | `lucene/core` | Ported (automaton, BKD, compress, FST, HNSW, packed, quantization) |

Internal-only packages (`internal/hppc`, `internal/tests`,
`internal/vectorization`) are not part of the public API. They are used
exclusively by other packages within this module.

---

### Known limitations

- **SegmentTermsEnum / IntersectTermsEnum deferred.** Full block-tree
  iterator wiring is tracked in backlog tasks 2691 and 2692.
- **PostingsFormat SPI wiring deferred.** Codec service-provider loading is
  a no-op; formats must be registered programmatically via
  `PostingsFormatByName`, `DocValuesFormatByName`, and
  `KnnVectorsFormatByName`.
- **ICU4J binary dictionaries absent.** `analysis/icu` implements the ICU
  Unicode segmentation pipeline but does not ship `.brk` dictionary files;
  CJK/Thai/Myanmar dictionary-based breaking falls back to rules-based mode.
- **Race detector unavailable on this host.** The TSAN VMA layout is
  incompatible with the Linux/ARM64 RPi kernel; tests are run without `-race`.
- **API stability.** This is an alpha release. All exported symbols are subject
  to change without notice until `v1.0.0`.
