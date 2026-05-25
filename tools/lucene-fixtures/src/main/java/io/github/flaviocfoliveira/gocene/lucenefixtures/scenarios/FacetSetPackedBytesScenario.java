package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.facet.FacetResult;
import org.apache.lucene.facet.FacetsCollector;
import org.apache.lucene.facet.FacetsCollectorManager;
import org.apache.lucene.facet.LabelAndValue;
import org.apache.lucene.facet.facetset.ExactFacetSetMatcher;
import org.apache.lucene.facet.facetset.FacetSetDecoder;
import org.apache.lucene.facet.facetset.FacetSetsField;
import org.apache.lucene.facet.facetset.LongFacetSet;
import org.apache.lucene.facet.facetset.MatchingFacetSetsCounts;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.MatchAllDocsQuery;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T12 (rmp 4620): {@code facet-set-packed-bytes}. Addresses
 * the facets audit row (verbatim): "No Lucene-produced FacetSet bytes
 * used in tests". Indexes {@value #NUM_DOCS} docs each carrying two
 * 3-dim {@link LongFacetSet}s packed via {@link FacetSetsField}; verifies
 * the anchor tuple count via {@link MatchingFacetSetsCounts} +
 * {@link ExactFacetSetMatcher} (decode path uses
 * {@link FacetSetDecoder#decodeLongs}).
 */
public final class FacetSetPackedBytesScenario implements CorpusScenario {

    public static final String NAME = "facet-set-packed-bytes";
    public static final int NUM_DOCS = 6;
    public static final String FIELD = "fset";
    /** Number of long dimensions per packed FacetSet (matches LongFacetSet ctor arity). */
    public static final int DIMS = 3;
    /** Canonical packed tuple every doc carries (anchors the matcher count). */
    private static final long[] ANCHOR = {1L, 2L, 3L};

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "FacetSet packed-bytes: " + DIMS + "-dim LongFacetSet via FacetSetsField; "
                + "verify via MatchingFacetSetsCounts + ExactFacetSetMatcher.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                // Two LongFacetSets per doc: anchor (every doc — pins the matcher
                // count) + seed-derived noise (exposes the packed-bytes layout
                // to byte-determinism checks).
                for (int i = 0; i < NUM_DOCS; i++) {
                    Document d = new Document();
                    d.add(new StringField("id", "fs-" + i, Field.Store.YES));
                    LongFacetSet anchor = new LongFacetSet(ANCHOR[0], ANCHOR[1], ANCHOR[2]);
                    d.add(FacetSetsField.create(FIELD, anchor, seededSet(seed, i)));
                    writer.addDocument(d);
                }
                writer.commit();
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             IndexReader reader = DirectoryReader.open(dir)) {
            if (reader.numDocs() != NUM_DOCS) {
                throw new IOException(NAME + ": numDocs mismatch, got "
                        + reader.numDocs() + " expected " + NUM_DOCS);
            }
            IndexSearcher searcher = new IndexSearcher(reader);
            FacetsCollector fc =
                    searcher.search(new MatchAllDocsQuery(), new FacetsCollectorManager());
            MatchingFacetSetsCounts counts = new MatchingFacetSetsCounts(
                    FIELD, fc, FacetSetDecoder::decodeLongs,
                    new ExactFacetSetMatcher("anchor",
                            new LongFacetSet(ANCHOR[0], ANCHOR[1], ANCHOR[2])));
            FacetResult result = counts.getAllChildren(FIELD);
            if (result == null) {
                throw new IOException(NAME + ": MatchingFacetSetsCounts returned null result");
            }
            if (result.childCount != 1) {
                throw new IOException(NAME + ": expected childCount=1, got "
                        + result.childCount);
            }
            LabelAndValue lv = result.labelValues[0];
            if (!"anchor".equals(lv.label)) {
                throw new IOException(NAME + ": label drift, got '" + lv.label + "'");
            }
            if (lv.value.intValue() != NUM_DOCS) {
                throw new IOException(NAME + ": anchor count=" + lv.value
                        + " want " + NUM_DOCS);
            }
        }
    }

    /** Deterministic 3-dim noise tuple for {@code (seed, doc)}. */
    public static LongFacetSet seededSet(long seed, int doc) {
        long base = (seed * 0x9E3779B97F4A7C15L) ^ ((long) doc * 17L + 3L);
        // Force every dimension > ANCHOR so it never collides with the matcher.
        long d0 = 1000L + Math.abs(base % 9000L);
        long d1 = 1000L + Math.abs((base * 31L) % 9000L);
        long d2 = 1000L + Math.abs((base * 131L) % 9000L);
        return new LongFacetSet(d0, d1, d2);
    }
}
