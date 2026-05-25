import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.document.*;
import org.apache.lucene.index.*;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.util.BytesRef;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

/**
 * Generates golden binary fixtures for Gocene cross-engine tests.
 *
 * Usage: java -jar fixture-gen.jar [output-dir]
 * Default output-dir: testdata/lucene-10.4.0-fixtures
 *
 * Writes a small deterministic Lucene 10.4.0 index covering:
 *   - Postings (term frequencies, positions, offsets, payloads)
 *   - DocValues (NUMERIC, BINARY, SORTED, SORTED_NUMERIC, SORTED_SET)
 *   - KNN float32 and byte vectors
 *   - Stored fields
 *   - FieldInfos, SegmentInfo (.si), compound file (.cfs/.cfe)
 */
public class FixtureGen {

    static final int NUM_DOCS = 20;
    static final int VECTOR_DIM = 4;

    public static void main(String[] args) throws IOException {
        String outDir = args.length > 0 ? args[0] : "testdata/lucene-10.4.0-fixtures";
        Path dir = Paths.get(outDir);
        Files.createDirectories(dir);

        System.out.println("Writing Lucene 10.4.0 fixtures to: " + dir.toAbsolutePath());

        try (FSDirectory fsDir = FSDirectory.open(dir);
             StandardAnalyzer analyzer = new StandardAnalyzer()) {

            IndexWriterConfig cfg = new IndexWriterConfig(analyzer);
            cfg.setOpenMode(IndexWriterConfig.OpenMode.CREATE);
            // Use default (Lucene104Codec) — writes .cfs compound segments
            cfg.setUseCompoundFile(true);

            try (IndexWriter writer = new IndexWriter(fsDir, cfg)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    writer.addDocument(buildDoc(i));
                }
                writer.forceMerge(1); // single segment for simplicity
                writer.commit();
            }
        }

        System.out.println("Done. Files generated:");
        try (var stream = Files.list(dir)) {
            stream.sorted().forEach(p -> System.out.println("  " + p.getFileName()
                    + " (" + fileSize(p) + " bytes)"));
        }
    }

    static Document buildDoc(int i) {
        Document doc = new Document();

        // Stored field
        doc.add(new StoredField("id", "doc-" + i));

        // Full-text field (postings with positions + offsets)
        FieldType ft = new FieldType(TextField.TYPE_STORED);
        ft.setStoreTermVectors(true);
        ft.setStoreTermVectorPositions(true);
        ft.setStoreTermVectorOffsets(true);
        ft.freeze();
        doc.add(new Field("body",
                "lucene document number " + i + " with some words for testing postings",
                ft));

        // Keyword field (for sorted doc-values + postings without positions)
        doc.add(new KeywordField("tag", "tag-" + (i % 5), Field.Store.YES));

        // NUMERIC doc-values
        doc.add(new NumericDocValuesField("num_dv", (long) i * 10));

        // BINARY doc-values
        doc.add(new BinaryDocValuesField("bin_dv", new BytesRef("bin-" + i)));

        // SORTED doc-values
        doc.add(new SortedDocValuesField("sorted_dv", new BytesRef("sorted-" + String.format("%03d", i))));

        // SORTED_NUMERIC doc-values (two values per doc)
        doc.add(new SortedNumericDocValuesField("sorted_num_dv", (long) i));
        doc.add(new SortedNumericDocValuesField("sorted_num_dv", (long) i * 2));

        // SORTED_SET doc-values
        doc.add(new SortedSetDocValuesField("sorted_set_dv", new BytesRef("val-a-" + (i % 3))));
        doc.add(new SortedSetDocValuesField("sorted_set_dv", new BytesRef("val-b-" + (i % 3))));

        // KNN float vector
        float[] floatVec = new float[VECTOR_DIM];
        for (int d = 0; d < VECTOR_DIM; d++) {
            floatVec[d] = (float) (i * 0.1 + d * 0.01);
        }
        doc.add(new KnnFloatVectorField("float_vec", floatVec));

        // KNN byte vector
        byte[] byteVec = new byte[VECTOR_DIM];
        for (int d = 0; d < VECTOR_DIM; d++) {
            byteVec[d] = (byte) ((i + d) % 128);
        }
        doc.add(new KnnByteVectorField("byte_vec", byteVec));

        // IntPoint (BKD / points format)
        doc.add(new IntPoint("int_point", i, i * 2));
        doc.add(new StoredField("int_point_stored", i));

        return doc;
    }

    static long fileSize(Path p) {
        try { return Files.size(p); } catch (IOException e) { return -1; }
    }
}
