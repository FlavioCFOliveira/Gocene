# Skipped / Deferred Tests Audit

**Generated:** 2026-06-11
**Policy:** No-Skip policy — tests must use `t.Fatal` with a blocker description instead of `t.Skip`

## Key Findings

- **Total `t.Skip` calls remaining: 0** (the no-skip policy is fully enforced)
- **Total deferred `t.Fatal` calls: 660** across 33 packages
- All deferred tests fail with descriptive blocker reasons, making the test suite informative about what remains unimplemented

## Summary Table

| Package | Deferred Tests | Blocker Summary |
|---------|:--------------:|-----------------|
| `index` | 338 | NRT reader, DeleteDocuments, MockDirectoryWrapper, IndexWriter integration, ForceMerge, merge scheduler, CannedTokenStream, monster tests |
| `search` | 140 | RandomIndexWriter, GeoTestUtil, spatial query factories, IndexSearcher, Sort/SortField, span queries, regexp flags |
| `codecs` | 26 | Lucene99 formats, PerField codecs, DocValuesSkipper, merged vector values, term vectors, stored fields |
| `util/bkd` | 22 | verify() helper, random seeding, MockDirectoryWrapper, bit-corruption, byte-exact comparison, offline path |
| `facets/taxonomy` | 17 | IndexWriter + FacetsCollector + DirectoryTaxonomyWriter/Reader pipeline |
| `memory` | 13 | MemoryIndex.createSearcher(), DirectoryReader, PostingsEnum, Span queries, DocValues |
| `facets` | 10 | RandomIndexWriter, DirectoryTaxonomyWriter, FacetsCollectorManager, DrillSideways |
| `util` | 8 | BitDocIdSet.RamBytesUsed, DocIdSetIterator.IntoBitSet, monster / stress tests |
| `queries/spans` | 8 | Full span query/scoring/explanation infrastructure deferred to backlog |
| `queries/function` | 8 | Full function query explanation/sort/KNN infrastructure deferred to backlog |
| `document` | 8 | KNN monster tests, PerFieldConsistency, point range queries |
| `search/comparators` | 6 | IndexWriter+IndexSearcher integration, DocIdSetIterator.IntoBitSet |
| `queryparser/flexible` | 6 | StandardQueryParser feature set (ranges, FuzzyQuery, MultiField, PointQueryParser) |
| `analysis` | 6 | MockGraphTokenFilter/MockTokenizer, Hunspell external dictionaries |
| `queries/payloads` | 5 | Full PayloadCheckQuery/PayloadTermQuery/span position infrastructure deferred |
| `util/hnsw` | 4 | GOMAXPROCS >= 2 concurrency requirement |
| `suggest` | 4 | WFSTCompletionLookup, AnalyzingSuggester, TSTLookup, FSTCompletionLookup |
| `expressions` | 4 | ANTLR JavascriptCompiler, IndexSearcher infrastructure |
| `misc/util/fst` | 3 | FSTTester, ListOfOutputs implementation |
| `facets/taxonomy/directory` | 3 | DirectoryTaxonomyWriter snapshot/rollback, SearcherTaxonomyManager |
| `analysis/hunspell` | 3 | External Hunspell dictionary repository |
| `util/fst` | 2 | Monster FST tests (multi-GiB) |
| `queryparser/flexible/spans` | 2 | SpanOrQuery/SpanTermQuery implementations |
| `queryparser` | 2 | Analyzer support, MultiPhraseQuery |
| `queries/intervals` | 2 | MockTokenizer, RandomIndexWriter, MatchesTestBase |
| `misc/store` | 2 | Hardlink filesystem support |
| `join` | 2 | PostingsEnum.Advance, PrefixQuery parents filter |
| `util/automaton` | 1 | Huge minimize test in short mode |
| `store` | 1 | Chunked ByteBuffersDirectory |
| `queryparser/xml` | 1 | Reuters fixture, IndexWriter/DirectoryReader |
| `queryparser/util` | 1 | Abstract base class port |
| `queries/function/docvalues` | 1 | Numeric DocValues + FunctionValues |
| `facets/taxonomywritercache` | 1 | 2GB monster test |
| **Total** | **660** | |

## Detailed Deferred Test Listing

### `index` (338 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestIndexWriterCommit` (multiple) | `index/index_writer_commit_test.go:109` | Reopen does not yet observe externally committed segments |
| `TestIndexWriterCommit` | `index/index_writer_commit_test.go:333` | DirectoryReader.Leaves does not yet expose per-segment leaves |
| `TestIndexWriterCommit` | `index/index_writer_commit_test.go:895` | SetLiveCommitData is not yet persisted to the commit point |
| `TestIndexWriterCommit` | `index/index_writer_commit_test.go:990` | SetLiveCommitData is not yet persisted to the commit point |
| `TestIndexWriterCommit` | `index/index_writer_commit_test.go:1085` | MockDirectoryWrapper disk-usage tracking not yet ported |
| `TestIndexWriterCommit` | `index/index_writer_commit_test.go:1098` | RandomIndexWriter / OpenIfChanged not yet available |
| `TestIndexWriterCommit` | `index/index_writer_commit_test.go:1111` | IndexWriterConfig.SetIndexCommit is not yet implemented |
| `TestIndexWriterMergePolicy` (11 calls) | `index/index_writer_merge_policy_test.go:298-800` | Requires full IndexWriter/MergeScheduler/DirectoryReader/failure injection/doc values update/NRT reader/soft deletes |
| `TestIndexWriterMerge` (4 calls) | `index/index_writer_merge_test.go:180-270` | MergeScheduler not yet implemented, disable background merge, compound file verification |
| `TestIndexWriterMerging` (16 calls) | `index/index_writer_merging_test.go:69-747` | AddIndexes, DirectoryReader.Open, DeleteDocuments, ForceMergeDeletes, LogMergePolicy, CMS not fully implemented |
| `TestIndexWriterForceMerge` (4 calls) | `index/index_writer_force_merge_test.go:108-248` | ForceMerge does not honor maxNumSegments; MockDirectoryWrapper; background overload not implemented |
| `TestSizeBoundedForceMerge` (11 calls) | `index/size_bounded_force_merge_test.go:121-369` | ForceMerge ignores LogByteSize/LogDocMergePolicy size caps; assertions deferred |
| `TestIndexWriterWithThreads` (14 calls) | `index/index_writer_with_threads_test.go:78-155` | MockDirectoryWrapper, Document pipeline, CMS, RandomIndexWriter (Sprint 55 option c) |
| `TestIndexWriterReader` (16 calls) | `index/index_writer_reader_test.go:44-442` | NRT reader, DeleteDocuments, MergedSegmentWarmer, SimpleMergedSegmentWarmer, setLeafSorter |
| `TestIndexWriterDelete` (14 calls) | `index/index_writer_delete_test.go:299-898` | infra gaps: DeleteDocuments no-op stubs, NRT reader, MockDirectoryWrapper, LogDocMergePolicy |
| `TestIndexWriterError` (2 calls) | `index/index_writer_error_test.go:248-279` | MockDirectoryWrapper IO injection; Rollback not yet implemented |
| `TestIndexWriterTragic` (2 calls) | `index/index_writer_tragic_test.go:30-37` | Tragic event injection; MockDirectory WriteSegmentInfos failure |
| `TestIndexWriterNrtIsCurrent` | `index/index_writer_nrt_is_current_test.go:48` | needs NRT DirectoryReader.open(writer) and openIfChanged |
| `TestIndexWriterOnDiskFull` (4 calls) | `index/index_writer_on_disk_full_test.go:52-83` | needs MockDirectoryWrapper for disk-full simulation |
| `TestIndexWriterOutOfFileDescriptors` | `index/index_writer_out_of_file_descriptors_test.go:42` | GOC-4137: MockDirectoryWrapper fault injection |
| `TestIndexWriterExceptions2` | `index/index_writer_exceptions2_test.go:181` | CrankyCodec/CrankyTokenFilter fault injection unavailable |
| `TestConcurrentMergeScheduler` (4 calls) | `index/concurrent_merge_scheduler_test.go:143-404` | Full merge async/integration/stalling test requires complete IndexWriter |
| `TestTragicIndexWriterDeadlock` (3 calls) | `index/tragic_index_writer_deadlock_test.go:30-44` | Sprint 55 option (c): MockDirectoryWrapper, CMS hooks |
| `TestTermVectorsReader` (11 calls) | `index/term_vectors_reader_test.go:133-193` | Sprint 55 option c: needs IndexWriter term-vectors flush + SegmentReader |
| `TestPayloads` (3 calls) | `index/payloads_test.go:131-155` | needs CannedTokenStream + PayloadAnalyzer + wired block-tree |
| `TestPayloadsOnVectors` (3 calls) | `index/payloads_on_vectors_test.go:67-148` | payload read-back requires CannedTokenStream + TermVectors |
| `TestPostingsOffsets` (12 calls) | `index/postings_offsets_test.go:63-191` | CannedTokenStream, English number-to-words helper, Analyzer getOffsetGap |
| `TestMaxPosition` (2 calls) | `index/max_position_test.go:49-61` | CannedTokenStream unimplemented |
| `TestOmitPositions` | `index/omit_positions_test.go:62` | needs NRT reader + wired block-tree postings |
| `TestOmitTf` | `index/omit_tf_test.go:146` | needs RandomIndexWriter + NRT path |
| `TestFieldReuse` | `index/field_reuse_test.go:72` | needs CannedTokenStream + IndexWriter.addDocument |
| `TestCodecs` (3 calls) | `index/codecs_test.go:52-82` | Lucene103PostingsFormat typed stubs; no postings round-trip |
| `TestDefaultCodecPersistence` | `index/default_codec_persistence_test.go:49` | blocked on rmp #4670 — IndexWriter.Commit does not invoke codec writers |
| `TestIndexWriterUnicode` (3 calls) | `index/index_writer_unicode_test.go:248-263` | GOC-4184: IndexWriter/DirectoryReader round-trip not ported |
| `TestIndexingSequenceNumbers` (4 calls) | `index/indexing_sequence_numbers_test.go:64-147` | needs AddDocument sequence numbers, NoDeletionPolicy, functional delete |
| `TestIndexSorting` (25 calls) | `index/index_sorting_test.go:289-1762` | GOC-4136: AssertingNeedsIndexSortCodec, RandomIndexWriter, AddDocuments, StoredFields, IndexSearcher |
| `TestBinaryDocValuesUpdates` (3 calls) | `index/binary_doc_values_updates_test.go:299-1530` | infra gap: NumDocs does not subtract applied deletes |
| `TestNumericDocValuesUpdates` (2 calls) | `index/numeric_doc_values_updates_test.go:42-231` | infra gap: no core readers; writer does not reject numeric update |
| `TestMixedDocValuesUpdates` (13 calls) | `index/mixed_doc_values_updates_test.go:27-100` | GOC-4202: pending updateDocValues + NRT reopen |
| `TestSegmentCoreReadersDV` (2 calls) | `index/segment_core_readers_dv_test.go:63-66` | GetCoreReaders()/GetDocValuesProducer() = nil (rmp #4) |
| `TestReaderClosed` | `index/reader_closed_test.go:56` | SegmentReader without core readers; No AlreadyClosedException |
| `TestDocInverterPerFieldErrorInfo` (2 calls) | `index/doc_inverter_per_field_error_info_test.go:53-78` | GOC-4199: pending SetInfoStream + DocInverter error reporting |
| `TestInfoStream` (2 calls) | `index/info_stream_test.go:73-82` | No SetInfoStream; no isEnableTestPoints (Sprint 55 option c) |
| `TestCheckIndexCompatibility` (6 calls) | `index/checkindex_compatibility_test.go:54-148` | checkindex not implemented |
| `TestIndexCommit` (3 calls) | `index/index_commit_test.go:99-135` | list commits / OpenDirectoryReaderAtCommitPoint not implemented |
| `TestDeletionPolicy` (6 calls) | `index/deletion_policy_test.go:67-163` | needs functional IndexCommit.Delete + commit-generation |
| `TestIsCurrent` (2 calls) | `index/is_current_test.go:33-42` | needs NRT IndexWriter.GetReader; DeleteDocuments is no-op stub |
| `TestNewestSegment` | `index/newest_segment_test.go:22` | Sprint 55 option c: needs IndexWriter.newestSegment |
| `TestSegmentToThreadMapping` | `index/segment_to_thread_mapping_test.go:20` | IndexSearcher has no Slices/LeafSlice API |
| `TestSumDocFreq` | `index/sum_doc_freq_test.go:39` | GOC-4173: needs RandomIndexWriter + MultiTerms |
| `TestDocCount` | `index/doc_count_test.go:39` | GOC-4140: needs RandomIndexWriter + MultiTerms |
| `TestMultiLevelSkipList` | `index/multi_level_skip_list_test.go:52` | GOC-4153: faithful port deferred |
| `TestParallelTermEnum` | `index/parallel_term_enum_test.go:57` | needs getOnlyLeafReader + wired block-tree terms |
| `TestParallelReaderEmptyIndex` (2 calls) | `index/parallel_reader_empty_index_test.go:53-71` | needs Directory copy + AddIndexes; DeleteDocuments |
| `TestRollingUpdates` (2 calls) | `index/rolling_updates_test.go:43-58` | infra gap: no NRT reader; LineFileDocs not ported |
| `TestIndexWriterFromReader` (8 calls) | `index/index_writer_from_reader_test.go:160-253` | blocked by rmp #118: commit-pinning/rollback |
| `TestIndexWriterThreadsToSegments` (2 calls) | `index/index_writer_threads_to_segments_test.go:214-222` | RandomIndexWriter; nightly (Sprint 55 option c) |
| `TestCrashCausesCorruptIndex` | `index/crash_causes_corrupt_index_test.go:98` | GOC-4165: crash-recovery + DirectoryReader/IndexSearcher not available |
| `TestTransactions` | `index/transactions_test.go:37` | port blocked: no MockDirectoryWrapper Failure |
| `TestForceMergeForever` | `index/force_merge_forever_test.go:56` | needs IndexWriter merge-hook |
| `TestThreadedForceMerge` | `index/threaded_force_merge_test.go:50` | DeleteDocuments(Term) is no-op stub |
| `TestStressIndexing` | `index/stress_indexing_test.go:93` | infra gap: DeleteDocuments is no-op stub |
| `TestStressDeletes` | `index/stress_deletes_test.go:30` | infra gap: DeleteDocuments no-op stubs |
| `TestSoftDeletesIntegration` (2 calls) | `index/soft_deletes_integration_test.go:14-18` | SoftUpdateDocument not yet implemented |
| `TestTryDelete` (2 calls) | `index/try_delete_test.go:112-123` | no NRT reader; DeleteDocumentsQuery no-op stub |
| `TestPerSegmentDeletes` | `index/per_segment_deletes_test.go:22` | deferred: IndexWriter.MaybeMerge, HasChangesInRam, NRT reader |
| `TestDirectoryReaderReopen` (2 calls) | `index/directory_reader_reopen_test.go:139-153` | needs openIfChanged; MockDirectoryWrapper |
| `TestAllFilesDetectMismatchedChecksum` | `index/all_files_detect_mismatched_checksum_test.go:41` | per-file CRC32 verification not implemented |
| `TestAllFilesDetectTruncation` | `index/all_files_detect_truncation_test.go:37` | per-file CRC32 verification not implemented |
| `TestAllFilesCheckIndexHeader` | `index/all_files_check_index_header_test.go:41` | OpenDirectoryReader does not validate codec headers |
| `TestAllFilesHaveCodecHeader` | `index/all_files_have_codec_header_test.go:54` | WriteSegmentInfos does not write CODEC_MAGIC header |
| `TestCodecHoldsOpenFiles` | `index/codec_holds_open_files_test.go:37` | no NRT reader + TestUtil.checkReader |
| `TestIndexTooManyDocs` | `index/index_too_many_docs_test.go:41` | IndexWriter MaxDocs cap |
| `TestDemoParallelLeafReader` | `index/demo_parallel_leaf_reader_test.go:24` | needs NRT reader reopen |
| `TestForTooMuchCloning` | `index/for_too_much_cloning_test.go:26` | clone-counting MockDirectoryWrapper |
| `TestIndexUpgrader` (2 calls) | `index/index_upgrader_test.go:54-99` | upgrader not implemented |
| `TestLongPostings` (2 calls) | `index/long_postings_test.go:50-61` | needs wired block-tree postings + RandomIndexWriter |
| `TestCustomTermFreq` | `index/custom_term_freq_test.go:432` | NeverForgetsSimilarity capture hook depends on SetSimilarity |
| **NRT Stress/Search/Indexing/Concurrency/Replication suites** (5 files, ~46 calls) | `index/nrt_stress_test.go`, `index/nrt_search_test.go`, `index/nrt_indexing_test.go`, `index/nrt_concurrency_test.go`, `index/replication_integration_test.go` | Import cycle / unimplemented APIs — NRT primitives not yet fixed |
| **Monster tests** (12 calls across 10 files) | `index/two_b_*.go`, `index/four_gb_*.go`, `index/twob_*.go` | Monster tests: >2B docs/terms/points/positions, multi-hour runtime, multi-GB heap/disk; deferred behind GOCENE_RUN_MONSTERS=1 |

### `search` (140 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestSearcherManager` (12 calls) | `search/searcher_manager_test.go:22-66` | Requires IndexWriter.GetReader — not yet implemented |
| `TestSimilarityScoring` (4 calls) | `search/similarity_scoring_test.go:77-479` | search / boolean search not implemented |
| `TestScoringReproducibility` (3 calls) | `search/scoring_reproducibility_test.go:74-254` | search / boolean search not implemented |
| `TestCustomCollector` (2 calls) | `search/custom_collector_test.go:64-114` | custom collector / search not implemented |
| `TestQueryExpansion` (3 calls) | `search/query_expansion_test.go:63-149` | rewrite not implemented |
| `TestHighlightingCompatibility` (2 calls) | `search/highlighting_compatibility_test.go:17-21` | highlight package not yet implemented |
| `TestBooleanScorer` (5 calls) | `search/boolean_scorer_test.go:153-233` | BooleanScorerSupplier, ReqExclBulkScorer, QueryUtils, IndexSearcher.Rewrite, TopScoreDocCollectorManager |
| `TestReqOptSumScorer` (8 calls) | `search/req_opt_sum_scorer_test.go:33-61` | requires RandomIndexWriter, BooleanQuery, TermQuery |
| `TestReqExclBulkScorer` | `search/req_excl_bulk_scorer_test.go:226` | needs RandomTwoPhaseView |
| `TestScorerUtil` | `search/scorer_util_test.go:89` | likelyLiveDocs / likelyImpactsEnum require index stack |
| `TestTermInSetQuery` (5 calls) | `search/term_in_set_query_test.go:401-429` | Requires doc values, RamUsageTester, FilterDirectoryReader, UsageTrackingQueryCachingPolicy, QueryVisitor |
| `TestSearchAfter` (4 calls) | `search/search_after_test.go:62-211` | Requires Sort, SortField, TopFieldCollector |
| `TestSortedSetSortField` (3 calls) | `search/sorted_set_sort_field_test.go:168-178` | requires IndexSearcher + RandomIndexWriter |
| `TestSortedSetDocValuesSetQuery` | `search/sorted_set_doc_values_set_query_test.go:19` | not yet ported (GOC-3220) |
| `TestTimeLimitingBulkScorer` | `search/time_limiting_bulk_scorer_test.go:83` | requires full IndexWriter/IndexSearcher/TermQuery |
| `TestReadAheadMatchAllDocsQuery` | `search/read_ahead_match_all_docs_query_test.go:22` | requires DenseConjunctionBulkScorer |
| `TestRegexpQuery` (4 calls) | `search/regexp_query_test.go:185-577` | RegExp flags, AutomatonProvider, syntax not yet defined |
| `TestAxiomaticSimilarity` (3 calls) | `search/test_axiomatic_similarity_test.go:22-32` | parameter validation deferred |
| `TestTermsEnum2` (4 calls) | `search/terms_enum2_test.go:74-118` | blocked: AutomatonTestUtil, MultiTerms |
| `TestMultiCollector` | `search/multi_collector_test.go:608` | setScorer not yet called |
| `TestSprint117Stubs` (5 calls) | `search/sprint117_stubs_test.go:59-266` | Rewrite returned nil; CreateWeight returned nil Weight |
| `TestLatLonPointDistanceSort` (4 calls) | `search/lat_lon_point_distance_sort_test.go:164-193` | NewDistanceSort, IndexSearcher.Search wiring, RandomIndexWriter, @Nightly |
| **Spatial query test suites** (~51 calls across 18 files) | `search/base_lat_lon_*.go`, `search/base_xy_*.go`, `search/base_spatial_*.go`, `search/lat_lon_*_queries_test.go`, `search/xy_*_queries_test.go` | blocked by RandomIndexWriter / GeoTestUtil / LatLonShape / LatLonPoint / XYShape query factories / Tessellator; remove when fixed |
| `TestBaseShapeEncoding` (2 calls) | `search/base_shape_encoding_test_case_test.go:176-187` | document.DecodeTriangle not yet recovery vertices; rotation-aware ShapeField decoder (backlog #2697) |

### `codecs` (26 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestLucene99PostingsFormat` | `codecs/lucene99_codec_test.go:67` | Lucene99PostingsFormat not yet implemented in Gocene |
| `TestLucene99StoredFieldsFormat` | `codecs/lucene99_codec_test.go:71` | Lucene99StoredFieldsFormat not yet implemented in Gocene |
| `TestLucene99DocValuesFormat` | `codecs/lucene99_codec_test.go:75` | Lucene99DocValuesFormat not yet implemented in Gocene |
| `TestPerFieldPostingsFormat2` (6 calls) | `codecs/per_field_postings_format2_test.go:86-148` | blocked by AssertingCodec / DirectPostingsFormat / MockAnalyzer / RandomIndexWriter / RandomCodec |
| `TestPerFieldDocValuesFormat` (4 calls) | `codecs/per_field_doc_values_format_test.go:37-499` | PerFieldDocValuesFormat not yet fully implemented (GC-212) |
| `TestPerFieldPostingsFormat` | `codecs/per_field_postings_format_test.go:166` | Randomized per-field postings testing not yet fully implemented |
| `TestSkipsInMergedByteVectorValues` | `codecs/merged_vector_values_test.go:24` | requires KnnVectorsWriter.MergedByteVectorValues (Sprint 55 gap) |
| `TestSkipsInMergedFloat32VectorValues` | `codecs/merged_vector_values_test.go:32` | requires KnnVectorsWriter.MergedFloat32VectorValues (Sprint 55 gap) |
| `TestLucene90DocValuesFormatVariableSkipInterval` (4 calls) | `codecs/lucene90_doc_values_format_variable_skip_interval_test.go:147-185` | DocValuesSkipper not yet wired; requires index-sort + indexedField helpers (deferred) |
| `TestCompressingTermVectorsFormat` (2 calls) | `codecs/compressing_term_vectors_format_test.go:360-393` | requires RandomIndexWriter / NoMergePolicy/LogMergePolicy |
| `TestLucene90TermVectorsFormat_Prefetch` | `codecs/lucene90_term_vectors_format_test.go:139` | requires block-based storage not yet ported |
| `TestLucene90StoredFieldsFormatHighCompression` | `codecs/lucene90_stored_fields_format_high_compression_test.go:92` | requires Mode-aware Lucene104Codec not yet ported |
| `TestCompressingStoredFieldsFormat` (2 calls) | `codecs/compressing_stored_fields_format_test.go:570-687` | full IndexWriter integration; reference data from Lucene Java |

### `util/bkd` (22 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestFourBBKD` | `util/bkd/fourbbkdpoints_test.go:36` | Monster test: >4B BKD points, ~4h, many GiB (GOC-4307) |
| `TestBKDRandom` | `util/bkd/bkd_test.go:93` | requires LuceneTestCase random() harness; deferred Sprint 56+ |
| `TestBKDRandomMulti` | `util/bkd/bkd_test.go:103` | requires reproducible random seeding + BigInt fixture; deferred |
| `TestBKDRandomWithMKD` | `util/bkd/bkd_test.go:114` | requires MockDirectoryWrapper + CorruptingIndexOutput |
| `TestBKD_MultiValued` | `util/bkd/bkd_test.go:123` | requires verify() helper + MutablePointTree reopen |
| `TestBKD_RandomBinaryTiny` | `util/bkd/bkd_test.go:129` | requires verify() helper |
| `TestBKD_RandomTiny` | `util/bkd/bkd_test.go:135` | requires verify() helper + @Nightly gating; deferred |
| `TestBKD_EmptyIndex` | `util/bkd/bkd_test.go:166` | requires verify() helper; deferred |
| `TestBKD_IndexIsSingleValue` | `util/bkd/bkd_test.go:173` | requires verify() helper; deferred |
| `TestBKD_WithAllEquals` | `util/bkd/bkd_test.go:217` | requires verify() helper; deferred |
| `TestBKD_NegativeValues` | `util/bkd/bkd_test.go:224` | requires verify() helper; deferred |
| `TestBKD_ZeroValue` | `util/bkd/bkd_test.go:231` | requires verify() helper; deferred |
| `TestBKD_MultiValued` | `util/bkd/bkd_test.go:241` | requires multi-valued verify() helper + MutablePointTree reopen; deferred |
| `TestBKD_CorruptOnPartition1` | `util/bkd/bkd_test.go:251` | requires FilterDirectory + bit-corruption helper |
| `TestBKD_BitFlippedOnPartition1` | `util/bkd/bkd_test.go:257` | requires bit-corruption helper |
| `TestBKD_ExactFromJava` | `util/bkd/bkd_test.go:264` | requires byte-exact comparison against Java fixture; deferred |
| `TestBKD_DataDimensionReordering` | `util/bkd/bkd_test.go:271` | data-dim reordering not exposed by Gocene BKDWriter |
| `TestBKD_Offline` | `util/bkd/bkd_test.go:277` | requires verify() helper exercising offline path; deferred |
| `TestBKD_RandomLarge` | `util/bkd/bkd_test.go:284` | requires verify() helper; deferred |
| `TestBKD_TotalPointCount` | `util/bkd/bkd_test.go:303` | requires verify() helper + totalPointCount guard; deferred |
| `TestBKD_MissingPoint` | `util/bkd/bkd_test.go:309` | requires verify() helper; deferred |
| `TestBKD_SparseValues` | `util/bkd/bkd_test.go:315` | requires verify() helper; deferred |

### `facets/taxonomy` (17 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestTaxonomyFacetCounts2` (4 calls) | `facets/taxonomy/test_taxonomy_facet_counts2_test.go:111-123` | requires IndexWriter + FacetsCollector + FastTaxonomyFacetCounts pipeline |
| `TestTaxonomyFacetCounts` (4 calls) | `facets/taxonomy/test_taxonomy_facet_counts_test.go:81-93` | requires IndexWriter + FacetsCollector + DirectoryTaxonomyWriter/Reader pipeline |
| `TestTaxonomyFacetAssociations` (4 calls) | `facets/taxonomy/test_taxonomy_facet_associations_test.go:124-136` | requires IndexWriter + FacetsCollector + TaxonomyFacetIntAssociations/FloatAssociations |
| `TestTaxonomyFacetValueSource` (2 calls) | `facets/taxonomy/test_taxonomy_facet_value_source_test.go:98-102` | requires IndexWriter + FacetsCollector + DocValues + TaxonomyFacets |
| `TestOrdinalData` (3 calls) | `facets/taxonomy/test_ordinal_data_test.go:92-100` | requires IndexWriter + DirectoryTaxonomyReader + ReindexingEnrichedDirectoryTaxonomyWriter |

### `memory` (13 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestMemoryIndex` | `memory/memory_index_test.go:118` | search not implemented |
| `TestMemoryIndexAgainstDirectory` (11 calls) | `memory/memory_index_against_directory_test.go:60-162` | requires MemoryIndex.createSearcher(), DirectoryReader, PostingsEnum, SpanQueries, PhraseQuery, DocValues, NormValues, IntPoint/LongPoint, termVectors, CannedTokenStream, duellReaders |

### `facets` (10 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestMultipleIndexFields` (5 calls) | `facets/test_multiple_index_fields_test.go:68-84` | requires RandomIndexWriter + DirectoryTaxonomyWriter + MultiFacets pipeline |
| `TestParallelDrillSideways` (3 calls) | `facets/test_parallel_drill_sideways_test.go:28-40` | requires DrillSideways + goroutine-pool + TaxonomyReader |
| `TestFacetIntegration` (2 calls) | `facets/facet_integration_test.go:59-131` | FacetsConfig.SetIndexPath not yet implemented; DrillDownQuery not yet implemented |

### `util` (8 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestFixedBitDocIdSet` | `util/fixed_bit_doc_id_set_test.go:541` | BitDocIdSet.RamBytesUsed not implemented |
| `TestFixedBitDocIdSet` (2 calls) | `util/fixed_bit_doc_id_set_test.go:571-597` | DocIdSetIterator.IntoBitSet not implemented |
| `TestSparseFixedBitDocIdSet` | `util/sparse_fixed_bit_doc_id_set_test.go:221` | BitDocIdSet.RamBytesUsed not implemented |
| `TestSparseFixedBitDocIdSet` (2 calls) | `util/sparse_fixed_bit_doc_id_set_test.go:245-261` | DocIdSetIterator.IntoBitSet not implemented |
| `TestStressRamUsageEstimator` | `util/stress_ram_usage_estimator_test.go:31` | stress monster test (GOCENE_RUN_MONSTERS) |
| `TestTwoBPagedBytes` | `util/twobpagedbytes_test.go:27` | monster test; PagedBytes.Copy not landed |

### `queries/spans` (8 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestSpanSimilarity` | `queries/spans/span_similarity_test.go:15` | deferred to backlog |
| `TestQueryRescorerWithSpans` | `queries/spans/query_rescorer_with_spans_test.go:15` | deferred to backlog |
| `TestSpanCollection` | `queries/spans/span_collection_test.go:15` | deferred to backlog |
| `TestBasics` | `queries/spans/test_basics_test.go:16` | deferred to backlog |
| `TestSpanSearchEquivalence` | `queries/spans/span_search_equivalence_test.go:15` | deferred to backlog |
| `TestSpanExplanationsOfNonMatches` | `queries/spans/span_explanations_of_non_matches_test.go:15` | deferred to backlog |
| `TestSpanExplanations` | `queries/spans/span_explanations_test.go:15` | deferred to backlog |
| `TestSpansEnum` | `queries/spans/spans_enum_test.go:14` | deferred to backlog |

### `queries/function` (8 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestFunctionQueryExplanations` | `queries/function/function_query_explanations_test.go:15` | deferred to backlog |
| `TestFunctionScoreExplanations` | `queries/function/function_score_explanations_test.go:15` | deferred to backlog |
| `TestKnnVectorSimilarityFunctions` | `queries/function/knn_vector_similarity_functions_test.go:15` | deferred to backlog |
| `TestFieldScoreQuery` | `queries/function/field_score_query_test.go:15` | deferred to backlog |
| `TestFunctionQuerySort` | `queries/function/function_query_sort_test.go:15` | deferred to backlog |
| `TestDocValuesFieldSources` | `queries/function/doc_values_field_sources_test.go:15` | deferred to backlog |
| `TestLongNormValueSource` | `queries/function/long_norm_value_source_test.go:15` | deferred to backlog |
| `TestSortedSetFieldSource` | `queries/function/sorted_set_field_source_test.go:16` | deferred to backlog |

### `document` (8 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestManyKnnDocs` (2 calls) | `document/many_knn_docs_test.go:34-42` | monster test: pending IndexWriter + HNSW (Sprint 55 stub) |
| `TestPerFieldConsistency` (3 calls) | `document/per_field_consistency_test.go:27-41` | requires IndexWriter/DirectoryReader; deferred (GOC-4013, Sprint 55 option c) |
| `TestPointValuesCompatibility` (3 calls) | `document/point_values_compatibility_test.go:208-268` | point range query not yet fully implemented; range query not implemented |

### `search/comparators` (6 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestTermOrdValComparatorAdaptiveSkipping` (5 calls) | `search/comparators/test_term_ord_val_comparator_adaptive_skipping_test.go:16-28` | requires complete IndexWriter+IndexSearcher integration |
| `TestUpdateableDocIdSetIterator` | `search/comparators/updateable_doc_id_set_iterator_test.go:103` | intoBitSet not on DocIdSetIterator interface — deferred |

### `queryparser/flexible` (6 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestStandardQPEnhancements` | `queryparser/flexible/test_standard_qp_enhancements_test.go:20` | deferred: requires MultiTermQuery rewrite + date-range resolution |
| `TestMultiAnalyzerQpHelper` | `queryparser/flexible/test_multi_analyzer_qp_helper_test.go:20` | deferred: requires multi-token position handling |
| `TestQpHelper` | `queryparser/flexible/test_qp_helper_test.go:22` | deferred: requires full StandardQueryParser |
| `TestStandardQP` | `queryparser/flexible/test_standard_qp_test.go:21` | deferred: requires complete StandardQueryParser |
| `TestPointQueryParser` | `queryparser/flexible/test_point_query_parser_test.go:22` | deferred: requires SetPointsConfigMap + point range query factories |
| `TestMultiFieldQpHelper` | `queryparser/flexible/test_multi_field_qp_helper_test.go:20` | deferred: requires MultiFieldQueryParser |

### `analysis` (6 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestGraphTokenizers` (6 calls) | `analysis/graph_tokenizers_test.go:430-460` | requires MockGraphTokenFilter/MockHoleInjectingTokenFilter/MockTokenizer infrastructure not yet ported |
| `TestHunspellStemmer` (3 calls) | `analysis/hunspell/stemmer_test.go:286-959` | requires external Hunspell dictionary repository / performance corpus |

### `queries/payloads` (5 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestPayloadCheckQuery` | `queries/payloads/payload_check_query_test.go:15` | deferred to backlog |
| `TestPayloadExplanations` | `queries/payloads/payload_explanations_test.go:15` | deferred to backlog |
| `TestPayloadSpans` | `queries/payloads/payload_spans_test.go:15` | deferred to backlog |
| `TestPayloadTermQuery` | `queries/payloads/payload_term_query_test.go:15` | deferred to backlog |
| `TestPayloadSpanPositions` | `queries/payloads/payload_span_positions_test.go:15` | deferred to backlog |

### `util/hnsw` (4 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestConcurrentHnswMerger` (2 calls) | `util/hnsw/concurrent_hnsw_merger_test.go:226-302` | requires GOMAXPROCS >= 2 for meaningful concurrency |
| `TestHnswConcurrentMergeBuilder` (2 calls) | `util/hnsw/hnsw_concurrent_merge_builder_test.go:94-297` | requires GOMAXPROCS >= 2 for meaningful concurrency |

### `suggest` (4 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestSuggestIntegration` (2 calls) | `suggest/suggest_integration_test.go:14-18` | NewWFSTCompletionLookup / NewAnalyzingSuggester not yet implemented |
| `TestPersistence` (2 calls) | `suggest/persistence_test.go:34-58` | TSTLookup.Store/Load / FSTCompletionLookup.Store/Load not yet implemented |

### `expressions` (4 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestExpressionValidation` | `expressions/expression_validation_test.go:15` | requires ANTLR JavascriptCompiler |
| `TestDemoExpressions` | `expressions/demo_expressions_test.go:18` | requires ANTLR JavascriptCompiler + IndexSearcher |
| `TestExpressionSorts` | `expressions/expression_sorts_test.go:15` | requires ANTLR JavascriptCompiler + IndexSearcher |
| `TestExpressionSortField` | `expressions/expression_sort_field_test.go:15` | requires ANTLR JavascriptCompiler + IndexSearcher |

### `misc/util/fst` (3 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestFSTsMisc` (3 calls) | `misc/util/fst/fsts_misc_test.go:24-37` | requires FSTTester / ListOfOutputs as full FST Outputs — not yet ported |

### `facets/taxonomy/directory` (3 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestConcurrentFacetedIndexing` | `facets/taxonomy/directory/test_concurrent_faceted_indexing_test.go:25` | requires IndexWriter + DirectoryTaxonomyWriter + ParallelTaxonomyArrays |
| `TestAlwaysRefreshDirectoryTaxonomyReader` (2 calls) | `facets/taxonomy/directory/test_always_refresh_directory_taxonomy_reader_test.go:27-33` | requires SearcherTaxonomyManager + DirectoryTaxonomyWriter snapshot/rollback |

### `util/fst` (2 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestTwoBFST` | `util/fst/twobfst_test.go:35` | monster test: ~3 GiB in-memory FSTs (GOC-4288) |
| `TestTwoBFSTOffHeap` | `util/fst/twobfst_off_heap_test.go:32` | monster test: ~3 GiB on-disk FSTs (GOC-4286) |

### `queryparser/flexible/spans` (2 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestSpans` (2 calls) | `queryparser/flexible/spans/spans_test.go:21-34` | deferred: requires SpanOrQuery/SpanTermQuery implementations |

### `queryparser` (2 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestMultiAnalyzer` | `queryparser/test_multi_analyzer_test.go:26` | deferred: requires generic Analyzer support + multi-token position handling |
| `TestMultiPhraseQueryParsing` | `queryparser/test_multi_phrase_query_parsing_test.go:21` | deferred: requires generic Analyzer support + MultiPhraseQuery |

### `queries/intervals` (2 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestComplexMatches` | `queries/intervals/complex_matches_test.go:16` | requires MatchesTestBase + full interval query execution; deferred |
| `TestPayloadFilteredInterval` | `queries/intervals/payload_filtered_interval_test.go:16` | requires MockTokenizer + RandomIndexWriter; deferred |

### `misc/store` (2 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestHardlinkCopyDirectoryWrapper` (2 calls) | `misc/store/hardlink_copy_directory_wrapper_test.go:64-146` | hardlinks not supported on this filesystem |

### `join` (2 deferred tests)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestBlockJoin_SimpleFilter` | `join/block_join_test.go:296` | blocked by PostingsEnum.Advance (rmp #4763) |
| `TestBlockJoin_MultiChildQueriesOfDiffParentLevels` | `join/block_join_test.go:910` | requires PrefixQuery + postings Advance fix (rmp #4760/#4763) |

### `util/automaton` (1 deferred test)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestMinimize` | `util/automaton/minimize_test.go:67` | huge minimize test in -short mode (Lucene @Nightly equivalent) |

### `store` (1 deferred test)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestMultiByteBuffersDirectory` | `store/multi_byte_buffers_directory_test.go:378` | requires chunked ByteBuffersDirectory constructor |

### `queryparser/xml` (1 deferred test)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestCoreParserTestIndexData` | `queryparser/xml/core_parser_test_index_data_test.go:23` | deferred: requires reuters21578.txt fixture + functional IndexWriter/DirectoryReader |

### `queryparser/util` (1 deferred test)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestQueryParserTestBase` | `queryparser/util/query_parser_test_base_test.go:28` | deferred: abstract base class port requires complete QueryParser + StandardQueryParser |

### `queries/function/docvalues` (1 deferred test)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestBoolValOfNumericDVs` | `queries/function/docvalues/bool_val_of_numeric_dvs_test.go:16` | requires indexed numeric DocValues + FunctionValues; deferred |

### `facets/taxonomywritercache` (1 deferred test)

| Test Function | File:Line | Blocker Reason |
|---------------|-----------|----------------|
| `TestTwoGBCharBlockArray` | `facets/taxonomywritercache/test_2gb_char_block_array_test.go:90` | @Monster: uses >2 GB of memory, deferred for explicit stress runs |

## Cross-Cutting Themes

The deferred blockers cluster around a few key missing infrastructure pieces:

1. **NRT (Near-Real-Time) reader** (`index`): `DirectoryReader.open(IndexWriter)` is the single most referenced blocker. Until this exists, writers cannot produce searchable readers.

2. **DeleteDocuments** (`index`): The DeleteDocuments(Term|Query) are no-op stubs. Without this, all delete-dependent features fail (forceMergeDeletes, soft deletes, mixed updates).

3. **MockDirectoryWrapper** (`index`, `util/bkd`): Fault injection, disk-full simulation, and file-size tracking are not ported, blocking many error-path tests.

4. **RandomIndexWriter + IndexSearcher** (`search`, `facets`, `codecs`, `memory`): These integration test helpers are not ported, blocking the entire search/spatial/facets test surface.

5. **CannedTokenStream** (`index`, `analysis`): Token injection infrastructure is missing, blocking payloads, offsets, and custom analyzer tests.

6. **GeoTestUtil / spatial query factories** (`search`): ~50+ calls blocked by missing LatLonPoint/LatLonShape/LatLonDocValuesField query factories and random geometry generators.

7. **Monster tests** (`index`, `util/bkd`, `util/fst`, `document`, `facets/taxonomywritercache`): ~25 tests are stubs for multi-hour/multi-GiB tests deferred behind `GOCENE_RUN_MONSTERS=1`.

8. **Full pipeline integration** (`facets`, `queries/*`, `suggest`, `expressions`, `memory`): Complete end-to-end pipelines not yet wired (facets taxonomy, span queries, function queries, suggesters, expressions).

9. **SegmentReader core readers** (`index`): Core reader integration (rmp #4) not yet wired for postings, doc values, norms, term vectors.

10. **IndexWriter merge pipeline** (`index`): ForceMerge/ForceMergeDeletes, merge scheduling, and merge configuration not yet fully implemented.

## Notes

- All `t.Skip()` calls have been eliminated from the codebase, in compliance with the no-skip policy.
- Each deferred test uses `t.Fatal()` with a descriptive reason, making the failing test suite informative about what remains unimplemented.
- Blockers reference upstream Lucene 10.4.0 features, GOC ticket numbers, rmp task references, and Sprint numbers where available.
- The `index` and `search` packages account for ~72% of all deferred tests (478 of 660), primarily due to the missing NRT reader and spatial query integration.
