package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.KnnFloatVectorField;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.StringField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.KnnFloatVectorQuery;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TopDocs;
import org.apache.lucene.store.FSDirectory;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

/**
 * Sprint 114 T9 (rmp 4617): {@code knn-hit-ordering}.
 *
 * <p>Addresses audit row (verbatim): "HNSW bytes in fixture exist but no
 * end-to-end search verifies identical hit ordering vs Lucene".
 *
 * <p>Builds a small {@link KnnFloatVectorField} corpus of {@value #NUM_DOCS}
 * docs at {@value #DIM} dimensions, then issues {@link #NUM_QUERIES} KNN
 * searches (k={@value #TOP_K}) with deterministically derived query
 * vectors and writes the resulting (rank, doc_id, score) tuples to
 * {@code knn-hits.tsv}. HNSW indexing is deterministic under
 * {@link Determinism#seed(long)} + the standard {@link NoMergePolicy} +
 * {@link SerialMergeScheduler} configuration this harness uses for every
 * codec scenario.
 */
public final class KnnHitOrderingScenario implements CorpusScenario {

    /** Number of documents indexed. */
    public static final int NUM_DOCS = 30;
    /** Vector dimension. */
    public static final int DIM = 4;
    /** k for the KNN search. */
    public static final int TOP_K = 5;
    /** Number of query vectors evaluated. */
    public static final int NUM_QUERIES = 3;

    /** TSV filename written next to the Lucene index inside the scenario directory. */
    public static final String TSV_NAME = "knn-hits.tsv";
    /** Score format string. */
    public static final String SCORE_FMT = "%.6f";

    @Override
    public String name() {
        return "knn-hit-ordering";
    }

    @Override
    public String description() {
        return "HNSW KNN top-k ordering: 30 docs, dim=4, 3 queries, k=5 (audit: identical hit ordering vs Lucene)";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             StandardAnalyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    writer.addDocument(buildDoc(i, seed));
                }
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                List<KnnRow> rows = evaluate(reader, seed);
                writeTsv(target.resolve(TSV_NAME), rows);
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tsv = source.resolve(TSV_NAME);
        if (!Files.exists(tsv)) {
            throw new IOException(name() + ": missing " + TSV_NAME);
        }
        List<KnnRow> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<KnnRow> recomputed = evaluate(reader, seed);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(name() + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                KnnRow a = recorded.get(i);
                KnnRow b = recomputed.get(i);
                if (!a.queryId.equals(b.queryId) || a.rank != b.rank || !a.docId.equals(b.docId)) {
                    throw new IOException(name() + ": row " + i + " key drift: "
                            + a + " vs " + b);
                }
                if (Math.abs(a.score - b.score) > 1e-6) {
                    throw new IOException(name() + ": row " + i + " score drift: "
                            + a + " vs " + b);
                }
            }
        }
    }

    /** Build the i-th document with a deterministic vector. */
    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        doc.add(new StoredField("id", id));
        doc.add(new StringField("id", id, Field.Store.NO));
        doc.add(new KnnFloatVectorField("vec", vectorFor(i, seed)));
        return doc;
    }

    /** Deterministic vector: derived from (i, seed) so each vector is distinct and non-zero. */
    private static float[] vectorFor(int i, long seed) {
        float[] v = new float[DIM];
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ ((long) i << 16);
        for (int k = 0; k < DIM; k++) {
            long m = mix ^ ((long) k * 0xBF58476D1CE4E5B9L);
            // Map to [0.01, 1.01) to avoid the all-zero vector pathology.
            v[k] = (float) (((m >>> 32) & 0xFFFF) / 65535.0 + 0.01);
        }
        return v;
    }

    /**
     * Fixed query vector catalogue. Independent of {@code seed} so the
     * scenario's {@link #verify(Path, long)} can re-evaluate the same
     * queries regardless of which seed produced the index (the seed only
     * stamps StringHelper.nextId() and the synthetic doc vectors; the
     * query catalogue is part of the scenario contract).
     */
    private static final float[][] QUERY_VECTORS = {
            {0.10f, 0.20f, 0.30f, 0.40f}, // q-0: a fixed reference vector
            {0.90f, 0.05f, 0.50f, 0.25f}, // q-1: a contrasting vector
            {0.50f, 0.50f, 0.50f, 0.50f}, // q-2: the unit-mean vector
    };

    private static float[] queryVectorFor(int q, long seed) {
        return QUERY_VECTORS[q];
    }

    /** Run KNN searches and return the flat list of (queryId, rank, docId, score). */
    private static List<KnnRow> evaluate(DirectoryReader reader, long seed) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        List<KnnRow> rows = new ArrayList<>();
        StoredFields sf = reader.storedFields();
        for (int q = 0; q < NUM_QUERIES; q++) {
            String qid = "q-" + q;
            float[] qv = queryVectorFor(q, seed);
            TopDocs top = searcher.search(new KnnFloatVectorQuery("vec", qv, TOP_K), TOP_K);
            for (int rank = 0; rank < top.scoreDocs.length; rank++) {
                ScoreDoc sd = top.scoreDocs[rank];
                String id = sf.document(sd.doc).get("id");
                rows.add(new KnnRow(qid, rank, id, sd.score));
            }
        }
        return rows;
    }

    private static void writeTsv(Path file, List<KnnRow> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_id\trank\tdoc_id\tscore (HNSW, %.6f)\n");
        for (KnnRow r : rows) {
            sb.append(r.queryId).append('\t').append(r.rank).append('\t')
                    .append(r.docId).append('\t')
                    .append(String.format(Locale.ROOT, SCORE_FMT, r.score)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    /** Parse a TSV file written by {@link #writeTsv}. */
    public static List<KnnRow> readTsv(Path file) throws IOException {
        List<KnnRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 4) {
                    throw new IOException("malformed row: " + line);
                }
                rows.add(new KnnRow(cols[0], Integer.parseInt(cols[1]), cols[2],
                        Double.parseDouble(cols[3])));
            }
        }
        return rows;
    }

    /** Single TSV row: (queryId, rank, docId, score). */
    public static final class KnnRow {
        public final String queryId;
        public final int rank;
        public final String docId;
        public final double score;
        public KnnRow(String queryId, int rank, String docId, double score) {
            this.queryId = queryId; this.rank = rank; this.docId = docId; this.score = score;
        }
        @Override public String toString() {
            return queryId + "#" + rank + "/" + docId + "@"
                    + String.format(Locale.ROOT, SCORE_FMT, score);
        }
    }
}
