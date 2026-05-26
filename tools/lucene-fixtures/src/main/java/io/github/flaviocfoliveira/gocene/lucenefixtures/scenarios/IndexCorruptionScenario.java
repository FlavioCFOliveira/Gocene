package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.index.CorruptIndexException;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexFormatTooOldException;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.io.RandomAccessFile;
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;

/**
 * Sprint 114 T8 (rmp 4616): {@code index-corruption}.
 *
 * <p>Generates two siblings under {@code target/}:
 * <ul>
 *   <li>{@code valid/} — a clean Lucene index, identical in shape to
 *       {@link SegmentInfoFormatScenario}. The directory is what
 *       downstream tests open and what the manifest digest covers.</li>
 *   <li>{@code corrupted/} — a byte-for-byte copy of {@code valid/} whose
 *       {@code segments_N} footer has been truncated (the last 8 bytes,
 *       including the CRC32 trailer, are removed). Opening the directory
 *       with Lucene MUST raise a {@link CorruptIndexException} (or, on
 *       certain JVMs, {@link java.io.EOFException} surfaced via
 *       {@link IndexFormatTooOldException}).</li>
 * </ul>
 *
 * <p>The corruption is deterministic for a given seed: same input bytes
 * truncated by the same fixed amount produce the same output bytes.
 * {@code Manifest.snapshot} therefore stays stable across runs even
 * though one of the artefacts is intentionally broken — the digest covers
 * both subtrees verbatim.
 *
 * <p>{@link #verify(Path, long)} asserts the {@code valid/} subtree opens
 * cleanly with a {@link DirectoryReader} AND that opening the
 * {@code corrupted/} subtree raises the expected exception. Either
 * outcome ({@link CorruptIndexException} or {@link java.io.EOFException})
 * is accepted because Lucene maps the underlying short-read condition to
 * different exception types depending on which header byte the reader
 * trips over first.
 */
public final class IndexCorruptionScenario implements CorpusScenario {

    /** Number of bytes truncated from {@code valid/segments_N} to corrupt it. */
    public static final int TRUNCATE_BYTES = 8;

    @Override
    public String name() {
        return "index-corruption";
    }

    @Override
    public String description() {
        return "Deterministic CorruptIndexException fixture: valid/ + corrupted/ siblings";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        Path valid = target.resolve("valid");
        Path corrupted = target.resolve("corrupted");
        Files.createDirectories(valid);
        // Re-seed before generate so the StringHelper state is identical
        // for valid/ and the subsequent corrupted/ copy.
        Determinism.seed(seed);
        writeValid(valid);
        // Re-seed once more for the deterministic copy step (no PRNG is
        // actually consumed, but keep the surrounding state consistent).
        Determinism.seed(seed);
        copyTree(valid, corrupted);
        truncateSegmentsFooter(corrupted);
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path valid = source.resolve("valid");
        Path corrupted = source.resolve("corrupted");
        // (1) valid/ opens cleanly.
        try (FSDirectory dir = FSDirectory.open(valid);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            if (reader.maxDoc() == 0) {
                throw new IOException(name() + ": valid/ has zero docs");
            }
        }
        // (2) corrupted/ throws. We accept any IOException subtype the
        //     reader chooses to surface — CorruptIndexException is the
        //     documented expectation; EOFException can wrap the same
        //     short-read on older JVMs.
        boolean threw = false;
        try (FSDirectory dir = FSDirectory.open(corrupted);
             DirectoryReader ignored = DirectoryReader.open(dir)) {
            // no-op
        } catch (CorruptIndexException | java.io.EOFException expected) {
            threw = true;
        } catch (IOException other) {
            // Some Lucene paths wrap the CRC mismatch as a plain IOException
            // ("checksum failed" or "footer mismatch"); accept those too.
            String msg = other.getMessage() == null ? "" : other.getMessage();
            if (msg.contains("checksum") || msg.contains("footer") || msg.contains("CRC")) {
                threw = true;
            } else {
                throw other;
            }
        }
        if (!threw) {
            throw new IOException(name() + ": corrupted/ opened without raising the expected exception");
        }
    }

    /**
     * Writes a minimal, deterministic Lucene index identical in shape to
     * {@link SegmentInfoFormatScenario}: three documents with one
     * {@link StringField}.
     */
    private static void writeValid(Path dir) throws IOException {
        try (FSDirectory d = FSDirectory.open(dir);
             StandardAnalyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(d, iwc)) {
                for (int i = 0; i < 3; i++) {
                    Document doc = new Document();
                    doc.add(new StringField("id", "id-" + i, Field.Store.NO));
                    writer.addDocument(doc);
                }
                writer.commit();
            }
        }
    }

    /** Recursively copies the contents of {@code src} into {@code dst}. */
    private static void copyTree(Path src, Path dst) throws IOException {
        Files.createDirectories(dst);
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(src)) {
            for (Path p : entries) {
                Path target = dst.resolve(p.getFileName().toString());
                if (Files.isDirectory(p)) {
                    copyTree(p, target);
                } else {
                    Files.copy(p, target, StandardCopyOption.REPLACE_EXISTING);
                }
            }
        }
    }

    /**
     * Truncates the trailing {@link #TRUNCATE_BYTES} bytes of the
     * {@code segments_N} file inside {@code dir}. The trailing bytes are
     * the CodecUtil footer's CRC32 (and part of the magic); removing them
     * forces any reader that validates the footer to raise.
     */
    private static void truncateSegmentsFooter(Path dir) throws IOException {
        Path segmentsFile = null;
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(dir, "segments_*")) {
            for (Path p : entries) {
                segmentsFile = p; // exactly one is expected
            }
        }
        if (segmentsFile == null) {
            throw new IOException("no segments_N file in " + dir);
        }
        long size = Files.size(segmentsFile);
        if (size < TRUNCATE_BYTES) {
            throw new IOException("segments file " + segmentsFile + " is shorter than "
                    + TRUNCATE_BYTES + " bytes (size=" + size + ")");
        }
        try (RandomAccessFile raf = new RandomAccessFile(segmentsFile.toFile(), "rw")) {
            raf.setLength(size - TRUNCATE_BYTES);
        }
    }
}
