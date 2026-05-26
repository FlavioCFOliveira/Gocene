package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.TokenFilter;
import org.apache.lucene.analysis.TokenStream;
import org.apache.lucene.analysis.Tokenizer;
import org.apache.lucene.analysis.core.WhitespaceTokenizer;
import org.apache.lucene.analysis.tokenattributes.CharTermAttribute;
import org.apache.lucene.analysis.tokenattributes.PayloadAttribute;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.NumericDocValuesField;
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
import org.apache.lucene.queries.CommonTermsQuery;
import org.apache.lucene.queries.function.FunctionScoreQuery;
import org.apache.lucene.queries.intervals.IntervalQuery;
import org.apache.lucene.queries.intervals.Intervals;
import org.apache.lucene.queries.mlt.MoreLikeThis;
import org.apache.lucene.queries.payloads.AveragePayloadFunction;
import org.apache.lucene.queries.payloads.PayloadDecoder;
import org.apache.lucene.queries.payloads.PayloadScoreQuery;
import org.apache.lucene.queries.spans.SpanTermQuery;
import org.apache.lucene.search.BooleanClause;
import org.apache.lucene.search.DoubleValuesSource;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TopDocs;
import org.apache.lucene.search.similarities.BM25Similarity;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.util.BytesRef;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.StringReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Sprint 114 T11 (rmp 4619): {@code queries-hit-corpus}. Addresses the
 * queries audit row (verbatim): "No binary artefacts identified in
 * queries module beyond query-runtime state". Pins runtime hit/score
 * parity across the lucene-queries catalogue: CommonTermsQuery,
 * FunctionScoreQuery (via {@link DoubleValuesSource#fromLongField}),
 * MoreLikeThis seeded by doc-0, IntervalQuery via {@link Intervals#ordered},
 * and PayloadScoreQuery over a term with a deterministic 4-byte payload.
 */
public final class QueriesHitCorpusScenario implements CorpusScenario {

    public static final int NUM_DOCS = 20;
    public static final String TSV_NAME = "queries-hits.tsv";
    public static final String SCORE_FMT = "%.6f";

    private static final String FIELD_BODY = "body";
    private static final String FIELD_PAY = "pay";
    private static final String FIELD_COUNT = "count";
    private static final String PAY_TERM = "tkpay";

    /** Stable query catalogue. Order matches insertion in {@link #buildQueries}. */
    private static final List<String> QUERY_IDS = List.of(
            "common-terms",
            "function-score",
            "more-like-this",
            "interval-ordered",
            "payload-score");

    @Override
    public String name() {
        return "queries-hit-corpus";
    }

    @Override
    public String description() {
        return "queries module hit/score corpus: 20 docs + 5 queries + queries-hits.tsv";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer bodyAnalyzer = new org.apache.lucene.analysis.standard.StandardAnalyzer();
             Analyzer payAnalyzer = new PayloadAnalyzer(seed)) {
            Map<String, Analyzer> perField = new LinkedHashMap<>();
            perField.put(FIELD_PAY, payAnalyzer);
            Analyzer wrapper = new org.apache.lucene.analysis.miscellaneous.PerFieldAnalyzerWrapper(
                    bodyAnalyzer, perField);
            IndexWriterConfig iwc = new IndexWriterConfig(wrapper)
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
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                List<HitRow> rows = evaluate(reader);
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
        List<HitRow> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<HitRow> recomputed = evaluate(reader);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(name() + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                HitRow a = recorded.get(i);
                HitRow b = recomputed.get(i);
                if (!a.queryId.equals(b.queryId) || a.rank != b.rank || !a.docId.equals(b.docId)) {
                    throw new IOException(name() + ": row " + i + " key drift: " + a + " vs " + b);
                }
                if (Math.abs(a.score - b.score) > 1e-6) {
                    throw new IOException(name() + ": row " + i + " score drift: " + a + " vs " + b);
                }
            }
        }
    }

    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        doc.add(new StoredField("id", id));
        doc.add(new StringField("id", id, Field.Store.NO));
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        int repAlpha = (int) ((mix & 0x3) + 1);
        int repBeta = (int) (((mix >>> 2) & 0x3) + 1);
        StringBuilder body = new StringBuilder();
        for (int k = 0; k < repAlpha; k++) body.append("alpha ");
        for (int k = 0; k < repBeta; k++) body.append("beta ");
        body.append("gamma delta ");
        if ((i % 3) == 0) body.append("epsilon ");
        if ((i % 4) == 0) body.append("zeta ");
        body.append("the and of in to ").append("pivot-").append(i);
        doc.add(new TextField(FIELD_BODY, body.toString().trim(), Field.Store.YES));
        doc.add(new TextField(FIELD_PAY, PAY_TERM, Field.Store.NO));
        doc.add(new NumericDocValuesField(FIELD_COUNT, (long) (i + 1)));
        return doc;
    }

    private static List<HitRow> evaluate(DirectoryReader reader) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        Map<String, Query> queries = buildQueries(reader);
        List<HitRow> rows = new ArrayList<>();
        StoredFields sf = reader.storedFields();
        for (Map.Entry<String, Query> e : queries.entrySet()) {
            String qid = e.getKey();
            TopDocs top = searcher.search(e.getValue(), NUM_DOCS);
            for (int rank = 0; rank < top.scoreDocs.length; rank++) {
                ScoreDoc sd = top.scoreDocs[rank];
                String id = sf.document(sd.doc).get("id");
                rows.add(new HitRow(qid, rank, id, sd.score));
            }
        }
        rows.sort((a, b) -> {
            int c = a.queryId.compareTo(b.queryId);
            if (c != 0) return c;
            return Integer.compare(a.rank, b.rank);
        });
        return rows;
    }

    private static Map<String, Query> buildQueries(DirectoryReader reader) throws IOException {
        Map<String, Query> q = new LinkedHashMap<>();
        CommonTermsQuery ct = new CommonTermsQuery(
                BooleanClause.Occur.SHOULD, BooleanClause.Occur.SHOULD, 0.5f);
        ct.add(new Term(FIELD_BODY, "alpha"));
        ct.add(new Term(FIELD_BODY, "the"));
        ct.add(new Term(FIELD_BODY, "epsilon"));
        q.put("common-terms", ct);
        q.put("function-score", FunctionScoreQuery.boostByValue(
                new org.apache.lucene.search.TermQuery(new Term(FIELD_BODY, "gamma")),
                DoubleValuesSource.fromLongField(FIELD_COUNT)));
        MoreLikeThis mlt = new MoreLikeThis(reader);
        mlt.setAnalyzer(new org.apache.lucene.analysis.standard.StandardAnalyzer());
        mlt.setMinTermFreq(1);
        mlt.setMinDocFreq(1);
        mlt.setFieldNames(new String[]{FIELD_BODY});
        String doc0Body = reader.storedFields().document(0).get(FIELD_BODY);
        q.put("more-like-this", mlt.like(FIELD_BODY, new StringReader(doc0Body)));
        q.put("interval-ordered", new IntervalQuery(FIELD_BODY,
                Intervals.ordered(Intervals.term("gamma"), Intervals.term("delta"))));
        q.put("payload-score", new PayloadScoreQuery(
                new SpanTermQuery(new Term(FIELD_PAY, PAY_TERM)),
                new AveragePayloadFunction(), PayloadDecoder.FLOAT_DECODER, true));
        if (!q.keySet().equals(new java.util.LinkedHashSet<>(QUERY_IDS))) {
            throw new IllegalStateException("QUERY_IDS / buildQueries drift");
        }
        return q;
    }

    private static void writeTsv(Path file, List<HitRow> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_id\trank\tdoc_id\tscore (%.6f)\n");
        for (HitRow r : rows) {
            sb.append(r.queryId).append('\t').append(r.rank).append('\t')
                    .append(r.docId).append('\t')
                    .append(String.format(Locale.ROOT, SCORE_FMT, r.score)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    public static List<HitRow> readTsv(Path file) throws IOException {
        List<HitRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 4) {
                    throw new IOException("malformed row: " + line);
                }
                rows.add(new HitRow(cols[0], Integer.parseInt(cols[1]), cols[2],
                        Double.parseDouble(cols[3])));
            }
        }
        return rows;
    }

    /** Single TSV row. */
    public static final class HitRow {
        public final String queryId;
        public final int rank;
        public final String docId;
        public final double score;
        public HitRow(String queryId, int rank, String docId, double score) {
            this.queryId = queryId; this.rank = rank; this.docId = docId; this.score = score;
        }
        @Override public String toString() {
            return queryId + "#" + rank + "/" + docId + "@"
                    + String.format(Locale.ROOT, SCORE_FMT, score);
        }
    }

    /** Analyzer emitting a single token with a deterministic 4-byte payload. */
    private static final class PayloadAnalyzer extends Analyzer {
        private final long seed;
        PayloadAnalyzer(long seed) { this.seed = seed; }
        @Override protected TokenStreamComponents createComponents(String fieldName) {
            Tokenizer src = new WhitespaceTokenizer();
            return new TokenStreamComponents(src, new PayloadFilter(src, seed));
        }
    }

    private static final class PayloadFilter extends TokenFilter {
        private final CharTermAttribute termAtt = addAttribute(CharTermAttribute.class);
        private final PayloadAttribute payAtt = addAttribute(PayloadAttribute.class);
        private final long seed;
        PayloadFilter(TokenStream in, long seed) { super(in); this.seed = seed; }
        @Override public boolean incrementToken() throws IOException {
            if (!input.incrementToken()) return false;
            long mix = (seed * 0x9E3779B97F4A7C15L) ^ termAtt.toString().hashCode();
            byte[] p = new byte[4];
            // First byte forced into [1, 127] so PayloadDecoder.FLOAT_DECODER
            // (single-byte float cast) yields a strictly positive score.
            p[0] = (byte) (((mix & 0x7F) | 0x01));
            p[1] = (byte) ((mix >>> 8) & 0xFF);
            p[2] = (byte) ((mix >>> 16) & 0xFF);
            p[3] = (byte) ((mix >>> 24) & 0xFF);
            payAtt.setPayload(new BytesRef(p));
            return true;
        }
    }
}
