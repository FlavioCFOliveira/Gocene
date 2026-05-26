package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.Codec;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Base class for scenarios that drive a real {@link IndexWriter}. Subclasses
 * declare the documents and codec quirks they need; this class owns the
 * deterministic lifecycle (directory, IWC, force-merge, commit, close).
 *
 * <p>Every IndexWriter scenario uses:
 * <ul>
 *   <li>{@link StandardAnalyzer} with no stopwords (predictable tokens).</li>
 *   <li>{@link NoMergePolicy} + {@link SerialMergeScheduler} so segment IDs
 *       and merge ordering are deterministic.</li>
 *   <li>{@link Lucene104Codec} (Lucene 10.4 default) with the no-arg
 *       constructor.</li>
 * </ul>
 */
public abstract class IndexCorpusScenario implements CorpusScenario {

    /** Number of documents indexed by default. Override when the format demands more. */
    protected int numDocs() {
        return 10;
    }

    /** Whether the writer should produce a compound (.cfs/.cfe) segment. */
    protected boolean useCompoundFile() {
        return false;
    }

    /** Codec used for the writer. Defaults to the Lucene 10.4 codec. */
    protected Codec codec() {
        return new Lucene104Codec();
    }

    /** Analyzer used for the writer. */
    protected Analyzer analyzer() {
        return new StandardAnalyzer();
    }

    /** Build the i-th document. Implementations MUST be deterministic in {@code seed}. */
    protected abstract Document buildDoc(int i, long seed);

    /**
     * Hook executed after every document has been added but before forceMerge.
     * Default: no-op. Override for scenarios that mutate the index (deletes,
     * updates).
     */
    protected void afterAdd(IndexWriter writer, long seed) throws IOException {
        // no-op
    }

    /** Verify reader contents. Default: assert {@link #numDocs()} live docs. */
    protected void verifyReader(IndexReader reader, long seed) throws IOException {
        int expected = expectedLiveDocs(seed);
        if (reader.numDocs() != expected) {
            throw new IOException(name() + ": numDocs mismatch, expected "
                    + expected + ", got " + reader.numDocs());
        }
    }

    /** Number of live (non-deleted) documents the verifier should expect. */
    protected int expectedLiveDocs(long seed) {
        return numDocs();
    }

    @Override
    public final void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = analyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(codec())
                    .setUseCompoundFile(useCompoundFile())
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            // CheckPendingFlush/Threads default to single-threaded for our usage.
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                int n = numDocs();
                for (int i = 0; i < n; i++) {
                    writer.addDocument(buildDoc(i, seed));
                }
                afterAdd(writer, seed);
                // Do NOT forceMerge under NoMergePolicy — it would log a warning
                // and have no effect. The writer is configured to produce a
                // single segment by construction.
                writer.commit();
            }
        }
    }

    @Override
    public final void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             IndexReader reader = DirectoryReader.open(dir)) {
            verifyReader(reader, seed);
        }
    }
}
