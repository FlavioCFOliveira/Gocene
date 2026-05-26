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
import org.apache.lucene.index.Term;
import org.apache.lucene.search.BooleanClause;
import org.apache.lucene.search.BooleanQuery;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TermQuery;
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
 * Sprint 114 T14 (rmp 4622): {@code highlight-offset-corpus}. Audit row
 * (verbatim, UnifiedHighlighter offset retrieval): "No Lucene-side
 * parity test for offset retrieval". Indexes docs with body under
 * DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS, runs UnifiedHighlighter over
 * a fixed catalogue, emits {@code highlights.tsv}, verifier re-runs UH.
 */
public final class HighlightOffsetCorpusScenario implements CorpusScenario {

    public static final int NUM_DOCS = 16;
    public static final String TSV_NAME = "highlights.tsv";

    private static final String FIELD_BODY = "body";
    private static final String FIELD_ID = "id";
    private static final List<String> QUERY_IDS = List.of(
            "term-alpha", "term-gamma", "term-epsilon", "bool-alpha-or-zeta");

    @Override public String name() { return "highlight-offset-corpus"; }

    @Override public String description() {
        return "UnifiedHighlighter offset corpus: " + NUM_DOCS + " docs + 4 queries";
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
                for (int i = 0; i < NUM_DOCS; i++) writer.addDocument(buildDoc(i, seed));
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
        if (!Files.exists(tsv)) throw new IOException(name() + ": missing " + TSV_NAME);
        List<Row> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             Analyzer analyzer = new StandardAnalyzer();
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<Row> recomputed = evaluate(reader, analyzer);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(name() + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                if (!recorded.get(i).equals(recomputed.get(i))) {
                    throw new IOException(name() + ": row " + i + " drift: "
                            + recorded.get(i) + " vs " + recomputed.get(i));
                }
            }
        }
    }

    private static FieldType bodyFieldType() {
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
        doc.add(new Field(FIELD_BODY, body.toString().trim(), bodyFieldType()));
        return doc;
    }

    private static Map<String, Query> buildQueries() {
        Map<String, Query> q = new LinkedHashMap<>();
        q.put("term-alpha", new TermQuery(new Term(FIELD_BODY, "alpha")));
        q.put("term-gamma", new TermQuery(new Term(FIELD_BODY, "gamma")));
        q.put("term-epsilon", new TermQuery(new Term(FIELD_BODY, "epsilon")));
        BooleanQuery.Builder bq = new BooleanQuery.Builder();
        bq.add(new TermQuery(new Term(FIELD_BODY, "alpha")), BooleanClause.Occur.SHOULD);
        bq.add(new TermQuery(new Term(FIELD_BODY, "zeta")), BooleanClause.Occur.SHOULD);
        q.put("bool-alpha-or-zeta", bq.build());
        if (!q.keySet().equals(new java.util.LinkedHashSet<>(QUERY_IDS))) {
            throw new IllegalStateException("QUERY_IDS / buildQueries drift");
        }
        return q;
    }

    private static List<Row> evaluate(DirectoryReader reader, Analyzer analyzer) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        UnifiedHighlighter uh = UnifiedHighlighter.builder(searcher, analyzer)
                .withMaxNoHighlightPassages(0).build();
        StoredFields sf = reader.storedFields();
        List<Row> rows = new ArrayList<>();
        for (Map.Entry<String, Query> e : buildQueries().entrySet()) {
            String qid = e.getKey();
            Query q = e.getValue();
            TopDocs top = searcher.search(q, NUM_DOCS);
            ScoreDoc[] scoreDocs = top.scoreDocs.clone();
            java.util.Arrays.sort(scoreDocs, (a, b) -> Integer.compare(a.doc, b.doc));
            String[] snippets = uh.highlight(FIELD_BODY, q, new TopDocs(top.totalHits, scoreDocs), 3);
            for (int hi = 0; hi < scoreDocs.length; hi++) {
                if (snippets[hi] == null) continue;
                rows.add(new Row(qid, sf.document(scoreDocs[hi].doc).get(FIELD_ID), 0, snippets[hi]));
            }
        }
        rows.sort((a, b) -> {
            int c = a.queryId().compareTo(b.queryId());
            if (c != 0) return c;
            c = a.docId().compareTo(b.docId());
            return c != 0 ? c : Integer.compare(a.snippetIndex(), b.snippetIndex());
        });
        return rows;
    }

    private static void writeTsv(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_id\tdoc_id\tsnippet_index\tsnippet_text\n");
        for (Row r : rows) {
            sb.append(r.queryId()).append('\t').append(r.docId()).append('\t')
                    .append(r.snippetIndex()).append('\t')
                    .append(TsvEscape.escape(r.snippetText())).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    public static List<Row> readTsv(Path file) throws IOException {
        List<Row> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 4) throw new IOException("malformed row: " + line);
                rows.add(new Row(cols[0], cols[1], Integer.parseInt(cols[2]), TsvEscape.unescape(cols[3])));
            }
        }
        return rows;
    }

    /** Single TSV row. */
    public record Row(String queryId, String docId, int snippetIndex, String snippetText) {
        @Override public String toString() {
            return queryId + "#" + snippetIndex + "/" + docId + ":\"" + snippetText + "\"";
        }
    }
}
