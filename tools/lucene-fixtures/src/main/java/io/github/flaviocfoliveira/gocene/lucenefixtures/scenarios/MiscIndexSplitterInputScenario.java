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
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Locale;

/**
 * Sprint 114 T24 (rmp 4632): {@code misc-index-splitter-input}. Addresses
 * the misc audit row (verbatim): "No interop test merging a Lucene-written
 * input" for IndexSplitter / IndexMergeTool.
 *
 * <p>Builds a Lucene-written multi-segment index by appending three
 * disjoint batches of {@value #BATCH_SIZE} documents and committing after
 * each batch. {@link NoMergePolicy} + {@link SerialMergeScheduler} +
 * {@code useCompoundFile=false} pin the final layout to {@value #NUM_SEGMENTS}
 * segments ({@code _0.*}, {@code _1.*}, {@code _2.*}) — the input shape
 * both misc tools operate on. Verify asserts leaf-count and total live
 * docs. Determinism via SplitMix64 + {@link Determinism#seed(long)}.
 */
public final class MiscIndexSplitterInputScenario implements CorpusScenario {

    public static final String NAME = "misc-index-splitter-input";
    public static final int BATCH_SIZE = 6;
    public static final int NUM_SEGMENTS = 3;
    public static final int TOTAL_DOCS = NUM_SEGMENTS * BATCH_SIZE;

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "Misc IndexSplitter/IndexMergeTool input: " + NUM_SEGMENTS
                + " Lucene-written segments (" + BATCH_SIZE
                + " docs each, commit per batch, NoMergePolicy, useCompoundFile=false).";
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
                    .setCommitOnClose(false);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                int docNum = 0;
                for (int b = 0; b < NUM_SEGMENTS; b++) {
                    for (int i = 0; i < BATCH_SIZE; i++) {
                        writer.addDocument(buildDoc(docNum++, seed));
                    }
                    writer.commit();
                }
            }
        }
    }

    @Override public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            int segs = reader.leaves().size();
            if (segs != NUM_SEGMENTS) {
                throw new IOException(String.format(Locale.ROOT,
                        "%s: leaf count=%d, want %d", NAME, segs, NUM_SEGMENTS));
            }
            int live = reader.numDocs();
            if (live != TOTAL_DOCS) {
                throw new IOException(String.format(Locale.ROOT,
                        "%s: numDocs=%d, want %d", NAME, live, TOTAL_DOCS));
            }
        }
    }

    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = String.format(Locale.ROOT, "doc-%03d", i);
        doc.add(new StoredField("id", id));
        doc.add(new StringField("id", id, Field.Store.NO));
        long mix = mix(seed ^ (long) i ^ 0x9E3779B97F4A7C15L);
        int reps = (int) ((mix & 0x3) + 1); // 1..4 occurrences of "alpha"
        StringBuilder body = new StringBuilder();
        for (int k = 0; k < reps; k++) body.append("alpha ");
        body.append("beta gamma ");
        if ((i % 2) == 0) body.append("delta ");
        body.append("pivot-").append(i);
        doc.add(new TextField("body", body.toString().trim(), Field.Store.NO));
        doc.add(new StringField("tag", "tag-" + (i % 3), Field.Store.NO));
        return doc;
    }

    /** SplitMix64 finalizer — same constants as SandboxIdversionPostingsScenario. */
    private static long mix(long z) {
        z = (z ^ (z >>> 30)) * 0xBF58476D1CE4E5B9L;
        z = (z ^ (z >>> 27)) * 0x94D049BB133111EBL;
        return z ^ (z >>> 31);
    }
}
