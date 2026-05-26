package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.misc.HighFreqTerms;
import org.apache.lucene.misc.TermStats;
import org.apache.lucene.store.FSDirectory;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Locale;

/**
 * Sprint 114 T24 (rmp 4632): {@code misc-highfreq-terms-corpus}. Addresses
 * the misc audit row (verbatim): "No tests; tool reads but does not write
 * a persisted artefact" for {@link HighFreqTerms}.
 *
 * <p>HighFreqTerms prints to System.out. We capture its logical output
 * (TermStats[] from {@link HighFreqTerms#getHighFreqTerms}) as a
 * deterministic {@code highfreq-terms.tsv} alongside the source index.
 * Columns: {@code term}, {@code doc_freq}, {@code total_term_freq},
 * sorted by {@code (doc_freq desc, term asc)}.
 */
public final class MiscHighfreqTermsCorpusScenario implements CorpusScenario {

    public static final String NAME = "misc-highfreq-terms-corpus";
    public static final String TSV_NAME = "highfreq-terms.tsv";
    public static final int NUM_DOCS = 20;
    public static final String FIELD = "body";
    public static final int TOP_N = 10;
    private static final String[] VOCAB = {
            "alpha", "beta", "gamma", "delta", "epsilon",
            "zeta", "eta", "theta", "iota", "kappa"
    };

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "Misc HighFreqTerms: " + NUM_DOCS + "-doc corpus + highfreq-terms.tsv "
                + "(top-" + TOP_N + " of " + FIELD + ", doc_freq desc / term asc).";
    }

    @Override public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             StandardAnalyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) writer.addDocument(buildDoc(i, seed));
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                writeTsv(target.resolve(TSV_NAME), computeRows(reader));
            }
        }
    }

    @Override public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tsv = source.resolve(TSV_NAME);
        if (!Files.exists(tsv)) throw new IOException(NAME + ": missing " + TSV_NAME);
        List<HighfreqRow> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<HighfreqRow> recomputed = computeRows(reader);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(NAME + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                HighfreqRow a = recorded.get(i), b = recomputed.get(i);
                if (!a.term.equals(b.term) || a.docFreq != b.docFreq
                        || a.totalTermFreq != b.totalTermFreq) {
                    throw new IOException(NAME + ": row " + i + " drift: " + a + " vs " + b);
                }
            }
        }
    }

    /** Build the i-th document with a deterministic {@code body} drawn from {@link #VOCAB}. */
    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = String.format(Locale.ROOT, "doc-%03d", i);
        doc.add(new StoredField("id", id));
        long mix = mix(seed ^ (long) i ^ 0x9E3779B97F4A7C15L);
        int span = 2 + (int) (mix & 0x3); // 2..5 distinct tokens per doc
        int start = Math.floorMod((int) (mix >>> 32) + i, VOCAB.length);
        StringBuilder body = new StringBuilder();
        for (int j = 0; j < span; j++) {
            String tok = VOCAB[(start + j) % VOCAB.length];
            int reps = 1 + (j & 0x1); // 1 or 2 occurrences per chosen token
            for (int r = 0; r < reps; r++) body.append(tok).append(' ');
        }
        doc.add(new TextField(FIELD, body.toString().trim(), Field.Store.NO));
        return doc;
    }

    /** Run {@link HighFreqTerms} and sort by (doc_freq desc, term asc). */
    private static List<HighfreqRow> computeRows(IndexReader reader) throws IOException {
        TermStats[] stats;
        try {
            stats = HighFreqTerms.getHighFreqTerms(
                    reader, TOP_N, FIELD, new HighFreqTerms.DocFreqComparator());
        } catch (Exception e) {
            throw new IOException(NAME + ": HighFreqTerms.getHighFreqTerms failed: "
                    + e.getMessage(), e);
        }
        List<HighfreqRow> rows = new ArrayList<>(stats.length);
        for (TermStats ts : stats) {
            rows.add(new HighfreqRow(ts.termtext.utf8ToString(), ts.docFreq, ts.totalTermFreq));
        }
        rows.sort(Comparator.comparingLong((HighfreqRow r) -> -r.docFreq)
                .thenComparing(r -> r.term));
        return rows;
    }

    private static void writeTsv(Path file, List<HighfreqRow> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# term\tdoc_freq\ttotal_term_freq\n");
        for (HighfreqRow r : rows) {
            sb.append(r.term).append('\t').append(r.docFreq).append('\t')
                    .append(r.totalTermFreq).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<HighfreqRow> readTsv(Path file) throws IOException {
        List<HighfreqRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 3) throw new IOException("malformed row: " + line);
                rows.add(new HighfreqRow(cols[0], Long.parseLong(cols[1]), Long.parseLong(cols[2])));
            }
        }
        return rows;
    }

    /** SplitMix64 finalizer — same constants as SandboxIdversionPostingsScenario. */
    private static long mix(long z) {
        z = (z ^ (z >>> 30)) * 0xBF58476D1CE4E5B9L;
        z = (z ^ (z >>> 27)) * 0x94D049BB133111EBL;
        return z ^ (z >>> 31);
    }

    /** Single TSV row. */
    public static final class HighfreqRow {
        public final String term;
        public final long docFreq;
        public final long totalTermFreq;

        public HighfreqRow(String term, long docFreq, long totalTermFreq) {
            this.term = term; this.docFreq = docFreq; this.totalTermFreq = totalTermFreq;
        }

        @Override public String toString() {
            return term + "/df=" + docFreq + "/ttf=" + totalTermFreq;
        }
    }
}
