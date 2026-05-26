# Gocene

Gocene is an idiomatic Go port of [Apache Lucene 10.4.0](https://github.com/apache/lucene), targeting byte-level wire compatibility with the original JVM implementation.

**Status:** alpha — APIs are unstable and subject to change.
**Lucene reference:** release tag `releases/lucene/10.4.0` (commit `9983b7c`)

---

## Table of contents

- [Compatibility guarantee](#compatibility-guarantee)
- [Package inventory](#package-inventory)
- [Running tests](#running-tests)
- [Known limitations](#known-limitations)

---

## Compatibility guarantee

Gocene targets **byte-identical** wire compatibility with Apache Lucene 10.4.0:
every artefact Gocene writes must be readable by Lucene 10.4.0 unchanged, and
every artefact Lucene 10.4.0 writes must be readable by Gocene without
reinterpretation. The full mandate is in [`CLAUDE.md`](CLAUDE.md) (section
*Binary Compatibility Mandate*); the contributor-facing guide on how to verify
locally is in [`CONTRIBUTING.md`](CONTRIBUTING.md) (section *Binary
compatibility (mandatory)*). The compatibility suite has two layers: a Java
fixture harness under `tools/lucene-fixtures/` that drives Lucene 10.4.0
directly, and a Go test layer under `internal/compat/` (per-package
round-trips behind the `compat` build tag, plus end-to-end combined scenarios
gated by `GOCENE_COMPAT_HARNESS=1`). Every pull request runs both layers as
the required `compat` CI job.

---

## Package inventory

| Package | Lucene module | Notes |
|---|---|---|
| `analysis` | `lucene/analysis` | Core tokenizer pipeline |
| `analysis/hunspell` | `lucene/analysis` | Hunspell spell checker |
| `analysis/icu` | `lucene/analysis` | ICU segmentation (no .brk dicts) |
| `analysis/ko` | `lucene/analysis` | Korean (Nori) analyzer |
| `analysis/kuromoji` | `lucene/analysis` | Japanese (Kuromoji) analyzer |
| `analysis/snowball` | `lucene/analysis` | Snowball stemmers |
| `analysis/synonym` | `lucene/analysis` | Synonym filter |
| `analysis/util` | `lucene/analysis` | Shared utilities |
| `backward_codecs` | `lucene/backward-codecs` | Read support for Lucene 4.0–10.3 indices |
| `bufferpool` | `lucene/core` | Reusable byte buffer pool |
| `classification` | `lucene/classification` | Document classifiers |
| `codecs` | `lucene/core` | ForUtil, PForUtil, BKD, HNSW, BlockTree, Compressing, Lucene90–104 |
| `collation` | `lucene/analysis` | Collation-based tokenizer |
| `document` | `lucene/core` | Field and document types |
| `expressions` | `lucene/expressions` | Scripted numeric field expressions |
| `facets` | `lucene/facet` | Faceted search (taxonomy, sorted-set, range) |
| `geo` | `lucene/core` | Geospatial primitives |
| `grouping` | `lucene/grouping` | Result grouping |
| `highlight` | `lucene/highlighter` | Unified and vector highlighters |
| `index` | `lucene/core` | Core indexing infrastructure |
| `join` | `lucene/join` | Parent/child join queries |
| `memory` | `lucene/memory` | In-memory index |
| `misc` | `lucene/misc` | Miscellaneous utilities |
| `monitor` | `lucene/monitor` | Persistent query monitor |
| `payloads` | `lucene/analysis` | Payload-bearing token filters |
| `queries` | `lucene/queries` | Extended query types (function, spans, intervals) |
| `queryparser` | `lucene/queryparser` | Classic, flexible, simple, surround, XML parsers |
| `replicator` | `lucene/replicator` | Index replication |
| `replicator/nrt` | `lucene/replicator` | Near-real-time replication |
| `sandbox` | `lucene/sandbox` | Experimental features |
| `search` | `lucene/core` | Query execution, BM25, similarities, KNN |
| `search/knn` | `lucene/core` | K-nearest-neighbour search strategies |
| `snowball` | `lucene/analysis` | Generated Snowball stemmer code |
| `spatial` | `lucene/spatial-extras` | Spatial (spatial4j-compatible) queries |
| `spatial3d` | `lucene/spatial3d` | 3D spatial geometry |
| `store` | `lucene/core` | Directory and I/O abstractions |
| `suggest` | `lucene/suggest` | Auto-suggest (FST, analysing, spell) |
| `util` | `lucene/core` | Automaton, BKD, compress, FST, HNSW, packed, quantization |
| `internal/hppc` | `lucene/core` | High-performance primitive collections (internal) |

---

## Running tests

```bash
# Run all tests (no -race; see Known limitations)
go test ./... -timeout 300s

# Run tests for a specific package
go test ./codecs/... -timeout 120s

# Run benchmarks
go test -bench=. -benchmem ./codecs/... ./search/... ./index/...
```

Test results are affected by pre-existing known failures in some packages (see
[Known limitations](#known-limitations)). The baseline for each package is
documented in the individual sprint summaries.

---

## Known limitations

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
  incompatible with the Linux/ARM64 RPi kernel; tests run without `-race`.

- **API stability.** Pre-release. The v1.0 API review is in progress;
  exported symbols may still change before the stable tag is cut.

---

## License

Apache License 2.0. See `LICENSE` for the full text and `NOTICE` for
attribution information. Gocene is derived from Apache Lucene 10.4.0
(releases/lucene/10.4.0, commit 9983b7c), which is also licensed under
Apache 2.0.
