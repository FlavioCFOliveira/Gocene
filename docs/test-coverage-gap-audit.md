# Gocene Test Coverage Gap Audit

**Generated:** 2026-06-11
**Scope:** All 33+ Gocene packages, 12,616 test functions, 660 deferred tests
**Purpose:** Identify every missing test needed to achieve 100% functional coverage and byte-for-byte binary compatibility certification against Apache Lucene 10.4.0

---

## Executive Summary

Gocene currently has **12,616 active test functions** across **1,944 test files** in 33 top-level packages. The compat test suite under `internal/compat/` adds **110 test files** across **23 package directories**, covering read-path byte parity for every audited binary artefact.

**660 tests are deferred** (using `t.Fatal` with descriptive blocker reasons, per the No-Skip policy). These deferrals represent concrete gaps that must be resolved to achieve production-grade certification.

### Top-Level Numbers

| Metric | Count |
|--------|-------|
| Total test files (all packages) | 1,944 |
| Total `Test*` functions | 12,616 |
| Active (non-deferred) test functions | ~11,956 |
| Deferred test functions (t.Fatal) | 660 |
| Compat test files (`internal/compat/`) | 110 |
| Compat test functions | ~360 |
| Combined E2E scenarios | 6 (S1–S6) |
| Compat manifest scenarios | 74+ |
| Packages with zero compat tests | 12 |
| Untested source files (estimated) | 300+ across all packages |

---

## Systemic Blockers (Cross-Package Impact)

These infrastructure gaps block tests across multiple packages simultaneously.

### BLOCKER-1: SegmentReader Core Readers (rmp #4)

**Status:** `GetCoreReaders()` returns `nil`. The SegmentReader does not expose postings, doc-values, norms, points, or term-vectors producers through the codec layer.

**Impact:** Blocks **12+ compat scenarios** across 8 packages:
- `facets/` — All 4 binary round-trips (taxonomy, associations, ords, facet-sets)
- `highlight/` — 2 deferred: offset stores, FVH phrase-list parity
- `grouping/` — 3 deferred: FirstPass/TopGroups collectors, term-group selector
- `classification/` — 3 deferred: all 3 classifier parity tests
- `suggest/` — 1 deferred: AnalyzingInfixSuggester sidecar
- `queries/` — 3 deferred: CommonTermsQuery, FunctionScoreQuery, MoreLikeThis
- `join/` — 4 deferred: ToParent/ToChild hit/score parity, QueryBitSetProducer, CheckJoinIndex
- `index/` — ~46 deferred NRT read-path tests

**Priority:** **Critical** — The single most impactful blocker in the entire project.

### BLOCKER-2: NRT DirectoryReader.open(IndexWriter)

**Status:** `DirectoryReader.open(IndexWriter)` is not functional. NRT primitives exist (NRTDirectoryReader, NRTManager, etc.) but cannot be instantiated from a live IndexWriter.

**Impact:** Blocks ~46 tests in `index/` plus 12 `SearcherManager` tests in `search/`, plus ~30 NRT stress/concurrency tests.

**Priority:** **Critical** — Without NRT, real-time search is impossible.

### BLOCKER-3: DeleteDocuments (Term/Query) — No-Op Stubs

**Status:** `IndexWriter.DeleteDocuments(Term)` and `DeleteDocuments(Query)` are no-op stubs. The delete queue infrastructure exists but the actual document removal does not execute.

**Impact:** Blocks ~30 tests in `index/` (delete, soft-delete, try-delete, per-segment deletes, stress-deletes).

**Priority:** **Critical** — Document deletion is a fundamental index operation.

### BLOCKER-4: RandomIndexWriter / GeoTestUtil / CannedTokenStream

**Status:** Several Lucene test-utility classes are not ported to Gocene:
- `RandomIndexWriter` — Generates randomized index content for stress/coverage testing
- `GeoTestUtil` — Random geometry generators needed by ~51 spatial query tests
- `CannedTokenStream` — Fixed token injection for testing specific tokenization scenarios
- `MockDirectoryWrapper` — Fault injection (disk-full, IO exceptions, corruption)
- `MockGraphTokenFilter` / `MockTokenizer` — Graph token stream test infrastructure

**Impact:** Blocks ~120+ tests across `search/`, `index/`, `analysis/`, `codecs/`.

**Priority:** **High** — These are test infrastructure, not production features, but they gate the verification of many production features.

---

## Package-Level Gap Catalog (Priority-Ranked)

### TIER 1 — Critical (blocks core functionality or many downstream tests)

| # | Package | Gap Description | Deferred Tests | Binary Formats | Root Cause |
|---|---------|----------------|:---:|---|---|
| 1 | **index** | NRT DirectoryReader.open(IW) | ~46 | segments_N, .si | IW→DR wiring incomplete |
| 2 | **index** | DeleteDocuments (Term/Query) no-op | ~30 | .liv, .fnm | Delete pipeline stubs |
| 3 | **index** | Codec write path (IW.Commit → codec writers) | ~10 | All codec formats | DefaultCodecPersistence not invoked |
| 4 | **index** | SegmentReader core readers (postings, DV, norms, points) | ~10 | All segment formats | rmp #4 not resolved |
| 5 | **index** | ForceMerge ignores maxNumSegments / ForceMergeDeletes | ~15 | .cfs, segments_N | Merge policy integration incomplete |
| 6 | **codecs** | Lucene99PostingsFormat (writer + reader) | 3 | .doc, .pos, .pay, .tim, .tip | Stub only |
| 7 | **codecs** | PerFieldPostingsFormat2 end-to-end | 6 | Per-field .tim/.tip | IW integration missing |
| 8 | **search** | IndexSearcher full integration (IW→DR→search) | 12+ | All segment formats | Depends on BLOCKER-1, BLOCKER-2 |
| 9 | **search** | Span query test suite (8 query types) | 8 | .pos, .pay (postings) | Source exists, tests deferred |
| 10 | **search** | BooleanScorerSupplier / ReqExclBulkScorer | ~5 | N/A | Scorer wiring incomplete |
| 11 | **store** | MockDirectoryWrapper fault injection wiring | ~50+ (in index) | N/A | Mock not wired into IW tests |
| 12 | **facets** | ALL 4 binary round-trips (taxonomy, associations, ords, facet-sets) | 8 | .cfs, .dvd/.dvm | BLOCKER-1 |
| 13 | **facets** | Taxonomy facet count pipeline (IW + FacetsCollector + DirectoryTaxonomyWriter) | 17 | Taxonomy directory | E2E pipeline not wired |
| 14 | **document** | Point range queries NOT implemented | 8 | BKD .kdd/.kdi | search.NewPointRangeQuery not wired |
| 15 | **document** | No dedicated binary-compat test package | 4 audit rows | .kdd, .fdt, .dvd | internal/compat/document/ doesn't exist |
| 16 | **join** | PostingsEnum.Advance-after-positioning bug | 2 | .doc, .pos, .pay | Core codec defect (rmp #4763) |
| 17 | **join** | PrefixQuery nil Weight (parent filter) | 1+ | .tim, .tip | rmp #4760 |
| 18 | **util/bkd** | Randomized bulk testing (verify() helper, Java byte-exact comparison) | 22 | BKD .kdd/.kdi | Test infra missing |
| 19 | **util/packed** | PackedInts byte-compat expansion | 0 (partial gap) | PackedInts streams | Has basic test, needs fixture expansion |
| 20 | **backward_codecs** | Multi-version backwards compatibility test suite | 10 stub files | All legacy formats | No committed per-version ZIPs |

### TIER 2 — High (blocks major functional areas)

| # | Package | Gap Description | Deferred Tests | Binary Formats | Root Cause |
|---|---------|----------------|:---:|---|---|
| 21 | **index** | IndexSorting integration (AssertingNeedsIndexSortCodec) | 25 | .si, .fnm, .nvd, .dvd | Codec hook not implemented |
| 22 | **index** | Mixed DocValues updates (numeric + binary in same session) | 13 | .dvd/.dvm generational | DV update merge incomplete |
| 23 | **index** | TermVectorsReader (requires SegmentReader + IW term-vectors flush) | 13 | .tvx, .tvd, .tvf | BLOCKER-1 + CannedTokenStream |
| 24 | **index** | Soft deletes integration (SoftUpdateDocument) | 2 | .liv, .dvd/.dvm | SoftUpdateDocument not implemented |
| 25 | **index** | CheckIndex compatibility | 6 | All segment files | Cross-engine checkindex not implemented |
| 26 | **codecs** | Lucene99DocValuesFormat | 1 | .dvd, .dvm | Stub only |
| 27 | **codecs** | Lucene99StoredFieldsFormat | 1 | .fdt, .fdx | Stub only |
| 28 | **codecs** | DocValuesSkipper functional | 3 | .dvd, .dvm | Skipper not fully wired |
| 29 | **codecs** | Lucene104HnswScalarQuantizedVectorsFormat | 0 (stub) | .veq, .vemq, .vec | Reader/writer not ported |
| 30 | **search** | Spatial query factories (~51 calls across 18 files) | 51 | BKD, postings | RandomIndexWriter/GeoTestUtil |
| 31 | **search** | SimilarityScoring (boolean search) | 4 | Postings, norms | BooleanSearch not implemented |
| 32 | **search** | ScoringReproducibility (TermQuery, BooleanQuery, PhraseQuery) | 3 | Postings, norms | IndexSearcher not integrated |
| 33 | **search** | QueryExpansion | 3 | Postings | Rewrite not implemented |
| 34 | **search** | Untested source files (30+ files with no test) | 0 (no tests at all) | N/A | See Appendix A |
| 35 | **analysis** | SynonymMap binary writer | 1 round-trip | .fst (CodecUtil) | Binary writer not ported |
| 36 | **analysis** | IndexWriter payload path not wired | 1 round-trip | .pos, .pay | IW flush path incomplete |
| 37 | **analysis** | Mock graph token filter infrastructure | 6 | N/A | Test infra not ported |
| 38 | **document** | Per-field consistency validation (IW schema enforcement) | 3 | N/A | IW/DR pipeline not wired |
| 39 | **facets** | Facet integration (SetIndexPath, DrillDownQuery) | 2 | Taxonomy dir + DocValues | Missing API implementations |
| 40 | **spatial** | Spatial4j BinaryCodec decoder | 0 (no test) | Spatial4j BinaryCodec | JTS/WKB used instead |
| 41 | **spatial** | Prefix-tree postings reader (.tim/.tip) | 0 (no test) | Lucene104 postings | Reader not implemented |
| 42 | **spatial** | CompositeSpatialStrategy round-trip | 6 (compat) | Both BinaryCodec + postings | Depends on #40, #41 |
| 43 | **spatial** | BBoxStrategy doc-values reader | 4 (compat) | .dvd/.dvm | DocValues reader incomplete |
| 44 | **queryparser** | Query.String() byte-parity vs Lucene toString() | 6 (compat) | N/A | Not verified across parsers |
| 45 | **queryparser** | IndexSearcher execution of parsed queries | 6 (compat) | Postings, StoredFields | BLOCKER-1 |
| 46 | **queryparser** | Full StandardQueryParser feature set | 6 | N/A | MultiField/MultiPhrase/Points/Fuzzy |
| 47 | **sandbox** | IDVersion end-to-end binary parity (L→G→L) | 1 (compat) | .tiv, .tipv | PerField dispatch + round-trip |
| 48 | **sandbox** | IDVersion integration with IndexWriter | 1 (compat) | IDVersion postings | IW pipeline |
| 49 | **geo** | GeoTestUtil random geometry generator | 0 (no test) | N/A | Blocking 51 search/spatial tests |
| 50 | **geo** | LatLonPoint/LatLonShape/XYShape query factories | 0 (no test) | BKD .kdd/.kdi | Factory wiring incomplete |

### TIER 3 — Medium (important for certification but not blocking core)

| # | Package | Gap Description | Deferred |
|---|---------|----------------|:---:|
| 51 | **util/hnsw** | Concurrency tests (GOMAXPROCS >= 2) | 4 |
| 52 | **util/fst** | Byte-level golden fixture comparison vs Lucene | 0 (gap) |
| 53 | **util/compress** | LZ4 byte-level golden fixture vs Lucene | 0 (gap) |
| 54 | **util/quantization** | Byte-level fixture vs Lucene scalar quantizer | 0 (gap) |
| 55 | **util/automaton** | TestMinimize_Huge (short mode only) | 1 |
| 56 | **util/bkd** | Monster test (>4B points) | 1 |
| 57 | **util/fst** | Monster tests (>3 GiB FSTs) | 2 |
| 58 | **codecs** | MergedVectorValues (byte + float32) | 2 |
| 59 | **codecs** | CompressingStoredFields IW integration | 2 |
| 60 | **codecs** | SimpleText/UniformSplit/Bloom/BlockTerms cross-engine | 9 (compat) |
| 61 | **search** | Multi-segment search expansion | 0 (gap) |
| 62 | **search** | Concurrency/stress tests | 0 (gap) |
| 63 | **search** | DocValues-based search (IndexOrDocValuesQuery) | 1 |
| 64 | **analysis** | Standard tokenizer output not byte-verified | 0 (gap) |
| 65 | **analysis** | Language-analyzer binary compat (Hunspell, Kuromoji, Nori, Smartcn) | 6 (compat) |
| 66 | **facets** | Multi-index-field routing (RandomIndexWriter + MultiFacets) | 5 |
| 67 | **facets** | Parallel DrillSideways + SearcherTaxonomyManager | 5 |
| 68 | **document** | KNN monster tests | 2 |
| 69 | **spatial** | WKT/GeoJSON writer/parser | 2 (compat) |
| 70 | **spatial3d** | geom/ subpackage (35+ files untested) | 0 (gap) |
| 71 | **spatial** | SpatialFieldsWriter/Reader implementation | 0 (gap) |
| 72 | **queryparser** | PointQueryParser + SpanOr/SpanTermQuery | 4 |
| 73 | **queryparser** | XML QueryParser IndexWriter/DirectoryReader fixture | 1 |
| 74 | **backward_codecs** | EndiannessReverser writer parity + round-trip | 2 (compat) |
| 75 | **backward_codecs** | 63 stub test files (documentation-only ports) | 63 stubs |
| 76 | **backward_codecs** | Lucene50 BlockPostingsFormat integration | 3 stubs |
| 77 | **join** | CheckJoinIndex port | 1 (compat) |
| 78 | **store** | Chunked ByteBuffersDirectory constructor | 1 |
| 79 | **highlight** | Core components (break_iterator, span_fragmenter, token_sources, passage, scorers) | 0 (gap) |
| 80 | **grouping** | GroupReducer, block_grouping_collector, term_group_facet_collector | 0 (gap) |
| 81 | **suggest** | 45+ untested files (document/, fst/, tst/, spell/, analyzing/) | 0 (gap) |
| 82 | **suggest** | WFSTCompletionLookup + AnalyzingSuggester Store/Load | 4 (compat) |
| 83 | **memory** | MemoryIndex API gaps (10 t.Fatal lines in parity harness) | 10 |
| 84 | **misc** | IndexSplitter/IndexMergeTool write-replay | 2 (compat) |
| 85 | **misc** | FSTTester, ListOfOutputs, UpToTwoPositiveIntOutputs | 3 |
| 86 | **expressions** | Top-level integration (4 t.Fatal tests, no JS compiler) | 4 |
| 87 | **classification** | boolean_perceptron_classifier untested | 0 (gap) |
| 88 | **queries/function/valuesource** | 36 untested source files | 0 (gap) |
| 89 | **queries/intervals** | 26 untested source files | 2 (fatal) |
| 90 | **queries/spans** | 6 untested source files | 2 (fatal) |
| 91 | **queries/payloads** | 6 untested source files | 2 (fatal) |
| 92 | **queries** | common_terms_query.go (365 LOC) and mlt/more_like_this_query.go (186 LOC) untested | 0 (gap) |
| 93 | **spi** | 21 format-interface files completely untested | 0 (gap) |

### TIER 4 — Low (deferred, nice-to-have, or structurally blocked)

| # | Package | Gap Description |
|---|---------|----------------|
| 94 | **util** | StressRamUsageEstimator, TwoBPagedBytes (monster tests) |
| 95 | **codecs** | Term vectors prefetch/block storage |
| 96 | **codecs** | BitVectors (stub only, no tests) |
| 97 | **replicator** | HTTP replicator (removed in Lucene 10.4.0) |
| 98 | **backward_codecs** | Lucene40 BlockTree, Lucene70 .si, Lucene90 HNSW v0, Lucene99/103 postings cross-engine fixtures (read-only in 10.4.0) |
| 99 | **snowball** | Language-specific stemmer accuracy validation (28 languages) |
| 100 | **collation** | Collation strength/decomposition level support |
| 101 | **sandbox** | Quantization KMeans verification, FuzzyLikeThisQuery |
| 102 | **misc** | SweetSpotSimilarity / HighFreqTerms runtime parity |
| 103 | **store** | RateLimiter parity gap (`getMinPauseCheckBytes`) |
| 104 | **join** | QueryBitSetProducer weak cache |
| 105 | **queryparser** | QueryParserTestBase abstract base class port |

---

## Package Coverage Summary Matrix

| Package | Test Files | Active Tests | Deferred | Compat Tests | Compat Rows | Untested Sources | Assessment |
|---------|:---:|:---:|:---:|:---:|:---:|:---:|---|
| `index` | 318 | ~2,031 | 338 | 9 | 8 | ~10 | **MASSIVE GAP** |
| `search` | 278 | ~432 | 369 | 3 | 2 | 30+ | **CRITICAL GAP** |
| `codecs` | 91 | ~337 | 26 | 17 | 16+ | ~5 | Good, block on IW |
| `util` | 124 | ~1,118 | 37 | 0 | 0 | ~3 | Strong, needs fixtures |
| `analysis` | 206 | ~1,091 | 6 | 4 | 4 | ~5 | Strong, minor gaps |
| `document` | 44 | ~282 | 8 | 0 | 0 | 0 | Good, 3 blockers |
| `facets` | 85 | ~362 | 31 | 5 | 4 | ~5 | Good, BLOCKER-1 |
| `spatial` | 30 | ~260 | 0 | 8 | 6 | ~8 | **CRITICAL GAP** |
| `queryparser` | 49 | ~300 | 12 | 3 | 2 | ~5 | Good, minor gaps |
| `sandbox` | 30 | ~105 | 0 | 3 | 2 | ~5 | Good, minor gaps |
| `backward_codecs` | 74 | ~250 | 0 | 10 | 7+ | 63 stubs | **MASSIVE STUBS** |
| `join` | 42 | ~266 | 2 | 3 | 4 | ~3 | Good, 2 blockers |
| `geo` | 27 | ~287 | 0 | 0 | 0 | 0 | Mature, needs factories |
| `store` | 53 | ~296 | 1 | 6 | 6 | ~1 | Mature, 1 gap |
| `queries` | 40 | ~250 | 43 | 3 | 3 | 110+ | **MASSIVE GAP** |
| `spatial3d` | 7 | ~50 | 37 | 0 | 0 | 35+ | **CRITICAL GAP** |
| `highlight` | 21 | ~60 | 24 | 5 | 2 | 25 | **HIGH GAP** |
| `grouping` | 18 | ~80 | 23 | 3 | 3 | 11 | **HIGH GAP** |
| `replicator` | 2 | ~30 | 22 | 3 | 2 | 1 | Low (upstream removed) |
| `suggest` | 11 | ~40 | 20 | 6 | 4 | 45+ | **CRITICAL GAP** |
| `memory` | 4 | ~16 | 13 | 3 | 1 | 1 | High (mostly stub) |
| `misc` | 4 | ~32 | 11 | 6 | 4 | 6 | Medium |
| `expressions` | 14 | ~50 | 9 | 3 | 1 | 5 | High (no JS compiler) |
| `classification` | 4 | ~20 | 7 | 3 | 3 | 12 | High (BLOCKER-1) |
| `collation` | 2 | ~6 | 5 | 0 | 0 | 0 | Low |
| `snowball` | 2 | ~10 | 4 | 0 | 0 | 1 | Low |
| `spi` | 2 | ~13 | 3 | 0 | 0 | 21 | **CRITICAL GAP** |
| `bufferpool` | 1 | ~7 | 2 | 0 | 0 | 0 | Adequate |
| `payloads` | 1 | ~3 | 1 | 0 | 0 | 0 | Adequate |
| **TOTAL** | **1,944** | **~11,956** | **660** | **110** | **74+** | **300+** | |

---

## E2E Combined Scenario Coverage

The 6 combined scenarios (S1–S6) provide end-to-end certification across multiple subsystems. All 6 are byte-deterministic at canary seeds `0xC0FFEE` and `0xDECAF`.

| Scenario | Subsystems | Read Path | Write Path |
|----------|-----------|:---:|:---:|
| S1: `combined-multi-segment-index-search` | index + codecs + search + stored fields + DV + points + KNN + term vectors + norms | ✅ PASS | ❌ DEFERRED (BLOCKER-1) |
| S2: `combined-reverse-index-search` | search scoring invariance (multi vs single segment) | ✅ PASS | ❌ DEFERRED (BLOCKER-1) |
| S3: `combined-facets-search` | facets + taxonomy + search | ✅ PASS | ❌ DEFERRED (BLOCKER-1) |
| S4: `combined-replicator-roundtrip` | replicator NRT CopyState wire format | ✅ PASS | ❌ DEFERRED (BLOCKER-1) |
| S5: `combined-suggester-fst` | suggest FST persistence + lookup | ✅ PASS | ❌ DEFERRED |
| S6: `combined-highlight-queryparser-analysis` | highlight + queryparser + analysis | ✅ PASS | ❌ DEFERRED |

**Key insight:** The read path is certified for all 6 scenarios. The write path is uniformly blocked by BLOCKER-1 (SegmentReader core-readers gap).

---

## Test Type Pyramid Assessment

The ideal test pyramid for Gocene's certification mandate should be:

```
        /\
       /E2E\         6 combined scenarios (S1-S6)
      /------\
     /Compat \       110 files, 74+ scenarios (read + write + round-trip)
    /---------\
   /Integration\     Within-Gocene multi-component tests
  /-------------\
 /   Unit Tests  \   12,616 functions across 1,944 files
/-----------------\
```

**Assessment:**

- **Unit tests:** Strong base (12,616 functions). Major gaps exist in `queries/function/valuesource` (36 files untested), `suggest` (45+ files untested), `spatial3d/geom` (35+ files untested), `spi` (21 files untested), and 30+ search source files untested.
- **Integration tests:** Partially present. The `index` package has good component-level integration (IW + DR, IW + DV, IW + merge). `search` lacks IW→search integration. `facets` lacks IW→FacetsCollector→DirectoryTaxonomyWriter integration. `queryparser` lacks parser→search integration.
- **Compat tests:** Read path is strong for 23 audited packages. Write path is uniformly blocked by BLOCKER-1. Round-trip path is deferred for most scenarios.
- **E2E tests:** 6 combined scenarios cover the major cross-subsystem interactions. Write-path legs are deferred.

---

## Recommended Implementation Roadmap

### Phase 1 — Unblock the Foundation (Critical Path)
*Estimated: 3–5 sprints*

1. **BLOCKER-1:** SegmentReader core readers (rmp #4) — unblocks 12+ compat scenarios
2. **BLOCKER-2:** NRT DirectoryReader.open(IndexWriter) — unblocks 46+ index tests + 12 search tests
3. **BLOCKER-3:** DeleteDocuments (Term/Query) — unblocks 30+ index tests
4. **BLOCKER-4:** RandomIndexWriter / CannedTokenStream — unblocks 120+ tests across search/index/analysis
5. **Codec write path:** IW.Commit → codec writers → produces readable index files
6. **ForceMerge/ForceMergeDeletes:** merge pipeline completion

### Phase 2 — Fill Major Functional Gaps
*Estimated: 4–6 sprints*

7. **search:** IndexSearcher full integration, BooleanScorer/ReqExclBulkScorer, Span query tests
8. **queries/function/valuesource:** 36 files need tests
9. **spatial:** Spatial4j BinaryCodec decoder, prefix-tree postings reader, BBoxStrategy DV reader
10. **queryparser:** Query.String() byte-parity, IndexSearcher execution, StandardQueryParser features
11. **suggest:** 45+ files need tests, Store/Load port
12. **spatial3d/geom:** 35+ files need tests
13. **spi:** 21 format-interface files need contract tests

### Phase 3 — Complete Compat Certification
*Estimated: 3–4 sprints*

14. Write-path compat for all 6 E2E combined scenarios
15. Round-trip compat for all 74+ manifest scenarios
16. Remaining per-package compat write+round-trip legs
17. backward_codecs: 63 stub test files → active test functions
18. Monster tests behind `GOCENE_RUN_MONSTERS=1`

### Phase 4 — Polish and Stress
*Estimated: 2–3 sprints*

19. Concurrency stress tests across all packages
20. Language-specific analyzer validation (28 snowball languages, hunspell, kuromoji, nori, smartcn)
21. Performance benchmarks with Lucene parity verification
22. Cross-engine CheckIndex on Gocene-written indexes

---

## Appendix A: Search Package — Untested Source Files

The following search package source files have NO corresponding test:

`blended_term_query.go`, `combined_field_query.go`, `date_range_query.go`, `doc_and_score_query.go`, `double_values_source.go`, `field_exists_query.go`, `hnsw_queue_saturation_collector.go`, `late_interaction_float_values_source.go`, `late_interaction_rescorer.go`, `live_field_values.go`, `long_values.go`, `long_values_source.go`, `lru_query_cache.go`, `matches.go`, `matches_iterator.go`, `matches_utils.go`, `multi_collector_manager.go`, `multi_term_query_constant_score_blended_wrapper.go`, `multi_value_mode.go`, `ngram_phrase_query.go`, `per_field_similarity.go`, `point_query.go`, `positive_scores_only_collector.go`, `query_cache.go`, `query_rescorer.go`, `query_visitor.go`, `rescore_top_n_query.go`, `rescorer.go`, `score_caching_wrapping_scorer.go`, `score_mode.go`, `scorer_supplier.go`, `similarity.go`, `skip_block_range_iterator.go`, `sorted_numeric_selector.go`, `sorted_set_selector.go`, `sort_rescorer.go`, `task_executor.go`, `time_limiting_knn_collector_manager.go`, `top_field_collector_manager.go`, `top_score_doc_collector_manager.go`, `total_hit_count_collector.go`, `total_hit_count_collector_manager.go`, `union_postings_enum.go`, `vector_scorer.go`

## Appendix B: Backward Codecs — Stub Test Files

63 test files in `backward_codecs/` are documentation-only stubs (0 test functions, `// Port of Java class XYZ — no executable code; full port deferred`). These cover: `backward_index/` (10 files), `lucene50/` (5 files), `lucene80/` (3 files), `lucene86/` (3 files), `lucene87/` (2 files), `lucene99/` (3 files), `compressing/` (3 files), and ~34 `_rw_*_test.go` / `_writer_test.go` files across various lucene version directories.

## Appendix C: SPI Package — Untested Format Interfaces

The `spi/` package defines 21 format-interface files with zero unit tests: `buffered_updates.go`, `codec.go`, `codec_util.go`, `compound_format.go`, `doc_values_format.go`, `doc_values.go`, `doc_values_iterators.go`, `field_infos_format.go`, `indexable_field.go`, `index_not_found_exception.go`, `knn_vectors_format.go`, `norms_format.go`, `points_format.go`, `postings_format.go`, `segment_commit_info.go`, `segment_info_format.go`, `segment_infos_format.go`, `segment_infos.go`, `sorter_doc_map.go`, `state.go`, `stored_fields_format.go`, `term_vectors_format.go`.

---

*End of audit.*
