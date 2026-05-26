package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.NumericDocValuesField;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.expressions.Expression;
import org.apache.lucene.expressions.SimpleBindings;
import org.apache.lucene.expressions.js.JavascriptCompiler;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.LeafReaderContext;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.index.Term;
import org.apache.lucene.search.DoubleValues;
import org.apache.lucene.search.DoubleValuesSource;
import org.apache.lucene.search.IndexSearcher;
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
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Sprint 114 T21 (rmp 4629): {@code expressions-eval-corpus}.
 *
 * <p>Addresses audit row (verbatim): "No artefact persists to disk; Gocene
 * port does not generate JVM bytecode so binary parity is N/A but interop
 * with Lucene-compiled exprs is missing". Compiles a fixed JS-expression
 * catalogue via {@link JavascriptCompiler#compile}, evaluates each one
 * against a deterministic 20-doc Lucene index ({@code id}, {@code a} =
 * seed*(i+1), {@code b} = seed*(i+1)+i, plus a {@code body} text field
 * matched by a fixed BM25-scored query), and writes
 * {@code expressions-eval.tsv} ({@code expr_id\tdoc_id\tvalue}, value
 * formatted {@code "%.10g"}, sorted by {@code (expr_id, doc_id)}).
 * The {@code _score} variable is bound to
 * {@link DoubleValuesSource#SCORES} evaluated through {@link TopDocs}.
 */
public final class ExpressionsEvalCorpusScenario implements CorpusScenario {

    /** Number of documents indexed. */
    public static final int NUM_DOCS = 20;

    /** TSV filename written next to the Lucene index inside the scenario directory. */
    public static final String TSV_NAME = "expressions-eval.tsv";

    /** Per-value format string. {@code %.10g} keeps 10 significant digits. */
    public static final String VALUE_FMT = "%.10g";

    /** Stable expression catalogue. Indices map 1:1 with {@code expr_id} in the TSV. */
    public static final List<String[]> EXPR_CATALOGUE = List.of(
            new String[]{"add", "a + b"},
            new String[]{"mul-sub", "a * b - 7"},
            new String[]{"max", "max(a, b)"},
            new String[]{"ternary", "a + b > 100 ? a : b"},
            new String[]{"score-mix", "_score + a / (b + 1)"}
    );

    // Verify tolerance: |a-b| <= ABS_EPS + REL_EPS*|b|. The two-term form
    // pins 1e-10 for values near 1 while accommodating %.10g rounding on
    // large products (a*b reaches ~10^14 at the canary seeds).
    private static final double ABS_EPS = 1.0e-10;
    private static final double REL_EPS = 5.0e-10;

    @Override
    public String name() {
        return "expressions-eval-corpus";
    }

    @Override
    public String description() {
        return "Expressions eval corpus: 20 docs x 5 compiled JS exprs + expressions-eval.tsv "
                + "(audit: interop with Lucene-compiled expressions)";
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
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                List<EvalRow> rows = evaluate(reader);
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
        List<EvalRow> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<EvalRow> recomputed = evaluate(reader);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(name() + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                EvalRow a = recorded.get(i);
                EvalRow b = recomputed.get(i);
                if (!a.exprId.equals(b.exprId) || !a.docId.equals(b.docId)) {
                    throw new IOException(name() + ": row " + i + " key drift: " + a + " vs " + b);
                }
                double tol = ABS_EPS + REL_EPS * Math.abs(b.value);
                if (Math.abs(a.value - b.value) > tol) {
                    throw new IOException(name() + ": row " + i + " value drift: " + a + " vs " + b);
                }
            }
        }
    }

    /** Build the i-th document with seed-derived {@code a}, {@code b}, and {@code body}. */
    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        long a = seed * (long) (i + 1);
        long b = seed * (long) (i + 1) + (long) i;
        doc.add(new StoredField("id", id));
        doc.add(new StringField("id", id, Field.Store.NO));
        doc.add(new NumericDocValuesField("a", a));
        doc.add(new NumericDocValuesField("b", b));
        // Body always contains "doc" so the BM25 TermQuery matches all 20 docs;
        // repetition is seed-derived so per-doc _score values are distinct.
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        int rep = (int) ((mix & 0x7) + 1);
        StringBuilder body = new StringBuilder();
        for (int k = 0; k < rep; k++) {
            body.append("doc ");
        }
        body.append("pivot-").append(i);
        doc.add(new TextField("body", body.toString().trim(), Field.Store.NO));
        return doc;
    }

    /** Compile the catalogue, score the BM25 query, evaluate every expr per doc. */
    private static List<EvalRow> evaluate(DirectoryReader reader) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        searcher.setSimilarity(new BM25Similarity());
        Query scoreQuery = new TermQuery(new Term("body", "doc"));

        // Per-doc BM25 score (per internal docID) — looked up via explain so we
        // never depend on iteration order leaking into the eval result.
        TopDocs top = searcher.search(scoreQuery, NUM_DOCS);
        Map<Integer, Double> scoreByDocId = new HashMap<>();
        for (ScoreDoc sd : top.scoreDocs) {
            scoreByDocId.put(sd.doc, (double) sd.score);
        }

        // JavascriptCompiler.compile throws a checked ParseException; the
        // catalogue is hard-coded so failure is a programmer bug.
        List<Expression> compiled = new ArrayList<>(EXPR_CATALOGUE.size());
        for (String[] row : EXPR_CATALOGUE) {
            try {
                compiled.add(JavascriptCompiler.compile(row[1]));
            } catch (java.text.ParseException pe) {
                throw new IOException("compile failure for " + row[0], pe);
            }
        }

        SimpleBindings bindings = new SimpleBindings();
        bindings.add("a", DoubleValuesSource.fromLongField("a"));
        bindings.add("b", DoubleValuesSource.fromLongField("b"));
        bindings.add("_score", DoubleValuesSource.SCORES);

        StoredFields sf = reader.storedFields();
        List<EvalRow> rows = new ArrayList<>();
        for (int e = 0; e < compiled.size(); e++) {
            String exprId = EXPR_CATALOGUE.get(e)[0];
            DoubleValuesSource vs = compiled.get(e).getDoubleValuesSource(bindings);
            for (LeafReaderContext leaf : reader.leaves()) {
                int base = leaf.docBase;
                int max = leaf.reader().maxDoc();
                // Per-leaf score wrapper; advanceExact(doc) returns the
                // BM25 score for the absolute docID (base+doc).
                DoubleValues scoreWrap = new DoubleValues() {
                    int cur = -1;
                    @Override
                    public boolean advanceExact(int doc) {
                        cur = doc + base;
                        return scoreByDocId.containsKey(cur);
                    }
                    @Override
                    public double doubleValue() {
                        Double v = scoreByDocId.get(cur);
                        return v == null ? 0.0d : v;
                    }
                };
                DoubleValues values = vs.getValues(leaf, scoreWrap);
                for (int doc = 0; doc < max; doc++) {
                    if (!values.advanceExact(doc)) {
                        // Every doc has a/b/score so this should not happen,
                        // but guard against codec quirks.
                        continue;
                    }
                    String id = sf.document(base + doc).get("id");
                    rows.add(new EvalRow(exprId, id, values.doubleValue()));
                }
            }
        }
        rows.sort(Comparator.<EvalRow, String>comparing(r -> r.exprId)
                .thenComparing(r -> r.docId));
        return rows;
    }

    /** Write rows as TSV. */
    private static void writeTsv(Path file, List<EvalRow> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# expr_id\tdoc_id\tvalue (").append(VALUE_FMT).append(")\n");
        for (EvalRow r : rows) {
            sb.append(r.exprId).append('\t').append(r.docId).append('\t')
                    .append(String.format(Locale.ROOT, VALUE_FMT, r.value)).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    /** Parse a TSV file written by {@link #writeTsv}. */
    public static List<EvalRow> readTsv(Path file) throws IOException {
        List<EvalRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) {
                    continue;
                }
                String[] cols = line.split("\t", -1);
                if (cols.length != 3) {
                    throw new IOException("malformed row: " + line);
                }
                rows.add(new EvalRow(cols[0], cols[1], Double.parseDouble(cols[2])));
            }
        }
        return rows;
    }

    /** Single TSV row. */
    public static final class EvalRow {
        public final String exprId;
        public final String docId;
        public final double value;
        public EvalRow(String exprId, String docId, double value) {
            this.exprId = exprId;
            this.docId = docId;
            this.value = value;
        }
        @Override
        public String toString() {
            return exprId + "/" + docId + "@" + String.format(Locale.ROOT, VALUE_FMT, value);
        }
    }
}
