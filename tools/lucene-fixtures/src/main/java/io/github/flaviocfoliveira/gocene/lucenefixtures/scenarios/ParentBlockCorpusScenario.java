package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.core.KeywordAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.index.Term;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TermQuery;
import org.apache.lucene.search.TopDocs;
import org.apache.lucene.search.join.BitSetProducer;
import org.apache.lucene.search.join.QueryBitSetProducer;
import org.apache.lucene.search.join.ScoreMode;
import org.apache.lucene.search.join.ToChildBlockJoinQuery;
import org.apache.lucene.search.join.ToParentBlockJoinQuery;
import org.apache.lucene.search.similarities.BM25Similarity;
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
 * Sprint 114 T15 (rmp 4623): {@code parent-block-corpus}. Addresses the
 * join audit row (verbatim from docs/compat-coverage.tsv):
 * "No binary artefacts originate in join; coverage gap is integration with
 *  Lucene-written parent-block segments".
 *
 * <p>Builds a parent-block index using
 * {@link IndexWriter#addDocuments(Iterable)} where every block is
 * (children..., parent). 5 parents, each with a seed-derived 2..4 child
 * count. Runs two fixed queries:
 * <ul>
 *   <li>{@link ToParentBlockJoinQuery} over child {@code color=color-0}
 *       -&gt; emits parent hits to {@code join-to-parent-hits.tsv}.</li>
 *   <li>{@link ToChildBlockJoinQuery} over parent {@code parent_id=p-1}
 *       -&gt; emits child hits to {@code join-to-child-hits.tsv}.</li>
 * </ul>
 */
public final class ParentBlockCorpusScenario implements CorpusScenario {

    public static final int NUM_PARENTS = 5;
    public static final String TSV_TO_PARENT = "join-to-parent-hits.tsv";
    public static final String TSV_TO_CHILD = "join-to-child-hits.tsv";
    public static final String SCORE_FMT = "%.6f";

    private static final String F_TYPE = "type";
    private static final String F_PARENT_ID = "parent_id";
    private static final String F_CHILD_ID = "child_id";
    private static final String F_COLOR = "color";
    private static final String V_PARENT = "parent";
    private static final String V_CHILD = "child";

    private static final String QID_TO_PARENT = "to-parent-color0";
    private static final String QID_TO_CHILD = "to-child-p1";

    @Override
    public String name() { return "parent-block-corpus"; }

    @Override
    public String description() {
        return "join module parent-block corpus: 5 parents + 2..4 children each + 2 block-join TSVs";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new KeywordAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setSimilarity(new BM25Similarity())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_PARENTS; i++) {
                    writer.addDocuments(buildBlock(i, seed));
                }
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                writeTsv(target.resolve(TSV_TO_PARENT), "parent_id",
                        evaluate(reader, QID_TO_PARENT, buildToParentQuery(), F_PARENT_ID, NUM_PARENTS));
                writeTsv(target.resolve(TSV_TO_CHILD), "child_id",
                        evaluate(reader, QID_TO_CHILD, buildToChildQuery(), F_CHILD_ID, 32));
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tp = source.resolve(TSV_TO_PARENT);
        Path tc = source.resolve(TSV_TO_CHILD);
        if (!Files.exists(tp)) throw new IOException(name() + ": missing " + TSV_TO_PARENT);
        if (!Files.exists(tc)) throw new IOException(name() + ": missing " + TSV_TO_CHILD);
        List<Hit> rp = readTsv(tp);
        List<Hit> rc = readTsv(tc);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            assertEq(rp, evaluate(reader, QID_TO_PARENT, buildToParentQuery(), F_PARENT_ID, NUM_PARENTS));
            assertEq(rc, evaluate(reader, QID_TO_CHILD, buildToChildQuery(), F_CHILD_ID, 32));
        }
    }

    // ---------------------------------------------------------------- block

    private static List<Document> buildBlock(int parentIdx, long seed) {
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) parentIdx;
        int childCount = (int) (mix & 0x3L); // 0..3
        if (childCount < 2) childCount += 2; // clamp into [2,4]
        List<Document> block = new ArrayList<>(childCount + 1);
        for (int j = 0; j < childCount; j++) {
            Document c = new Document();
            c.add(new StringField(F_TYPE, V_CHILD, Field.Store.YES));
            c.add(new StringField(F_CHILD_ID, "p-" + parentIdx + "-c-" + j, Field.Store.YES));
            c.add(new StringField(F_COLOR, "color-" + ((parentIdx + j) % 3), Field.Store.YES));
            block.add(c);
        }
        Document p = new Document();
        p.add(new StringField(F_TYPE, V_PARENT, Field.Store.YES));
        p.add(new StringField(F_PARENT_ID, "p-" + parentIdx, Field.Store.YES));
        block.add(p);
        return block;
    }

    // ---------------------------------------------------------------- query

    private static BitSetProducer parentsFilter() {
        return new QueryBitSetProducer(new TermQuery(new Term(F_TYPE, V_PARENT)));
    }

    private static Query buildToParentQuery() {
        return new ToParentBlockJoinQuery(
                new TermQuery(new Term(F_COLOR, "color-0")), parentsFilter(), ScoreMode.Avg);
    }

    private static Query buildToChildQuery() {
        return new ToChildBlockJoinQuery(
                new TermQuery(new Term(F_PARENT_ID, "p-1")), parentsFilter());
    }

    // ---------------------------------------------------------------- eval

    private static List<Hit> evaluate(DirectoryReader reader, String qid, Query q,
                                      String storedField, int limit) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        TopDocs top = searcher.search(q, limit);
        StoredFields sf = reader.storedFields();
        List<Hit> rows = new ArrayList<>(top.scoreDocs.length);
        for (int rank = 0; rank < top.scoreDocs.length; rank++) {
            ScoreDoc sd = top.scoreDocs[rank];
            rows.add(new Hit(qid, rank, sf.document(sd.doc).get(storedField), sd.score));
        }
        return rows;
    }

    // ---------------------------------------------------------------- io

    private static void writeTsv(Path file, String idColumn, List<Hit> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_id\trank\t").append(idColumn).append("\tscore (%.6f)\n");
        for (Hit r : rows) {
            sb.append(r.queryId).append('\t').append(r.rank).append('\t')
                    .append(r.id).append('\t')
                    .append(String.format(Locale.ROOT, SCORE_FMT, r.score)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<Hit> readTsv(Path file) throws IOException {
        List<Hit> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 4) throw new IOException("malformed row: " + line);
                rows.add(new Hit(cols[0], Integer.parseInt(cols[1]), cols[2],
                        Double.parseDouble(cols[3])));
            }
        }
        return rows;
    }

    private static void assertEq(List<Hit> recorded, List<Hit> recomputed) throws IOException {
        if (recorded.size() != recomputed.size()) {
            throw new IOException("parent-block-corpus: row count drift recorded="
                    + recorded.size() + " recomputed=" + recomputed.size());
        }
        for (int i = 0; i < recorded.size(); i++) {
            Hit x = recorded.get(i), y = recomputed.get(i);
            if (!x.queryId.equals(y.queryId) || x.rank != y.rank || !x.id.equals(y.id)) {
                throw new IOException("parent-block-corpus: row " + i + " key drift: " + x + " vs " + y);
            }
            if (Math.abs(x.score - y.score) > 1e-6) {
                throw new IOException("parent-block-corpus: row " + i + " score drift: " + x + " vs " + y);
            }
        }
    }

    /** One TSV row, used for both parent and child variants. */
    public static final class Hit {
        public final String queryId;
        public final int rank;
        public final String id;
        public final double score;
        public Hit(String queryId, int rank, String id, double score) {
            this.queryId = queryId; this.rank = rank; this.id = id; this.score = score;
        }
        @Override public String toString() {
            return queryId + "#" + rank + "/" + id + "@"
                    + String.format(Locale.ROOT, SCORE_FMT, score);
        }
    }
}
