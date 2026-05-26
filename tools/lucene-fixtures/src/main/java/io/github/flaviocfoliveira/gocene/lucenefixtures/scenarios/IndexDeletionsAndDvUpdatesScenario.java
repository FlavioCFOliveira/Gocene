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
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.LeafReaderContext;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.NumericDocValues;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.index.Term;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T8 (rmp 4616): {@code index-deletions-and-dv-updates}.
 *
 * <p>Drives an {@link IndexWriter} through three phases so the resulting
 * segment carries every per-generation artefact the {@code index/} audit
 * rows depend on:
 *
 * <ol>
 *   <li>Phase 1 indexes 12 documents with a stored {@code "id"} field, a
 *       {@link NumericDocValuesField} {@code "count"} and a
 *       {@link StringField} {@code "tag"}; commit produces generation 0
 *       ({@code segments_1}, {@code .si}, {@code .fnm}, postings + DV).</li>
 *   <li>Two documents are deleted ({@code id=doc-3} and {@code id=doc-7})
 *       and a commit produces the generational {@code _0_1.liv} live-docs
 *       bitmap.</li>
 *   <li>{@code "count"} is updated to {@code 999L} for {@code id=doc-5} and
 *       a third commit produces the generational
 *       {@code _0_Lucene90_0_1.dvd}/{@code .dvm} pair.</li>
 * </ol>
 *
 * <p>{@code useCompoundFile=false}, {@link NoMergePolicy} and
 * {@link SerialMergeScheduler} pin the segment layout so the auxiliary
 * files are visible at the directory root and the generation numbers stay
 * stable across runs. {@link Determinism#seed(long)} is the first call so
 * Lucene's {@code StringHelper.nextId()} produces a deterministic segment
 * id.
 */
public final class IndexDeletionsAndDvUpdatesScenario implements CorpusScenario {

    /** Number of documents indexed in phase 1. */
    public static final int NUM_DOCS = 12;
    /** Documents whose ids will be deleted in phase 2. */
    public static final String[] DELETED_IDS = {"doc-3", "doc-7"};
    /** Document whose "count" doc-value will be updated in phase 3. */
    public static final String UPDATED_ID = "doc-5";
    /** Updated DV value applied in phase 3. */
    public static final long UPDATED_COUNT = 999L;

    @Override
    public String name() {
        return "index-deletions-and-dv-updates";
    }

    @Override
    public String description() {
        return "IndexWriter lifecycle: 12 docs, 2 deletes, 1 numeric DV update across 3 commits";
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
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                // Phase 1: add NUM_DOCS deterministic docs.
                for (int i = 0; i < NUM_DOCS; i++) {
                    Document doc = new Document();
                    String id = "doc-" + i;
                    // Stored so verify() can recover the id from a doc number.
                    doc.add(new StoredField("id", id));
                    doc.add(new StringField("id", id, Field.Store.NO));
                    doc.add(new NumericDocValuesField("count", (long) i + seed));
                    doc.add(new StringField("tag", "tag-" + (i % 3), Field.Store.NO));
                    writer.addDocument(doc);
                }
                writer.commit();

                // Phase 2: delete two docs by id. Each Term targets the
                // unstored StringField so the deletion is by exact key.
                for (String id : DELETED_IDS) {
                    writer.deleteDocuments(new Term("id", id));
                }
                writer.commit();

                // Phase 3: update one doc's numeric DV. NoMergePolicy keeps
                // the update isolated to a generational .dvd/.dvm pair.
                writer.updateNumericDocValue(new Term("id", UPDATED_ID), "count", UPDATED_COUNT);
                writer.commit();
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            int expectedLive = NUM_DOCS - DELETED_IDS.length;
            if (reader.numDocs() != expectedLive) {
                throw new IOException(name() + ": numDocs=" + reader.numDocs()
                        + ", want " + expectedLive);
            }
            // Confirm the updated DV value is visible on the surviving doc.
            boolean sawUpdate = false;
            for (LeafReaderContext leaf : reader.leaves()) {
                NumericDocValues nv = leaf.reader().getNumericDocValues("count");
                if (nv == null) continue;
                StoredFields sf = leaf.reader().storedFields();
                for (int d = 0; d < leaf.reader().maxDoc(); d++) {
                    if (leaf.reader().getLiveDocs() != null
                            && !leaf.reader().getLiveDocs().get(d)) {
                        continue;
                    }
                    if (!nv.advanceExact(d)) continue;
                    String id = sf.document(d).get("id");
                    if (UPDATED_ID.equals(id)) {
                        if (nv.longValue() != UPDATED_COUNT) {
                            throw new IOException(name() + ": " + UPDATED_ID
                                    + ".count=" + nv.longValue() + ", want " + UPDATED_COUNT);
                        }
                        sawUpdate = true;
                    }
                }
            }
            if (!sawUpdate) {
                throw new IOException(name() + ": did not find " + UPDATED_ID + " in any leaf");
            }
        }
    }
}
