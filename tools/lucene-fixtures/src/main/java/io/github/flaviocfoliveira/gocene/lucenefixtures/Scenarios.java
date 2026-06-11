package io.github.flaviocfoliveira.gocene.lucenefixtures;

import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.AnalyzingInfixSidecarScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.BlockPackedWriterScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.BwcBigEndianStoreScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.BwcPacked64LegacyScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.DirectMonotonicBlockPackedScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PackedIntsPacked64Scenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.ClassifierLabelCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.DocumentPointsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.DocumentShapeDocValuesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.DocumentRangeDocValuesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CombinedFacetsSearchScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CombinedHighlightQueryparserAnalysisScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CombinedMultiSegmentIndexSearchScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CombinedReplicatorRoundtripScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CombinedReverseIndexSearchScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CombinedSuggesterFstScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.Completion104PostingsScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CompletionFstScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CompoundFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CompressingStoredFieldsScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.DocValuesFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.ExpressionsEvalCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FacetAssociationPayloadScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FacetSetPackedBytesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FacetSortedsetOrdsScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FastVectorHighlightPhrasesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FieldInfosFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FstBlobScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.GroupingResultCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.HighlightOffsetCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.IndexCorruptionScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.IndexDeletionsAndDvUpdatesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.KnnHitOrderingScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.KnnVectorsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.LiveDocsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.MiscHighfreqTermsCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.MemoryIndexFlushScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.MiscIndexSplitterInputScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.MonitorIndexSegmentScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.MonitorQueryBlobScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.NormsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.ParentBlockCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PerFieldDispatchScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PointsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PostingsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.QueriesHitCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.QueryparserTreesAndHitsScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SandboxIdversionPostingsScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.GeoEncodedPointsScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.ReplicatorNrtCopyStateScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.ScalarQuantizedKnnScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.Spatial3dSerializableScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SpatialBboxDvScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SpatialCompositeScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SpatialPrefixTreeScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SpatialSerializedDvShapeScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SpatialWktGeojsonScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SearchScoringCorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SegmentInfoFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SmokeScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SoftDeletesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.StorePrimitivesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.StoredFieldsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SynonymFstScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.TaxonomyDirectoryScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.TermVectorsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.TokenPayloadScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.WfstScenario;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Registry of binary-compatibility scenarios.
 *
 * <p>Sprint 114 T2 shipped the smoke scenario. T3 adds the foundational format
 * scenarios listed below. Further per-package tasks (T5..Tn) plug additional
 * scenarios in via {@link #register(CorpusScenario)}.
 */
public final class Scenarios {

    private static final Map<String, CorpusScenario> REGISTRY = new LinkedHashMap<>();

    static {
        register(new SmokeScenario());
        register(new StorePrimitivesScenario());
        register(new PostingsFormatScenario());
        register(new DocValuesFormatScenario());
        register(new StoredFieldsFormatScenario());
        register(new TermVectorsFormatScenario());
        register(new NormsFormatScenario());
        register(new PointsFormatScenario());
        register(new KnnVectorsFormatScenario());
        register(new CompoundFormatScenario());
        register(new FieldInfosFormatScenario());
        register(new SegmentInfoFormatScenario());
        register(new LiveDocsFormatScenario());
        register(new FstBlobScenario());
        // Sprint 114 T7 (rmp 4615): three new codec-specific scenarios.
        register(new PerFieldDispatchScenario());
        register(new CompressingStoredFieldsScenario());
        register(new ScalarQuantizedKnnScenario());
        // Sprint 114 T8 (rmp 4616): two new index-package scenarios.
        // Appended at the end of the registration order so existing
        // baseline.tsv rows keep their position.
        register(new IndexDeletionsAndDvUpdatesScenario());
        register(new IndexCorruptionScenario());
        // T8 helper: soft-deletes coverage for the soft-deletes audit row.
        register(new SoftDeletesScenario());
        // Sprint 114 T9 (rmp 4617): search-side numerical-parity scenarios.
        // Appended at the end so existing baseline.tsv rows keep their
        // index-order anchors.
        register(new SearchScoringCorpusScenario());
        register(new KnnHitOrderingScenario());
        // Sprint 114 T10 (rmp 4618): analysis-side scenarios. Appended in
        // stack order — after the search-* scenarios — so the manifest
        // ordering for prior rows is preserved.
        register(new SynonymFstScenario());
        register(new TokenPayloadScenario());
        // Sprint 114 T11 (rmp 4619): queries-module hit/score corpus. Appended
        // after the search-* and analysis-* scenarios so existing baseline.tsv
        // row positions remain stable.
        register(new QueriesHitCorpusScenario());
        // Sprint 114 T12 (rmp 4620): facets-module scenarios. Appended in
        // stack order — after the queries-* scenario — so the manifest
        // ordering for prior rows is preserved.
        register(new TaxonomyDirectoryScenario());
        register(new FacetAssociationPayloadScenario());
        register(new FacetSortedsetOrdsScenario());
        register(new FacetSetPackedBytesScenario());
        // Sprint 114 T13 (rmp 4621): suggest-module scenarios. Appended in
        // stack order — after the facets-* scenarios — so the manifest
        // ordering for prior rows is preserved.
        register(new CompletionFstScenario());
        register(new WfstScenario());
        register(new AnalyzingInfixSidecarScenario());
        register(new Completion104PostingsScenario());
        // Sprint 114 T14 (rmp 4622): highlight-module scenarios. Appended in
        // stack order — after the suggest-* scenarios — so the manifest
        // ordering for prior rows is preserved.
        register(new HighlightOffsetCorpusScenario());
        register(new FastVectorHighlightPhrasesScenario());
        // Sprint 114 T15 (rmp 4623): join-module parent-block corpus. Appended
        // in stack order — after the highlight-* scenarios — so the manifest
        // ordering for prior rows is preserved.
        register(new ParentBlockCorpusScenario());
        // Sprint 114 T16 (rmp 4624): grouping-module result corpus. Appended
        // in stack order — after the join-* scenario — so the manifest
        // ordering for prior rows is preserved.
        register(new GroupingResultCorpusScenario());
        // Sprint 114 T17 (rmp 4625): classification cross-engine label
        // corpus. Appended in stack order — after the grouping-* scenario —
        // so the manifest ordering for prior rows is preserved.
        register(new ClassifierLabelCorpusScenario());
        // Sprint 114 T18 (rmp 4626): monitor-module scenarios. Appended in
        // stack order — after the classification scenario — so the
        // manifest ordering for prior rows is preserved.
        register(new MonitorQueryBlobScenario());
        register(new MonitorIndexSegmentScenario());
        // Sprint 114 T19 (rmp 4627): replicator NRT CopyState wire frame.
        // Appended in stack order — after the monitor-* scenarios — so the
        // manifest ordering for prior rows is preserved. The two remaining
        // replicator audit rows (HTTP frames, session/revision) are tracked
        // as DEFERRED_ROWS in Manifest.java because Lucene 10.4.0 removed
        // the HttpReplicator / SessionToken / RevisionFile production surface.
        register(new ReplicatorNrtCopyStateScenario());
        // Sprint 114 T20 (rmp 4628): spatial / spatial3d / geo scenarios.
        // Appended in stack order — after the replicator-* scenario — so
        // the manifest ordering for prior rows is preserved. Stack order
        // mirrors the rmp 4628 deliverable list: SerializedDV, prefix-tree,
        // composite, BBox DV, WKT/GeoJSON, spatial3d, geo encoded points.
        register(new SpatialSerializedDvShapeScenario());
        register(new SpatialPrefixTreeScenario());
        register(new SpatialCompositeScenario());
        register(new SpatialBboxDvScenario());
        register(new SpatialWktGeojsonScenario());
        register(new Spatial3dSerializableScenario());
        register(new GeoEncodedPointsScenario());
        // Sprint 114 T21 (rmp 4629): expressions JavaScript-compiled eval
        // corpus. Appended in stack order — after the spatial-* scenarios
        // — so the manifest ordering for prior rows is preserved. The
        // round-trip leg is currently deferred on the Gocene side because
        // Lucene compiles JavaScript to JVM bytecode (no on-disk artefact
        // exists) and Gocene's port does not produce JVM bytecode.
        register(new ExpressionsEvalCorpusScenario());
        // Sprint 114 T22 (rmp 4630): queryparser trees + hits across the
        // six lucene-queryparser parsers (classic, complex-phrase, surround,
        // flexible, simple, ext). Appended in stack order — after the
        // expressions-* scenario — so the manifest ordering for prior rows
        // is preserved.
        register(new QueryparserTreesAndHitsScenario());
        // Sprint 114 T23 (rmp 4631): sandbox-module scenarios. Appended in
        // stack order — after the queryparser-* scenario — so the manifest
        // ordering for prior rows is preserved. The sandbox quantization
        // audit row is tracked as a DEFERRED row in Manifest.java because
        // Lucene 10.4.0 sandbox/codecs/quantization ships only KMeans and
        // SampleReader (no on-disk format).
        register(new SandboxIdversionPostingsScenario());
        // Sprint 114 T24 (rmp 4632): misc-module scenarios appended in
        // stack order. misc-index-splitter-input feeds both IndexSplitter
        // and IndexMergeTool tests; misc-highfreq-terms-corpus pins the
        // HighFreqTerms tool's logical output as a deterministic TSV.
        register(new MiscIndexSplitterInputScenario());
        register(new MiscHighfreqTermsCorpusScenario());
        // Sprint 114 T25 (rmp 4633): memory-module scenario. Appended in
        // stack order — after the misc-* scenarios — so existing baseline.tsv
        // rows keep their positions. Addresses the memory audit row
        // gap_notes: "No persisted binary artefact; gap is the absence of
        // byte-for-byte parity tests vs Lucene MemoryIndex internal layout
        // (where applicable to merges)."
        register(new MemoryIndexFlushScenario());
        // Sprint 114 T26 (rmp 4634): backward_codecs scenarios. Only two
        // audit rows have a writable surface in Lucene 10.4.0's
        // lucene-backward-codecs jar: legacy packed (LegacyDirectWriter)
        // and the big-endian store wrapper (EndiannessReverserUtil).
        // The remaining seven audit rows are read-only formats in 10.4 —
        // every per-version codec (Lucene70SegmentInfoFormat,
        // Lucene90HnswVectorsFormat, Lucene99PostingsFormat, Lucene99
        // ScalarQuantizedVectorsFormat, Lucene103PostingsFormat,
        // Lucene40BlockTreeTermsReader) and the multi-version corpora row
        // throw UnsupportedOperationException from their write paths and
        // are tracked as DEFERRED_ROWS in Manifest.java.
        register(new BwcPacked64LegacyScenario());
        // Sprint 8 T18 (rmp 140): PackedInts byte-compat fixture expansion.
        // Produce raw byte-level fixtures for core util.packed formats
        // that Gocene compares against for byte-for-byte compat.
        register(new PackedIntsPacked64Scenario());
        register(new BlockPackedWriterScenario());
        register(new DirectMonotonicBlockPackedScenario());
        register(new BwcBigEndianStoreScenario());
        // Sprint 114 T5 (rmp 4611): six combined end-to-end scenarios.
        // Appended in stack order at the very end so existing baseline.tsv
        // rows keep their positions. Each scenario composes >=2 audited
        // subsystems and emits a deterministic TSV alongside its index.
        register(new CombinedMultiSegmentIndexSearchScenario());
        register(new CombinedReverseIndexSearchScenario());
        register(new CombinedFacetsSearchScenario());
        register(new CombinedReplicatorRoundtripScenario());
        register(new CombinedSuggesterFstScenario());
        register(new CombinedHighlightQueryparserAnalysisScenario());
        // Sprint 8 T20 (rmp 142): document-package binary-compat test scenarios.
        // Appended in stack order — after the combined scenarios — so the
        // manifest ordering for prior rows is preserved.
        register(new DocumentPointsFormatScenario());
        register(new DocumentShapeDocValuesScenario());
        register(new DocumentRangeDocValuesScenario());
    }

    private Scenarios() {}

    public static void register(CorpusScenario scenario) {
        if (REGISTRY.containsKey(scenario.name())) {
            throw new IllegalStateException("scenario already registered: " + scenario.name());
        }
        REGISTRY.put(scenario.name(), scenario);
    }

    public static CorpusScenario require(String name) {
        CorpusScenario s = REGISTRY.get(name);
        if (s == null) {
            throw new IllegalArgumentException("unknown scenario: " + name
                    + " (known: " + REGISTRY.keySet() + ")");
        }
        return s;
    }

    public static Map<String, CorpusScenario> all() {
        // Preserve insertion order so the CLI list / manifest / Makefile loop are stable.
        return Collections.unmodifiableMap(new LinkedHashMap<>(REGISTRY));
    }
}
