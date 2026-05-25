package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.facet.FacetsConfig;
import org.apache.lucene.facet.sortedset.DefaultSortedSetDocValuesReaderState;
import org.apache.lucene.facet.sortedset.SortedSetDocValuesFacetField;
import org.apache.lucene.facet.sortedset.SortedSetDocValuesReaderState;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T12 (rmp 4620): {@code facet-sortedset-ords}. Addresses
 * the facets audit row (verbatim): "No Lucene-emitted sorted-set ord
 * file consumed by tests". Indexes {@value #NUM_DOCS} docs over two
 * dim/value pairs via {@link SortedSetDocValuesFacetField}; verifies the
 * resulting {@link DefaultSortedSetDocValuesReaderState} exposes a
 * non-empty {@link SortedSetDocValuesReaderState.OrdRange} for each dim.
 */
public final class FacetSortedsetOrdsScenario implements CorpusScenario {

    public static final String NAME = "facet-sortedset-ords";
    public static final int NUM_DOCS = 6;
    public static final String DIM_A = "color";
    public static final String DIM_B = "size";
    public static final String[] VALUES_A = {"red", "green", "blue"};
    public static final String[] VALUES_B = {"s", "m", "l"};

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "SortedSetDocValuesFacetField for two dim/value pairs; "
                + "verify ord encoding via DefaultSortedSetDocValuesReaderState.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        FacetsConfig config = new FacetsConfig();
        // SortedSetDocValuesFacetField uses the default field name; no setup needed.
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    Document d = new Document();
                    d.add(new StringField("id", "ss-" + i, Field.Store.YES));
                    d.add(new SortedSetDocValuesFacetField(DIM_A, valueA(seed, i)));
                    d.add(new SortedSetDocValuesFacetField(DIM_B, valueB(seed, i)));
                    writer.addDocument(config.build(d));
                }
                writer.commit();
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        FacetsConfig config = new FacetsConfig();
        try (FSDirectory dir = FSDirectory.open(source);
             IndexReader reader = DirectoryReader.open(dir)) {
            if (reader.numDocs() != NUM_DOCS) {
                throw new IOException(NAME + ": numDocs mismatch, got "
                        + reader.numDocs() + " expected " + NUM_DOCS);
            }
            DefaultSortedSetDocValuesReaderState state =
                    new DefaultSortedSetDocValuesReaderState(reader, config);
            checkDim(state, DIM_A);
            checkDim(state, DIM_B);
        }
    }

    private static void checkDim(DefaultSortedSetDocValuesReaderState state, String dim)
            throws IOException {
        SortedSetDocValuesReaderState.OrdRange range = state.getOrdRange(dim);
        if (range == null) {
            throw new IOException(NAME + ": no OrdRange registered for dim '" + dim + "'");
        }
        if (range.end() < range.start()) {
            throw new IOException(NAME + ": empty OrdRange for dim '" + dim
                    + "' [" + range.start() + ".." + range.end() + "]");
        }
    }

    /** Deterministic value-A pick for {@code (seed, i)}. */
    public static String valueA(long seed, int i) {
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        return VALUES_A[(int) (Math.floorMod(mix, VALUES_A.length))];
    }

    /** Deterministic value-B pick for {@code (seed, i)}. */
    public static String valueB(long seed, int i) {
        long mix = (seed * 0xBF58476D1CE4E5B9L) ^ ((long) i * 31L);
        return VALUES_B[(int) (Math.floorMod(mix, VALUES_B.length))];
    }
}
