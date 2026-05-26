package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
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
import org.apache.lucene.queryparser.classic.QueryParser;
import org.apache.lucene.queryparser.complexPhrase.ComplexPhraseQueryParser;
import org.apache.lucene.queryparser.ext.ExtendableQueryParser;
import org.apache.lucene.queryparser.flexible.standard.StandardQueryParser;
import org.apache.lucene.queryparser.simple.SimpleQueryParser;
import org.apache.lucene.queryparser.surround.parser.QueryParserConstants;
import org.apache.lucene.queryparser.surround.query.BasicQueryFactory;
import org.apache.lucene.queryparser.surround.query.SrndQuery;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TopDocs;
import org.apache.lucene.search.similarities.BM25Similarity;
import org.apache.lucene.store.FSDirectory;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Sprint 114 T22 (rmp 4630): {@code queryparser-trees-and-hits}. Addresses
 * the queryparser audit row (verbatim): "No binary artefacts; behavioural
 * parity tested only via Gocene-internal cases". Parity is enforced by
 * emitting two TSVs per seed: the canonical {@link Query#toString()} for
 * every (parser_id, query_id) pair and the per-rank (doc_id, score) tuples
 * obtained by executing each parsed Query against a deterministic 20-doc
 * index. Verification re-parses every query, asserts {@code toString()}
 * matches the recorded tree, re-executes, and asserts hits within 1e-6.
 */
public final class QueryparserTreesAndHitsScenario implements CorpusScenario {

    public static final int NUM_DOCS = 20;
    public static final String TSV_TREES = "qp-trees.tsv";
    public static final String TSV_HITS = "qp-hits.tsv";
    public static final String SCORE_FMT = "%.6f";

    private static final String FIELD_BODY = "body";
    private static final String FIELD_ID = "id";

    /** Parser identifiers in stack order. */
    public static final List<String> PARSER_IDS = List.of(
            "classic", "complex-phrase", "surround", "flexible", "simple", "ext");

    @Override
    public String name() {
        return "queryparser-trees-and-hits";
    }

    @Override
    public String description() {
        return "queryparser trees + hits across classic/complex-phrase/surround/flexible/simple/ext";
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
                for (int i = 0; i < NUM_DOCS; i++) {
                    writer.addDocument(buildDoc(i, seed));
                }
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                Map<String, Query> queries = buildAll(analyzer);
                writeTreesTsv(target.resolve(TSV_TREES), queries);
                writeHitsTsv(target.resolve(TSV_HITS), reader, queries);
            }
        } catch (Exception e) {
            if (e instanceof IOException io) throw io;
            throw new IOException("qp scenario generate failed: " + e, e);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path trees = source.resolve(TSV_TREES);
        Path hits = source.resolve(TSV_HITS);
        if (!Files.exists(trees) || !Files.exists(hits)) {
            throw new IOException(name() + ": missing TSV(s) under " + source);
        }
        List<TreeRow> recTrees = readTreesTsv(trees);
        List<HitRow> recHits = readHitsTsv(hits);
        try (Analyzer analyzer = new StandardAnalyzer();
             FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            Map<String, Query> queries = buildAll(analyzer);
            // Re-parse / re-toString: assert every recorded tree matches.
            for (TreeRow r : recTrees) {
                Query q = queries.get(r.parserId + "/" + r.queryId);
                if (q == null) {
                    throw new IOException("verify: missing key " + r.parserId + "/" + r.queryId);
                }
                if (!q.toString().equals(r.parsedToString)) {
                    throw new IOException("verify: toString drift parser=" + r.parserId
                            + " query=" + r.queryId + " recorded=" + r.parsedToString
                            + " recomputed=" + q.toString());
                }
            }
            // Re-execute: assert every recorded hit matches within 1e-6.
            List<HitRow> recomputed = executeAll(reader, queries);
            if (recomputed.size() != recHits.size()) {
                throw new IOException("verify: hit row count drift recorded="
                        + recHits.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recHits.size(); i++) {
                HitRow a = recHits.get(i);
                HitRow b = recomputed.get(i);
                if (!a.parserId.equals(b.parserId) || !a.queryId.equals(b.queryId)
                        || a.rank != b.rank || !a.docId.equals(b.docId)) {
                    throw new IOException("verify: hit key drift row " + i + ": " + a + " vs " + b);
                }
                if (Math.abs(a.score - b.score) > 1e-6) {
                    throw new IOException("verify: hit score drift row " + i + ": " + a + " vs " + b);
                }
            }
        } catch (Exception e) {
            if (e instanceof IOException io) throw io;
            throw new IOException("qp scenario verify failed: " + e, e);
        }
    }

    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        doc.add(new StoredField(FIELD_ID, id));
        doc.add(new StringField(FIELD_ID, id, Field.Store.NO));
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        int repAlpha = (int) ((mix & 0x3) + 1);
        int repBeta = (int) (((mix >>> 2) & 0x3) + 1);
        StringBuilder body = new StringBuilder();
        for (int k = 0; k < repAlpha; k++) body.append("alpha ");
        for (int k = 0; k < repBeta; k++) body.append("beta ");
        body.append("gamma delta ");
        if ((i % 3) == 0) body.append("epsilon ");
        if ((i % 4) == 0) body.append("zeta ");
        body.append("the quick brown fox pivot-").append(i);
        doc.add(new TextField(FIELD_BODY, body.toString().trim(), Field.Store.YES));
        return doc;
    }

    /** Build the full grammar suite keyed by "parser_id/query_id". */
    private static Map<String, Query> buildAll(Analyzer analyzer) throws Exception {
        Map<String, Query> all = new LinkedHashMap<>();
        // classic
        QueryParser classic = new QueryParser(FIELD_BODY, analyzer);
        all.put("classic/term", classic.parse("alpha"));
        all.put("classic/bool-and", classic.parse("alpha AND beta"));
        all.put("classic/phrase", classic.parse("\"gamma delta\""));
        all.put("classic/prefix", classic.parse("alph*"));
        // complex-phrase
        ComplexPhraseQueryParser cp = new ComplexPhraseQueryParser(FIELD_BODY, analyzer);
        all.put("complex-phrase/exact", cp.parse("\"gamma delta\""));
        all.put("complex-phrase/wildcard", cp.parse("\"alph* beta\""));
        // surround
        all.put("surround/and", parseSurround("AND(alpha, beta)"));
        all.put("surround/near", parseSurround("3W(alpha, beta)"));
        // flexible
        StandardQueryParser flex = new StandardQueryParser(analyzer);
        all.put("flexible/term", flex.parse("gamma", FIELD_BODY));
        all.put("flexible/range", flex.parse("[alpha TO gamma]", FIELD_BODY));
        // simple
        SimpleQueryParser simple = new SimpleQueryParser(analyzer, FIELD_BODY);
        all.put("simple/and", simple.parse("alpha + beta"));
        all.put("simple/or", simple.parse("alpha | gamma"));
        // ext (no extension registered ==> behaves like classic for this catalogue)
        ExtendableQueryParser ext = new ExtendableQueryParser(FIELD_BODY, analyzer);
        all.put("ext/term", ext.parse("delta"));
        all.put("ext/bool-or", ext.parse("alpha OR gamma"));
        return all;
    }

    /** Parse a surround expression and lower it to a Lucene Query. */
    private static Query parseSurround(String text) throws Exception {
        SrndQuery sq = org.apache.lucene.queryparser.surround.parser.QueryParser.parse(text);
        BasicQueryFactory qf = new BasicQueryFactory();
        Query q = sq.makeLuceneQueryField(FIELD_BODY, qf);
        // Defensive: QueryParserConstants is referenced so the surround parser
        // jar is forced onto the classpath for transitive reflection scans.
        if (QueryParserConstants.EOF < 0) throw new IllegalStateException("unreachable");
        return q;
    }

    private static List<HitRow> executeAll(DirectoryReader reader, Map<String, Query> queries)
            throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        StoredFields sf = reader.storedFields();
        List<HitRow> rows = new ArrayList<>();
        for (Map.Entry<String, Query> e : queries.entrySet()) {
            String[] split = e.getKey().split("/", 2);
            String parserId = split[0];
            String queryId = split[1];
            TopDocs top = searcher.search(e.getValue(), NUM_DOCS);
            for (int rank = 0; rank < top.scoreDocs.length; rank++) {
                ScoreDoc sd = top.scoreDocs[rank];
                String id = sf.document(sd.doc).get(FIELD_ID);
                rows.add(new HitRow(parserId, queryId, rank, id, sd.score));
            }
        }
        rows.sort(Comparator.<HitRow, String>comparing(h -> h.parserId)
                .thenComparing(h -> h.queryId).thenComparingInt(h -> h.rank));
        return rows;
    }

    private static void writeTreesTsv(Path file, Map<String, Query> queries) throws IOException {
        List<TreeRow> rows = new ArrayList<>();
        for (Map.Entry<String, Query> e : queries.entrySet()) {
            String[] split = e.getKey().split("/", 2);
            rows.add(new TreeRow(split[0], split[1], e.getValue().toString()));
        }
        rows.sort(Comparator.<TreeRow, String>comparing(r -> r.parserId)
                .thenComparing(r -> r.queryId));
        StringBuilder sb = new StringBuilder();
        sb.append("# parser_id\tquery_id\tquery_text\tparsed_to_string\n");
        for (TreeRow r : rows) {
            sb.append(r.parserId).append('\t').append(r.queryId).append('\t')
                    .append(escape(r.parserId + ":" + r.queryId)).append('\t')
                    .append(escape(r.parsedToString)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static void writeHitsTsv(Path file, DirectoryReader reader, Map<String, Query> queries)
            throws IOException {
        List<HitRow> rows = executeAll(reader, queries);
        StringBuilder sb = new StringBuilder();
        sb.append("# parser_id\tquery_id\trank\tdoc_id\tscore (%.6f)\n");
        for (HitRow r : rows) {
            sb.append(r.parserId).append('\t').append(r.queryId).append('\t')
                    .append(r.rank).append('\t').append(r.docId).append('\t')
                    .append(String.format(Locale.ROOT, SCORE_FMT, r.score)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    public static List<TreeRow> readTreesTsv(Path file) throws IOException {
        List<TreeRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 4) throw new IOException("malformed tree row: " + line);
                rows.add(new TreeRow(cols[0], cols[1], unescape(cols[3])));
            }
        }
        return rows;
    }

    public static List<HitRow> readHitsTsv(Path file) throws IOException {
        List<HitRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 5) throw new IOException("malformed hit row: " + line);
                rows.add(new HitRow(cols[0], cols[1], Integer.parseInt(cols[2]),
                        cols[3], Double.parseDouble(cols[4])));
            }
        }
        return rows;
    }

    private static String escape(String s) {
        return s.replace("\\", "\\\\").replace("\t", "\\t").replace("\n", "\\n");
    }

    private static String unescape(String s) {
        StringBuilder out = new StringBuilder(s.length());
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (c == '\\' && i + 1 < s.length()) {
                char n = s.charAt(++i);
                switch (n) {
                    case '\\' -> out.append('\\');
                    case 't' -> out.append('\t');
                    case 'n' -> out.append('\n');
                    default -> out.append(n);
                }
            } else {
                out.append(c);
            }
        }
        return out.toString();
    }

    /** Single tree row. */
    public static final class TreeRow {
        public final String parserId;
        public final String queryId;
        public final String parsedToString;
        public TreeRow(String parserId, String queryId, String parsedToString) {
            this.parserId = parserId; this.queryId = queryId; this.parsedToString = parsedToString;
        }
    }

    /** Single hit row. */
    public static final class HitRow {
        public final String parserId;
        public final String queryId;
        public final int rank;
        public final String docId;
        public final double score;
        public HitRow(String parserId, String queryId, int rank, String docId, double score) {
            this.parserId = parserId; this.queryId = queryId; this.rank = rank;
            this.docId = docId; this.score = score;
        }
        @Override public String toString() {
            return parserId + "/" + queryId + "#" + rank + "/" + docId + "@"
                    + String.format(Locale.ROOT, SCORE_FMT, score);
        }
    }
}
