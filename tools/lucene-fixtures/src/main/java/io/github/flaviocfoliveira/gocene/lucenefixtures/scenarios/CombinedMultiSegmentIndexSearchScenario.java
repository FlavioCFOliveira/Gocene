package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.FieldType;
import org.apache.lucene.document.IntPoint;
import org.apache.lucene.document.KnnFloatVectorField;
import org.apache.lucene.document.NumericDocValuesField;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexOptions;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.index.Term;
import org.apache.lucene.index.VectorSimilarityFunction;
import org.apache.lucene.search.BooleanClause;
import org.apache.lucene.search.BooleanQuery;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.PhraseQuery;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TermQuery;
import org.apache.lucene.search.TopDocs;
import org.apache.lucene.search.similarities.BM25Similarity;
import org.apache.lucene.store.FSDirectory;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Sprint 114 T5 (rmp 4611), S1 {@code combined-multi-segment-index-search}.
 * 3 commits => 3 segments under NoMergePolicy; docs carry stored fields,
 * NumericDocValues, IntPoint, KnnFloatVectorField, term vectors and norms.
 * Runs 5 TermQuery + 2 PhraseQuery + 1 BooleanQuery and emits {@value #TSV_NAME}
 * sorted by (query_id asc, rank asc), scores formatted as {@value #SCORE_FMT}.
 */
public final class CombinedMultiSegmentIndexSearchScenario implements CorpusScenario {

    public static final String NAME = "combined-multi-segment-index-search";
    /** TSV emitted next to the index. */
    public static final String TSV_NAME = "s1-hits.tsv";
    /** Score formatting: 6 decimal digits, ROOT locale. */
    public static final String SCORE_FMT = "%.6f";
    /** Per-segment doc count. Three commits produce 3 segments. */
    public static final int DOCS_PER_SEGMENT = 6;
    /** Total documents indexed. */
    public static final int NUM_DOCS = DOCS_PER_SEGMENT * 3;
    /** KNN dimensionality. */
    public static final int VECTOR_DIM = 4;

    /** Stable query catalogue (kept in stack order). */
    public static final List<String> QUERY_IDS = List.of(
            "tq-alpha", "tq-beta", "tq-gamma", "tq-delta", "tq-epsilon",
            "ph-alpha-beta", "ph-gamma-delta",
            "bool-alpha-or-zeta");

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Multi-segment (3) Lucene index + 8-query catalogue; emits s1-hits.tsv.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setSimilarity(new BM25Similarity())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int seg = 0; seg < 3; seg++) {
                    for (int j = 0; j < DOCS_PER_SEGMENT; j++) {
                        int i = seg * DOCS_PER_SEGMENT + j;
                        writer.addDocument(buildDoc(i, seed));
                    }
                    writer.commit();
                }
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                if (reader.leaves().size() != 3) {
                    throw new IOException(NAME + ": expected exactly 3 segments, got "
                            + reader.leaves().size());
                }
                writeTsv(target.resolve(TSV_NAME), evaluate(reader));
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tsv = source.resolve(TSV_NAME);
        if (!Files.isRegularFile(tsv)) {
            throw new IOException(NAME + ": missing " + TSV_NAME);
        }
        List<Row> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<Row> recomputed = evaluate(reader);
            assertEqualRows(recorded, recomputed);
        }
    }

    /** Build a deterministic doc with every audited per-doc field type.
     *  Package-private so {@link CombinedReverseIndexSearchScenario} can
     *  reuse the exact same doc shape (S2 indexes the SAME doc set into a
     *  single segment). */
    static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        doc.add(new StoredField("id", id));
        doc.add(new StringField("id", id, Field.Store.NO));
        doc.add(new NumericDocValuesField("rank_dv", (long) i));
        doc.add(new IntPoint("rank_pt", i));
        // KNN vector: 4-D, deterministic from (seed, i).
        float[] vec = new float[VECTOR_DIM];
        for (int k = 0; k < VECTOR_DIM; k++) {
            // Small bounded floats; deterministic; non-zero norm.
            vec[k] = (float) (((mix >>> (k * 8)) & 0xFF) / 255.0) + 1e-3f;
        }
        doc.add(new KnnFloatVectorField("vec", vec, VectorSimilarityFunction.EUCLIDEAN));
        // Body field with term-vectors enabled.
        int repAlpha = (int) ((mix & 0x3) + 1);
        int repBeta = (int) (((mix >>> 2) & 0x3) + 1);
        StringBuilder body = new StringBuilder();
        for (int k = 0; k < repAlpha; k++) body.append("alpha ");
        for (int k = 0; k < repBeta; k++) body.append("beta ");
        body.append("gamma delta ");
        if ((i % 3) == 0) body.append("epsilon ");
        if ((i % 4) == 0) body.append("zeta ");
        body.append("pivot ").append(id);
        doc.add(new Field("body", body.toString().trim(), bodyType()));
        return doc;
    }

    private static FieldType bodyType() {
        FieldType ft = new FieldType(TextField.TYPE_NOT_STORED);
        ft.setIndexOptions(IndexOptions.DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS);
        ft.setStoreTermVectors(true);
        ft.setStoreTermVectorPositions(true);
        ft.setStoreTermVectorOffsets(true);
        ft.freeze();
        return ft;
    }

    /** Build the fixed query catalogue. {@link LinkedHashMap} preserves order.
     *  Package-private so the reverse-index scenario reuses the same catalogue. */
    static Map<String, Query> buildQueries() {
        Map<String, Query> q = new LinkedHashMap<>();
        q.put("tq-alpha", new TermQuery(new Term("body", "alpha")));
        q.put("tq-beta", new TermQuery(new Term("body", "beta")));
        q.put("tq-gamma", new TermQuery(new Term("body", "gamma")));
        q.put("tq-delta", new TermQuery(new Term("body", "delta")));
        q.put("tq-epsilon", new TermQuery(new Term("body", "epsilon")));
        q.put("ph-alpha-beta", new PhraseQuery("body", "alpha", "beta"));
        q.put("ph-gamma-delta", new PhraseQuery("body", "gamma", "delta"));
        q.put("bool-alpha-or-zeta", new BooleanQuery.Builder()
                .add(new TermQuery(new Term("body", "alpha")), BooleanClause.Occur.SHOULD)
                .add(new TermQuery(new Term("body", "zeta")), BooleanClause.Occur.SHOULD)
                .build());
        if (!q.keySet().equals(new java.util.LinkedHashSet<>(QUERY_IDS))) {
            throw new IllegalStateException("QUERY_IDS / buildQueries drift");
        }
        return q;
    }

    /** Evaluate the fixed catalogue under BM25, return rows sorted by (qid, rank).
     *  Package-private — see {@link CombinedReverseIndexSearchScenario}. */
    static List<Row> evaluate(DirectoryReader reader) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        StoredFields sf = reader.storedFields();
        List<Row> rows = new ArrayList<>();
        for (Map.Entry<String, Query> entry : buildQueries().entrySet()) {
            String qid = entry.getKey();
            TopDocs top = searcher.search(entry.getValue(), NUM_DOCS);
            for (int rank = 0; rank < top.scoreDocs.length; rank++) {
                ScoreDoc sd = top.scoreDocs[rank];
                String id = sf.document(sd.doc).get("id");
                rows.add(new Row(qid, rank, id, sd.score));
            }
        }
        rows.sort((a, b) -> {
            int c = a.queryId().compareTo(b.queryId());
            if (c != 0) return c;
            return Integer.compare(a.rank(), b.rank());
        });
        return rows;
    }

    static void writeTsv(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_id\trank\tdoc_id\tscore\n");
        for (Row r : rows) {
            sb.append(r.queryId()).append('\t').append(r.rank()).append('\t').append(r.docId())
                    .append('\t').append(String.format(Locale.ROOT, SCORE_FMT, r.score()))
                    .append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    static List<Row> readTsv(Path file) throws IOException {
        List<Row> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 4) {
                    throw new IOException("malformed row: " + line);
                }
                rows.add(new Row(cols[0], Integer.parseInt(cols[1]), cols[2],
                        Double.parseDouble(cols[3])));
            }
        }
        return rows;
    }

    /** Compare two row lists at ±1e-6 tolerance on score; identity on keys. */
    static void assertEqualRows(List<Row> a, List<Row> b) throws IOException {
        if (a.size() != b.size()) {
            throw new IOException(NAME + ": row count drift recorded=" + a.size()
                    + " recomputed=" + b.size());
        }
        for (int i = 0; i < a.size(); i++) {
            Row x = a.get(i);
            Row y = b.get(i);
            if (!x.queryId().equals(y.queryId()) || x.rank() != y.rank()
                    || !x.docId().equals(y.docId())) {
                throw new IOException(NAME + ": row " + i + " key drift: " + x + " vs " + y);
            }
            if (Math.abs(x.score() - y.score()) > 1e-6) {
                throw new IOException(NAME + ": row " + i + " score drift: " + x + " vs " + y);
            }
        }
    }

    /** Single TSV row. */
    public record Row(String queryId, int rank, String docId, double score) {
        @Override public String toString() {
            return queryId + "#" + rank + "/" + docId + "@"
                    + String.format(Locale.ROOT, SCORE_FMT, score);
        }
    }
}
