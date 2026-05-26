package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.FieldType;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexOptions;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.queryparser.classic.ParseException;
import org.apache.lucene.queryparser.classic.QueryParser;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TopDocs;
import org.apache.lucene.search.similarities.BM25Similarity;
import org.apache.lucene.search.uhighlight.UnifiedHighlighter;
import org.apache.lucene.store.FSDirectory;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Sprint 114 T5 (rmp 4611), S6 {@code combined-highlight-queryparser-analysis}.
 * Classic QueryParser -> StandardAnalyzer -> UnifiedHighlighter chain over 3
 * queries; emits {@value #TSV_NAME} (query_text, doc_id, snippet_index,
 * snippet) sorted by (query_text, doc_id, snippet_index) with TsvEscape
 * round-tripping {@code \t \n \r \\}.
 */
public final class CombinedHighlightQueryparserAnalysisScenario implements CorpusScenario {

    public static final String NAME = "combined-highlight-queryparser-analysis";
    public static final String TSV_NAME = "s6-highlights.tsv";
    public static final int NUM_DOCS = 12;
    private static final String FIELD_BODY = "body";
    private static final String FIELD_ID = "id";

    /** The three deterministic query strings (kept in stack order). */
    public static final List<String> QUERY_TEXTS = List.of(
            "alpha AND gamma",
            "\"alpha beta\"",
            "epsilon OR zeta");

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "classic QueryParser -> StandardAnalyzer -> UnifiedHighlighter chain; emits s6-highlights.tsv.";
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
                writeTsv(target.resolve(TSV_NAME), evaluate(reader, analyzer));
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
             Analyzer analyzer = new StandardAnalyzer();
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<Row> recomputed = evaluate(reader, analyzer);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(NAME + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                if (!recorded.get(i).equals(recomputed.get(i))) {
                    throw new IOException(NAME + ": row " + i + " drift: "
                            + recorded.get(i) + " vs " + recomputed.get(i));
                }
            }
        }
    }

    private static FieldType bodyType() {
        FieldType ft = new FieldType(TextField.TYPE_STORED);
        ft.setIndexOptions(IndexOptions.DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS);
        ft.freeze();
        return ft;
    }

    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        doc.add(new StoredField(FIELD_ID, id));
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        StringBuilder body = new StringBuilder();
        for (int k = 0, n = (int) ((mix & 0x3) + 1); k < n; k++) body.append("alpha ");
        for (int k = 0, n = (int) (((mix >>> 2) & 0x3) + 1); k < n; k++) body.append("beta ");
        body.append("gamma delta ");
        if ((i % 3) == 0) body.append("epsilon ");
        if ((i % 4) == 0) body.append("zeta ");
        body.append("pivot ").append(id);
        doc.add(new Field(FIELD_BODY, body.toString().trim(), bodyType()));
        return doc;
    }

    private static List<Row> evaluate(DirectoryReader reader, Analyzer analyzer)
            throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        UnifiedHighlighter uh = UnifiedHighlighter.builder(searcher, analyzer)
                .withMaxNoHighlightPassages(0).build();
        StoredFields sf = reader.storedFields();
        Map<String, Query> parsed = parseAll(analyzer);
        List<Row> rows = new ArrayList<>();
        for (Map.Entry<String, Query> e : parsed.entrySet()) {
            String qtext = e.getKey();
            Query q = e.getValue();
            TopDocs top = searcher.search(q, NUM_DOCS);
            ScoreDoc[] sds = top.scoreDocs.clone();
            // Sort by doc-id for stable iteration order before highlight().
            java.util.Arrays.sort(sds, (a, b) -> Integer.compare(a.doc, b.doc));
            String[] snippets = uh.highlight(FIELD_BODY, q, new TopDocs(top.totalHits, sds), 3);
            for (int hi = 0; hi < sds.length; hi++) {
                if (snippets[hi] == null) continue;
                rows.add(new Row(qtext, sf.document(sds[hi].doc).get(FIELD_ID), 0, snippets[hi]));
            }
        }
        rows.sort((a, b) -> {
            int c = a.queryText().compareTo(b.queryText());
            if (c != 0) return c;
            c = a.docId().compareTo(b.docId());
            return c != 0 ? c : Integer.compare(a.snippetIndex(), b.snippetIndex());
        });
        return rows;
    }

    private static Map<String, Query> parseAll(Analyzer analyzer) {
        Map<String, Query> q = new LinkedHashMap<>();
        QueryParser qp = new QueryParser(FIELD_BODY, analyzer);
        for (String qtext : QUERY_TEXTS) {
            try {
                q.put(qtext, qp.parse(qtext));
            } catch (ParseException pe) {
                throw new IllegalStateException(NAME + ": failed to parse '" + qtext + "'", pe);
            }
        }
        return q;
    }

    private static void writeTsv(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_text\tdoc_id\tsnippet_index\tsnippet\n");
        for (Row r : rows) {
            sb.append(TsvEscape.escape(r.queryText())).append('\t').append(r.docId()).append('\t')
                    .append(r.snippetIndex()).append('\t').append(TsvEscape.escape(r.snippetText()))
                    .append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<Row> readTsv(Path file) throws IOException {
        List<Row> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 4) throw new IOException("malformed row: " + line);
                rows.add(new Row(TsvEscape.unescape(cols[0]), cols[1],
                        Integer.parseInt(cols[2]), TsvEscape.unescape(cols[3])));
            }
        }
        return rows;
    }

    /** Single TSV row. */
    public record Row(String queryText, String docId, int snippetIndex, String snippetText) {
        @Override public String toString() {
            return queryText + "/" + docId + "#" + snippetIndex + ":\"" + snippetText + "\"";
        }
    }
}
