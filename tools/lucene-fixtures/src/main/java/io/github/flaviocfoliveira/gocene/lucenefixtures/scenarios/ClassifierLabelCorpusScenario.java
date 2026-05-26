package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.classification.BM25NBClassifier;
import org.apache.lucene.classification.ClassificationResult;
import org.apache.lucene.classification.KNearestNeighborClassifier;
import org.apache.lucene.classification.SimpleNaiveBayesClassifier;
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
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.util.BytesRef;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

/** Sprint 114 T17 (rmp 4625): {@code classifier-label-corpus}. Audit row
 * (verbatim): "No binary artefacts identified; classification reads existing
 * indices.". Builds a small training index (30 docs across three labels —
 * "spam"/"ham"/"news", deterministic via {@code i%3}), holds out 5 docs as
 * the test set, runs {@link SimpleNaiveBayesClassifier},
 * {@link BM25NBClassifier} and {@link KNearestNeighborClassifier}, and emits
 * {@code classifier-labels.tsv}. {@code verify} re-runs the same pipeline
 * and asserts label equality plus score within {@code 1e-6}. */
public final class ClassifierLabelCorpusScenario implements CorpusScenario {

    public static final String TSV_LABELS = "classifier-labels.tsv";
    public static final String SCORE_FMT = "%.6f";
    public static final String CLASSIFIER_SIMPLE_NB = "simple-naive-bayes";
    public static final String CLASSIFIER_BM25_NB = "bm25-nb";
    public static final String CLASSIFIER_KNN = "knn";
    public static final int NUM_TRAIN_DOCS = 30;
    public static final int NUM_HELD_OUT = 5;
    public static final int KNN_K = 3;

    private static final String[] LABELS = {"spam", "ham", "news"};
    private static final String F_ID = "id", F_BODY = "body", F_LABEL = "label";

    @Override public String name() { return "classifier-label-corpus"; }
    @Override public String description() { return "classification cross-engine label corpus: 3 classifiers x 5 held-out docs"; }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false).setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler()).setCommitOnClose(true);
            try (IndexWriter w = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_TRAIN_DOCS; i++) w.addDocument(buildDoc(i, seed));
            }
            List<Row> rows;
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                rows = evaluate(reader, analyzer, seed);
            }
            writeTsv(target.resolve(TSV_LABELS), rows);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tsv = source.resolve(TSV_LABELS);
        if (!Files.exists(tsv)) throw new IOException(name() + ": missing " + TSV_LABELS);
        List<Row> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             Analyzer analyzer = new StandardAnalyzer();
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<Row> recomputed = evaluate(reader, analyzer, seed);
            assertRows(recorded, recomputed);
        }
    }

    private static Document buildDoc(int i, long seed) {
        String label = LABELS[i % LABELS.length];
        // Deterministic per-(seed,i) repetition count of the label keyword so
        // the term frequencies vary but never on wall-clock or shared RNG state.
        int rep = (int) (((seed * 0x9E3779B97F4A7C15L) ^ (long) i) & 0x3L) + 1;
        StringBuilder body = new StringBuilder();
        for (int k = 0; k < rep; k++) body.append(label).append(' ');
        // Mix in two cross-label tokens so the classifiers must rely on
        // proportions rather than mere disjoint vocabulary.
        body.append("alpha beta");
        Document d = new Document();
        d.add(new StoredField(F_ID, "train-" + i));
        d.add(new StringField(F_ID, "train-" + i, Field.Store.NO));
        d.add(new TextField(F_BODY, body.toString().trim(), Field.Store.YES));
        d.add(new StringField(F_LABEL, label, Field.Store.YES));
        return d;
    }

    /** Builds the held-out test set as bodies + expected labels, using the
     *  same deterministic schema as the training docs but with distinct ids
     *  so the indexed/held-out partition is exact. */
    private static List<Held> buildHeldOut(long seed) {
        List<Held> out = new ArrayList<>(NUM_HELD_OUT);
        for (int j = 0; j < NUM_HELD_OUT; j++) {
            int idx = NUM_TRAIN_DOCS + j;
            String label = LABELS[idx % LABELS.length];
            int rep = (int) (((seed * 0x9E3779B97F4A7C15L) ^ (long) idx) & 0x3L) + 1;
            StringBuilder body = new StringBuilder();
            for (int k = 0; k < rep; k++) body.append(label).append(' ');
            body.append("alpha beta");
            out.add(new Held("test-" + j, body.toString().trim()));
        }
        return out;
    }

    private static List<Row> evaluate(DirectoryReader reader, Analyzer analyzer, long seed) throws IOException {
        StoredFields sf = reader.storedFields();
        if (sf == null) throw new IOException("classifier-label-corpus: no stored fields");
        List<Held> held = buildHeldOut(seed);
        List<Row> rows = new ArrayList<>();

        SimpleNaiveBayesClassifier snb = new SimpleNaiveBayesClassifier(
                reader, analyzer, null, F_LABEL, F_BODY);
        BM25NBClassifier bm25 = new BM25NBClassifier(
                reader, analyzer, null, F_LABEL, F_BODY);
        KNearestNeighborClassifier knn = new KNearestNeighborClassifier(
                reader, null, analyzer, null, KNN_K, 0, 0, F_LABEL, F_BODY);

        for (Held h : held) {
            rows.add(classify(CLASSIFIER_SIMPLE_NB, h.id, snb.assignClass(h.body)));
            rows.add(classify(CLASSIFIER_BM25_NB, h.id, bm25.assignClass(h.body)));
            rows.add(classify(CLASSIFIER_KNN, h.id, knn.assignClass(h.body)));
        }
        rows.sort((a, b) -> {
            int c = a.classifierId.compareTo(b.classifierId);
            return c != 0 ? c : a.testDocId.compareTo(b.testDocId);
        });
        return rows;
    }

    private static Row classify(String classifierId, String testDocId,
                                ClassificationResult<BytesRef> cr) {
        String predicted = cr == null || cr.assignedClass() == null
                ? "" : cr.assignedClass().utf8ToString();
        double score = cr == null ? 0.0 : cr.score();
        return new Row(classifierId, testDocId, predicted, score);
    }

    private static void writeTsv(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder(
                "# classifier_id\ttest_doc_id\tpredicted_label\tconfidence (%.6f)\n");
        for (Row r : rows) {
            sb.append(r.classifierId).append('\t').append(r.testDocId).append('\t')
                    .append(r.predictedLabel).append('\t')
                    .append(String.format(Locale.ROOT, SCORE_FMT, r.confidence)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<Row> readTsv(Path file) throws IOException {
        List<Row> out = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] c = line.split("\t", -1);
                if (c.length != 4) throw new IOException("malformed row: " + line);
                out.add(new Row(c[0], c[1], c[2], Double.parseDouble(c[3])));
            }
        }
        return out;
    }

    private static void assertRows(List<Row> a, List<Row> b) throws IOException {
        if (a.size() != b.size()) {
            throw new IOException("classifier-label-corpus: row count drift "
                    + a.size() + " vs " + b.size());
        }
        for (int i = 0; i < a.size(); i++) {
            Row x = a.get(i), y = b.get(i);
            if (!x.classifierId.equals(y.classifierId) || !x.testDocId.equals(y.testDocId)
                    || !x.predictedLabel.equals(y.predictedLabel)) {
                throw new IOException("classifier-label-corpus: row " + i
                        + " key/label drift: " + x + " vs " + y);
            }
            if (Math.abs(x.confidence - y.confidence) > 1e-6) {
                throw new IOException("classifier-label-corpus: row " + i
                        + " confidence drift: " + x + " vs " + y);
            }
        }
    }

    /** Held-out test row: id + body fed to {@code assignClass}. */
    private record Held(String id, String body) {}

    /** Per-(classifier,test_doc) row in {@code classifier-labels.tsv}. */
    public record Row(String classifierId, String testDocId, String predictedLabel, double confidence) {}
}
