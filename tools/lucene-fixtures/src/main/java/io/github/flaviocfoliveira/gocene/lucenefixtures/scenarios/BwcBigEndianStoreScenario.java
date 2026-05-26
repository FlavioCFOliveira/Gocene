package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.backward_codecs.store.EndiannessReverserUtil;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexInput;
import org.apache.lucene.store.IndexOutput;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T26 (rmp 4634): {@code bwc-big-endian-store}. Addresses the
 * backward_codecs audit row (verbatim):
 *
 * <pre>
 *   backward_codecs  Legacy big-endian store wrappers
 *       lucene_class: org.apache.lucene.backward_codecs.store.EndiannessReverserUtil
 *       gap_notes:    "No fixture from an old big-endian Lucene index."
 * </pre>
 *
 * <p>{@link EndiannessReverserUtil} is a writable surface in Lucene 10.4.0's
 * {@code lucene-backward-codecs} jar: pre-Lucene 8.6 indices are big-endian,
 * Lucene 8.6+ flipped to little-endian, and the reverser bridges the gap by
 * wrapping a fresh {@link IndexOutput} (or {@link IndexInput}) such that
 * primitive short/int/long writes are emitted in big-endian byte order while
 * everything else (vInt/vLong/string/byte/bytes) flows through unmodified.
 *
 * <p>File layout (single file {@value #FILE_NAME}):
 * <pre>
 *   bytes "BE\x00" + version int(BE)               // 3-byte ASCII tag + raw BE int
 *   vInt   count = 16                              // little-endian-agnostic varint
 *   for i in 0..count-1:
 *       short  (short)(seed + i)                   // BIG-endian on disk
 *       int    (int)(seed * (i + 5))               // BIG-endian on disk
 *       long   (seed ^ (long)i) << (i & 0x3F)      // BIG-endian on disk
 *       string "be-frame-" + i + "-seed-" + seed   // vInt-length + UTF-8 bytes
 * </pre>
 *
 * <p>No {@code CodecUtil} header/footer is used: pre-8.6 Lucene corpora pre-
 * date the universal big-endian header magic constants {@code BE_MAGIC} that
 * 10.4.0 still expects in the {@code .si} reader. The fixture intentionally
 * mirrors the lowest-level wire pattern the reverser was designed for: raw
 * BE primitives interleaved with vInt-encoded scalars, no codec framing.
 */
public final class BwcBigEndianStoreScenario implements CorpusScenario {

    public static final String NAME = "bwc-big-endian-store";
    public static final String FILE_NAME = "bwc-big-endian-store.dat";
    public static final int COUNT = 16;
    public static final int VERSION = 1;
    /** Magic ASCII tag preceding the version int, kept short on purpose. */
    public static final byte[] MAGIC = {(byte) 'B', (byte) 'E', (byte) 0};

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Big-endian store wrapper round-trip via EndiannessReverserUtil.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Path dir = target.toAbsolutePath();
        Files.createDirectories(dir);
        try (FSDirectory directory = FSDirectory.open(dir);
             IndexOutput out = EndiannessReverserUtil.createOutput(directory, FILE_NAME, IOContext.DEFAULT)) {
            // Raw magic + version int (BE because the wrapper is in scope).
            out.writeBytes(MAGIC, 0, MAGIC.length);
            out.writeInt(VERSION);
            out.writeVInt(COUNT);
            for (int i = 0; i < COUNT; i++) {
                out.writeShort(shortValue(seed, i));
                out.writeInt(intValue(seed, i));
                out.writeLong(longValue(seed, i));
                out.writeString(stringValue(seed, i));
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path dir = source.toAbsolutePath();

        // Pass 1: read through the reverser wrapper (logical readback).
        try (FSDirectory directory = FSDirectory.open(dir);
             IndexInput in = EndiannessReverserUtil.openInput(directory, FILE_NAME, IOContext.DEFAULT)) {
            byte[] tag = new byte[MAGIC.length];
            in.readBytes(tag, 0, tag.length);
            for (int k = 0; k < MAGIC.length; k++) {
                if (tag[k] != MAGIC[k]) {
                    throw new IOException(NAME + ": magic mismatch at byte " + k);
                }
            }
            int version = in.readInt();
            if (version != VERSION) {
                throw new IOException(NAME + ": version mismatch, expected " + VERSION + ", got " + version);
            }
            int count = in.readVInt();
            if (count != COUNT) {
                throw new IOException(NAME + ": count mismatch, expected " + COUNT + ", got " + count);
            }
            for (int i = 0; i < COUNT; i++) {
                short s = in.readShort();
                if (s != shortValue(seed, i)) {
                    throw new IOException(NAME + ": short[" + i + "] mismatch");
                }
                int v = in.readInt();
                if (v != intValue(seed, i)) {
                    throw new IOException(NAME + ": int[" + i + "] mismatch");
                }
                long L = in.readLong();
                if (L != longValue(seed, i)) {
                    throw new IOException(NAME + ": long[" + i + "] mismatch");
                }
                String want = stringValue(seed, i);
                String got = in.readString();
                if (!want.equals(got)) {
                    throw new IOException(NAME + ": string[" + i + "] mismatch want='"
                            + want + "' got='" + got + "'");
                }
            }
        }

        // Pass 2: byte-level proof that the short/int/long fields are stored
        // big-endian on disk. Open WITHOUT the reverser and re-read the first
        // record's short field, asserting the LE-interpretation differs from
        // the BE-expected value (this guarantees the wrapper actually
        // reversed endianness rather than being a no-op).
        try (FSDirectory directory = FSDirectory.open(dir);
             ChecksumIndexInput raw = directory.openChecksumInput(FILE_NAME)) {
            raw.skipBytes(MAGIC.length);                // skip the 3-byte tag
            int rawVersionLE = raw.readInt();           // LE read of the BE-stored version
            int expectBEAsLE = Integer.reverseBytes(VERSION);
            if (rawVersionLE != expectBEAsLE) {
                throw new IOException(NAME + ": raw LE-read of version did not match "
                        + "reverseBytes(VERSION); got " + rawVersionLE + " want " + expectBEAsLE
                        + " (reverser may not be writing big-endian)");
            }
        }
    }

    public static short shortValue(long seed, int i) {
        return (short) (seed + i);
    }

    public static int intValue(long seed, int i) {
        return (int) (seed * (long) (i + 5));
    }

    public static long longValue(long seed, int i) {
        return (seed ^ (long) i) << (i & 0x3F);
    }

    public static String stringValue(long seed, int i) {
        return "be-frame-" + i + "-seed-" + seed;
    }
}
