package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.NumericDocValuesField;
import org.apache.lucene.document.StringField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.Term;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T8 (rmp 4616) helper: {@code index-soft-deletes}.
 *
 * <p>Exercises {@link IndexWriterConfig#setSoftDeletesField(String)}: a
 * "soft delete" is logically a deletion but the doc remains in the
 * segment, marked by a tombstone on the configured field. The on-disk
 * shape is a generational {@code .liv} + DV update (Lucene encodes the
 * soft-deletes set via a {@link NumericDocValuesField} tombstone), so
 * the file layout is identical to a hard-delete scenario; what differs
 * is {@code SegmentCommitInfo.softDelCount} ({@code > 0}) and
 * {@code delCount} ({@code 0}).
 *
 * <p>Six documents are indexed; one is soft-deleted via
 * {@link IndexWriter#softUpdateDocument(Term, Iterable,
 * org.apache.lucene.index.IndexableField...)}. The scenario commits
 * twice so the on-disk layout settles.
 *
 * <p>Determinism is forced by {@link Determinism#seed(long)},
 * {@link NoMergePolicy}, {@link SerialMergeScheduler} and
 * {@code useCompoundFile=false}.
 */
public final class SoftDeletesScenario implements CorpusScenario {

    /** Field name used as the soft-deletes tombstone marker. */
    public static final String SOFT_DELETES_FIELD = "__soft_delete";
    /** Number of documents added in phase 1. */
    public static final int NUM_DOCS = 6;
    /** Document id that is soft-deleted in phase 2. */
    public static final String SOFT_DELETED_ID = "sd-3";

    @Override
    public String name() {
        return "index-soft-deletes";
    }

    @Override
    public String description() {
        return "IndexWriter with setSoftDeletesField: 6 docs, 1 soft-deleted via softUpdateDocument";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
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
            iwc.setSoftDeletesField(SOFT_DELETES_FIELD);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    Document doc = new Document();
                    doc.add(new StringField("id", "sd-" + i, Field.Store.YES));
                    doc.add(new NumericDocValuesField("rank", seed + i));
                    writer.addDocument(doc);
                }
                writer.commit();

                // Soft-delete sd-3 by writing a tombstone DV update on the
                // configured soft-deletes field. softUpdateDocument keeps
                // the existing document in place and stamps the tombstone.
                writer.softUpdateDocument(
                        new Term("id", SOFT_DELETED_ID),
                        java.util.Collections.singletonList(new StringField("id", SOFT_DELETED_ID, Field.Store.YES)),
                        new NumericDocValuesField(SOFT_DELETES_FIELD, 1L));
                writer.commit();
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            // A soft-delete adds a tombstone but does NOT remove the doc;
            // softUpdateDocument reindexes the doc, so numDocs grows by 1.
            int expected = NUM_DOCS + 1;
            if (reader.numDocs() != expected) {
                throw new IOException(name() + ": numDocs=" + reader.numDocs()
                        + ", want " + expected);
            }
        }
    }
}
