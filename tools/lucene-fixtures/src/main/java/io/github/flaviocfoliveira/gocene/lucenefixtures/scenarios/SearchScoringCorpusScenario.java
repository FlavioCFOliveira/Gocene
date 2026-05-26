package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.index.Term;
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
 * Sprint 114 T9 (rmp 4617): {@code search-scoring-corpus}.
 *
 * <p>Addresses audit row (verbatim): "No persisted search artefact; gap is
 * the absence of a numerical-parity corpus vs Lucene scores".
 *
 * <p>Drives a deterministic {@link IndexWriter} run of {@value #NUM_DOCS}
 * documents with a stored {@code id} and a {@code body} {@link TextField},
 * then evaluates a fixed query set under {@link BM25Similarity} with the
 * default {@code (k1=1.2, b=0.75)} parameters and emits a {@code
 * scoring.tsv} alongside the index. Rows are formatted as
 * {@code query_id\tdoc_id\tscore} with score in {@code "%.6f"} (Locale.ROOT)
 * for stable cross-run comparison. Ordering is: query_id ascending, then
 * score descending, then doc_id ascending.
 *
 * <p>The query set covers the three idiomatic shapes a search-side audit
 * must pin: 5 {@link TermQuery} hits over distinct {@code body} terms, 2
 * {@link PhraseQuery} hits, and 1 multi-clause {@link BooleanQuery}.
 */
public final class SearchScoringCorpusScenario implements CorpusScenario {

    /** Number of documents indexed. */
    public static final int NUM_DOCS = 12;

    /** TSV filename written next to the Lucene index inside the scenario directory. */
    public static final String TSV_NAME = "scoring.tsv";

    /** Score format string. Locale.ROOT keeps the decimal separator as '.'. */
    public static final String SCORE_FMT = "%.6f";

    /** Stable query catalogue. Indices map 1:1 with {@code query_id} in the TSV. */
    private static final List<String> QUERY_IDS = List.of(
            "tq-alpha", "tq-beta", "tq-gamma", "tq-delta", "tq-epsilon",
            "ph-alpha-beta", "ph-gamma-delta",
            "bool-alpha-or-zeta");

    @Override
    public String name() {
        return "search-scoring-corpus";
    }

    @Override
    public String description() {
        return "BM25 scoring corpus: 12 docs + 8 queries + scoring.tsv (audit: numerical-parity vs Lucene)";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             StandardAnalyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setSimilarity(new BM25Similarity())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    writer.addDocument(buildDoc(i, seed));
                }
            }
            // Now read the index and evaluate the query set.
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                List<ScoreRow> rows = evaluate(reader, seed);
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
        List<ScoreRow> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<ScoreRow> recomputed = evaluate(reader, seed);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(name() + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                ScoreRow a = recorded.get(i);
                ScoreRow b = recomputed.get(i);
                if (!a.queryId.equals(b.queryId) || !a.docId.equals(b.docId)) {
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

    /** Build the i-th document with a seed-derived body. */
    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        doc.add(new StoredField("id", id));
        doc.add(new StringField("id", id, Field.Store.NO));
        // Deterministic body: pin the vocabulary so query hits are predictable, but
        // vary multiplicity by (i, seed) so BM25 produces a spread of scores.
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        int repAlpha = (int) ((mix & 0x3) + 1);            // 1..4
        int repBeta = (int) (((mix >>> 2) & 0x3) + 1);     // 1..4
        StringBuilder body = new StringBuilder();
        for (int k = 0; k < repAlpha; k++) body.append("alpha ");
        for (int k = 0; k < repBeta; k++) body.append("beta ");
        body.append("gamma delta ");
        if ((i % 3) == 0) body.append("epsilon ");
        if ((i % 4) == 0) body.append("zeta ");
        // A unique pivot per document so TF varies across the corpus.
        body.append("pivot-").append(i);
        doc.add(new TextField("body", body.toString().trim(), Field.Store.NO));
        return doc;
    }

    /** Evaluate the fixed query set against {@code reader} under BM25. */
    private static List<ScoreRow> evaluate(DirectoryReader reader, long seed) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        Map<String, Query> queries = buildQueries();
        List<ScoreRow> rows = new ArrayList<>();
        for (Map.Entry<String, Query> entry : queries.entrySet()) {
            String qid = entry.getKey();
            TopDocs top = searcher.search(entry.getValue(), NUM_DOCS);
            StoredFields sf = reader.storedFields();
            for (ScoreDoc sd : top.scoreDocs) {
                String id = sf.document(sd.doc).get("id");
                rows.add(new ScoreRow(qid, id, sd.score));
            }
        }
        rows.sort((a, b) -> {
            int c = a.queryId.compareTo(b.queryId);
            if (c != 0) return c;
            c = Double.compare(b.score, a.score); // descending by score
            if (c != 0) return c;
            return a.docId.compareTo(b.docId);
        });
        return rows;
    }

    /** Build the fixed query set. {@link LinkedHashMap} preserves QUERY_IDS order. */
    private static Map<String, Query> buildQueries() {
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
        // Sanity: QUERY_IDS and q.keySet() must agree.
        if (!q.keySet().equals(new java.util.LinkedHashSet<>(QUERY_IDS))) {
            throw new IllegalStateException("QUERY_IDS / buildQueries() drift");
        }
        return q;
    }

    /** Write rows as TSV (no header; column meanings are stable and documented). */
    private static void writeTsv(Path file, List<ScoreRow> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_id\tdoc_id\tscore (BM25, %.6f)\n");
        for (ScoreRow r : rows) {
            sb.append(r.queryId).append('\t').append(r.docId).append('\t')
                    .append(String.format(Locale.ROOT, SCORE_FMT, r.score)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    /** Parse a TSV file written by {@link #writeTsv}. */
    public static List<ScoreRow> readTsv(Path file) throws IOException {
        List<ScoreRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 3) {
                    throw new IOException("malformed row: " + line);
                }
                rows.add(new ScoreRow(cols[0], cols[1], Double.parseDouble(cols[2])));
            }
        }
        return rows;
    }

    /** Single TSV row. Field-visible for terseness; record-style accessors not needed. */
    public static final class ScoreRow {
        public final String queryId;
        public final String docId;
        public final double score;
        public ScoreRow(String queryId, String docId, double score) {
            this.queryId = queryId; this.docId = docId; this.score = score;
        }
        @Override public String toString() {
            return queryId + "/" + docId + "@" + String.format(Locale.ROOT, SCORE_FMT, score);
        }
    }
}
