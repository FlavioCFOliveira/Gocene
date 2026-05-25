package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.SortedDocValuesField;
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
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.ScoreMode;
import org.apache.lucene.search.Sort;
import org.apache.lucene.search.TermQuery;
import org.apache.lucene.search.Weight;
import org.apache.lucene.search.grouping.BlockGroupingCollector;
import org.apache.lucene.search.grouping.FirstPassGroupingCollector;
import org.apache.lucene.search.grouping.GroupDocs;
import org.apache.lucene.search.grouping.SearchGroup;
import org.apache.lucene.search.grouping.TermGroupSelector;
import org.apache.lucene.search.grouping.TopGroups;
import org.apache.lucene.search.grouping.TopGroupsCollector;
import org.apache.lucene.search.similarities.BM25Similarity;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.util.BytesRef;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Locale;

/** Sprint 114 T16 (rmp 4624): {@code grouping-result-corpus}. Audit row
 * (verbatim): "No binary artefacts originate in grouping module.". Indexes
 * 20 flat docs + 3 parent blocks, runs FirstPass+SecondPass and
 * BlockGroupingCollector, emits two TSVs (per-doc + totals); verify re-runs
 * both collectors and asserts each tuple within 1e-6. */
public final class GroupingResultCorpusScenario implements CorpusScenario {

    public static final String TSV_RESULTS = "grouping-results.tsv";
    public static final String TSV_TOTALS = "grouping-totals.tsv";
    public static final String SCORE_FMT = "%.6f";
    public static final String COLLECTOR_FIRST_PASS = "first-pass";
    public static final String COLLECTOR_BLOCK = "block-group";
    public static final int NUM_FLAT_DOCS = 20;
    public static final int NUM_PARENT_BLOCKS = 3;

    private static final String F_ID = "id", F_BODY = "body", F_GROUP = "group", F_TYPE = "type";
    private static final String V_PARENT = "parent", V_CHILD = "child";
    private static final String QUERY_TERM = "alpha";
    private static final int TOP_N_GROUPS = 5, MAX_DOCS_PER_GROUP = 4;

    @Override public String name() { return "grouping-result-corpus"; }
    @Override public String description() { return "grouping module corpus: 2 collectors + 2 TSVs"; }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec()).setSimilarity(new BM25Similarity())
                    .setUseCompoundFile(false).setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler()).setCommitOnClose(true);
            try (IndexWriter w = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_FLAT_DOCS; i++) w.addDocument(buildFlat(i, seed));
                for (int p = 0; p < NUM_PARENT_BLOCKS; p++) w.addDocuments(buildBlock(p, seed));
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                Eval ev = evaluate(reader);
                writeResults(target.resolve(TSV_RESULTS), ev.rows);
                writeTotals(target.resolve(TSV_TOTALS), ev.totals);
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tr = source.resolve(TSV_RESULTS), tt = source.resolve(TSV_TOTALS);
        if (!Files.exists(tr)) throw new IOException(name() + ": missing " + TSV_RESULTS);
        if (!Files.exists(tt)) throw new IOException(name() + ": missing " + TSV_TOTALS);
        List<Row> recRows = readResults(tr);
        List<Tot> recTotals = readTotals(tt);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            Eval ev = evaluate(reader);
            assertRows(recRows, ev.rows);
            assertTotals(recTotals, ev.totals);
        }
    }

    private static Document doc(String id, String body, String group, String type) {
        Document d = new Document();
        d.add(new StoredField(F_ID, id));
        d.add(new StringField(F_ID, id, Field.Store.NO));
        d.add(new TextField(F_BODY, body, Field.Store.YES));
        d.add(new SortedDocValuesField(F_GROUP, new BytesRef(group)));
        if (type != null) d.add(new StringField(F_TYPE, type, Field.Store.YES));
        return d;
    }

    private static Document buildFlat(int i, long seed) {
        int rep = (int) (((seed * 0x9E3779B97F4A7C15L) ^ (long) i) & 0x3L) + 1;
        StringBuilder b = new StringBuilder();
        for (int k = 0; k < rep; k++) b.append(QUERY_TERM).append(' ');
        b.append("beta gamma");
        return doc("doc-" + i, b.toString().trim(), "g-" + (i % 5), null);
    }

    private static List<Document> buildBlock(int p, long seed) {
        int n = 2 + (int) (((seed * 0x9E3779B97F4A7C15L) ^ (long) (1000L + p)) & 0x1L);
        List<Document> out = new ArrayList<>(n + 1);
        for (int j = 0; j < n; j++) {
            out.add(doc("b-" + p + "-c-" + j, QUERY_TERM + " beta", "g-" + (p % 5), V_CHILD));
        }
        out.add(doc("b-" + p + "-p", QUERY_TERM, "g-parent-" + p, V_PARENT));
        return out;
    }

    private static Eval evaluate(DirectoryReader reader) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        TermQuery body = new TermQuery(new Term(F_BODY, QUERY_TERM));
        StoredFields sf = reader.storedFields();
        List<Row> rows = new ArrayList<>();
        List<Tot> totals = new ArrayList<>();

        // first-pass + second-pass to materialise per-doc rows.
        FirstPassGroupingCollector<BytesRef> first = new FirstPassGroupingCollector<>(
                new TermGroupSelector(F_GROUP), Sort.RELEVANCE, TOP_N_GROUPS);
        searcher.search(body, first);
        Collection<SearchGroup<BytesRef>> groups = first.getTopGroups(0);
        int fHits = 0, fGroups = 0;
        if (groups != null) {
            TopGroupsCollector<BytesRef> sec = new TopGroupsCollector<>(
                    new TermGroupSelector(F_GROUP), groups,
                    Sort.RELEVANCE, Sort.RELEVANCE, MAX_DOCS_PER_GROUP, true);
            searcher.search(body, sec);
            TopGroups<BytesRef> tg = sec.getTopGroups(0);
            if (tg != null) {
                fGroups = tg.groups.length;
                for (GroupDocs<BytesRef> g : tg.groups) {
                    String key = g.groupValue() == null ? "" : g.groupValue().utf8ToString();
                    ScoreDoc[] sd = g.scoreDocs();
                    for (int r = 0; r < sd.length; r++) {
                        rows.add(new Row(COLLECTOR_FIRST_PASS, key, r,
                                sf.document(sd[r].doc).get(F_ID), sd[r].score));
                    }
                    fHits += (int) g.totalHits().value();
                }
            }
        }
        totals.add(new Tot(COLLECTOR_FIRST_PASS, fHits, fGroups));

        // block-group: restrict the search to docs inside an addDocuments block.
        Weight lastDoc = searcher.createWeight(
                searcher.rewrite(new TermQuery(new Term(F_TYPE, V_PARENT))),
                ScoreMode.COMPLETE_NO_SCORES, 1.0f);
        BlockGroupingCollector block = new BlockGroupingCollector(
                Sort.RELEVANCE, TOP_N_GROUPS, true, lastDoc);
        BooleanQuery blockQuery = new BooleanQuery.Builder()
                .add(body, BooleanClause.Occur.MUST)
                .add(new BooleanQuery.Builder()
                        .add(new TermQuery(new Term(F_TYPE, V_CHILD)), BooleanClause.Occur.SHOULD)
                        .add(new TermQuery(new Term(F_TYPE, V_PARENT)), BooleanClause.Occur.SHOULD)
                        .build(), BooleanClause.Occur.MUST)
                .build();
        searcher.search(blockQuery, block);
        TopGroups<?> btg = block.getTopGroups(Sort.RELEVANCE, 0, 0, MAX_DOCS_PER_GROUP);
        int bHits = 0, bGroups = 0;
        if (btg != null) {
            bGroups = btg.groups.length;
            for (int gi = 0; gi < btg.groups.length; gi++) {
                GroupDocs<?> g = btg.groups[gi];
                ScoreDoc[] sd = g.scoreDocs();
                String key = "block-" + gi;
                for (int r = 0; r < sd.length; r++) {
                    rows.add(new Row(COLLECTOR_BLOCK, key, r,
                            sf.document(sd[r].doc).get(F_ID), sd[r].score));
                }
                bHits += (int) g.totalHits().value();
            }
        }
        totals.add(new Tot(COLLECTOR_BLOCK, bHits, bGroups));

        rows.sort((a, b) -> {
            int c = a.collectorId.compareTo(b.collectorId);
            if (c != 0) return c;
            c = a.groupKey.compareTo(b.groupKey);
            return c != 0 ? c : Integer.compare(a.rank, b.rank);
        });
        totals.sort((a, b) -> a.collectorId.compareTo(b.collectorId));
        return new Eval(rows, totals);
    }

    private static void writeResults(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder("# collector_id\tgroup_key\trank\tdoc_id\tscore (%.6f)\n");
        for (Row r : rows) {
            sb.append(r.collectorId).append('\t').append(r.groupKey).append('\t')
                    .append(r.rank).append('\t').append(r.docId).append('\t')
                    .append(String.format(Locale.ROOT, SCORE_FMT, r.score)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static void writeTotals(Path file, List<Tot> rows) throws IOException {
        StringBuilder sb = new StringBuilder("# collector_id\ttotal_hit_count\ttotal_group_count\n");
        for (Tot r : rows) {
            sb.append(r.collectorId).append('\t').append(r.totalHits).append('\t')
                    .append(r.totalGroups).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<Row> readResults(Path file) throws IOException {
        List<Row> out = new ArrayList<>();
        for (String[] c : readTsv(file, 5)) {
            out.add(new Row(c[0], c[1], Integer.parseInt(c[2]), c[3], Double.parseDouble(c[4])));
        }
        return out;
    }

    private static List<Tot> readTotals(Path file) throws IOException {
        List<Tot> out = new ArrayList<>();
        for (String[] c : readTsv(file, 3)) {
            out.add(new Tot(c[0], Integer.parseInt(c[1]), Integer.parseInt(c[2])));
        }
        return out;
    }

    private static List<String[]> readTsv(Path file, int cols) throws IOException {
        List<String[]> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] c = line.split("\t", -1);
                if (c.length != cols) throw new IOException("malformed row: " + line);
                rows.add(c);
            }
        }
        return rows;
    }

    private static void assertRows(List<Row> a, List<Row> b) throws IOException {
        if (a.size() != b.size()) throw drift("results row count", a.size(), b.size());
        for (int i = 0; i < a.size(); i++) {
            Row x = a.get(i), y = b.get(i);
            if (!x.collectorId.equals(y.collectorId) || !x.groupKey.equals(y.groupKey)
                    || x.rank != y.rank || !x.docId.equals(y.docId)) {
                throw new IOException("grouping-result-corpus: row " + i + " key drift: " + x + " vs " + y);
            }
            if (Math.abs(x.score - y.score) > 1e-6) {
                throw new IOException("grouping-result-corpus: row " + i + " score drift: " + x + " vs " + y);
            }
        }
    }

    private static void assertTotals(List<Tot> a, List<Tot> b) throws IOException {
        if (a.size() != b.size()) throw drift("totals row count", a.size(), b.size());
        for (int i = 0; i < a.size(); i++) {
            Tot x = a.get(i), y = b.get(i);
            if (!x.collectorId.equals(y.collectorId) || x.totalHits != y.totalHits
                    || x.totalGroups != y.totalGroups) {
                throw new IOException("grouping-result-corpus: totals row " + i + " drift: " + x + " vs " + y);
            }
        }
    }

    private static IOException drift(String what, int a, int b) {
        return new IOException("grouping-result-corpus: " + what + " drift " + a + " vs " + b);
    }

    private record Eval(List<Row> rows, List<Tot> totals) {}

    /** Per-doc row in {@code grouping-results.tsv}. */
    public record Row(String collectorId, String groupKey, int rank, String docId, double score) {}

    /** Per-collector row in {@code grouping-totals.tsv}. */
    public record Tot(String collectorId, int totalHits, int totalGroups) {}
}
