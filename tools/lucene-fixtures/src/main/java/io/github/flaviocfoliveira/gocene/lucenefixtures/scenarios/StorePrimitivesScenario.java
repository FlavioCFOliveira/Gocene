package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexInput;
import org.apache.lucene.store.IndexOutput;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Store-primitives scenario: a single {@code store-primitives.dat} file that
 * exercises every primitive serialisation method on Lucene's
 * {@link org.apache.lucene.store.DataOutput} contract.
 *
 * <p>The scenario is the audit-row companion of the Sprint 114 T6 task and
 * covers, by construction, IndexInput/IndexOutput primitives (vInt, vLong,
 * zigzag, string), MMapDirectory / NIOFSDirectory file IO, BufferedChecksum
 * CRC32, NativeFSLockFactory file-naming, RateLimitedIndexOutput byte-fidelity
 * and FSDirectory file naming.
 *
 * <p>Layout (Lucene 10.4.0 {@link CodecUtil} framing):
 * <pre>
 *   IndexHeader( codec="GoceneStorePrimitives", version=0, id=16B(seed), suffix="" )
 *   vInt    count = 8
 *   for i in 0..count-1:
 *       vInt    (int)(seed * (i+1) & 0x7FFFFFFF)
 *       vLong   seed * (long)(i+3)
 *       zInt    (int)(seed * (i+1) - 7)
 *       zLong   seed * (long)(i+1) * -3
 *       string  "frame-" + i + "-seed-" + seed
 *       byte    (byte) i
 *       short   (short)(seed + i)       (LE on disk)
 *       int     (int)(seed * (i+5))     (LE on disk)
 *       long    seed &lt;&lt; i             (LE on disk)
 *   Footer  ( FOOTER_MAGIC, algorithmId=0, CRC32 of preceding bytes )
 * </pre>
 *
 * <p>Byte-determinism: identical seeds MUST produce byte-identical bytes
 * across Lucene runs and across the Gocene Go-side mirror.
 */
public final class StorePrimitivesScenario implements CorpusScenario {

    public static final String NAME = "store-primitives";
    public static final String CODEC = "GoceneStorePrimitives";
    public static final int VERSION = 0;
    public static final int COUNT = 8;
    public static final String FILE_NAME = "store-primitives.dat";

    @Override
    public String name() {
        return NAME;
    }

    @Override
    public String description() {
        return "Store primitives fixture: vInt/vLong/zInt/zLong/string/byte/short/int/long per frame.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        if (target == null) {
            throw new IllegalArgumentException("target must not be null");
        }
        Determinism.seed(seed);
        Path dir = target.toAbsolutePath();
        Files.createDirectories(dir);
        try (FSDirectory directory = FSDirectory.open(dir);
             IndexOutput out = directory.createOutput(FILE_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, Determinism.idBytes(seed), "");
            out.writeVInt(COUNT);
            for (int i = 0; i < COUNT; i++) {
                out.writeVInt(vIntValue(seed, i));
                out.writeVLong(vLongValue(seed, i));
                out.writeZInt(zIntValue(seed, i));
                out.writeZLong(zLongValue(seed, i));
                out.writeString(stringValue(seed, i));
                out.writeByte((byte) i);
                out.writeShort(shortValue(seed, i));
                out.writeInt(intValue(seed, i));
                out.writeLong(longValue(seed, i));
            }
            CodecUtil.writeFooter(out);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        if (source == null) {
            throw new IllegalArgumentException("source must not be null");
        }
        Determinism.seed(seed);
        Path dir = source.toAbsolutePath();
        try (FSDirectory directory = FSDirectory.open(dir);
             ChecksumIndexInput in = directory.openChecksumInput(FILE_NAME)) {
            CodecUtil.checkIndexHeader(in, CODEC, VERSION, VERSION, Determinism.idBytes(seed), "");
            int count = in.readVInt();
            if (count != COUNT) {
                throw new IOException("store-primitives: count mismatch, expected "
                        + COUNT + ", got " + count);
            }
            for (int i = 0; i < COUNT; i++) {
                expectVInt(in, i, vIntValue(seed, i));
                expectVLong(in, i, vLongValue(seed, i));
                expectZInt(in, i, zIntValue(seed, i));
                expectZLong(in, i, zLongValue(seed, i));
                expectString(in, i, stringValue(seed, i));
                expectByte(in, i, (byte) i);
                expectShort(in, i, shortValue(seed, i));
                expectInt(in, i, intValue(seed, i));
                expectLong(in, i, longValue(seed, i));
            }
            CodecUtil.checkFooter(in);
        }
    }

    // ---- deterministic frame generators ----

    public static int vIntValue(long seed, int i) {
        return (int) (seed * (long) (i + 1) & 0x7FFFFFFFL);
    }

    public static long vLongValue(long seed, int i) {
        return seed * (long) (i + 3);
    }

    public static int zIntValue(long seed, int i) {
        return (int) (seed * (long) (i + 1) - 7L);
    }

    public static long zLongValue(long seed, int i) {
        return seed * (long) (i + 1) * -3L;
    }

    public static String stringValue(long seed, int i) {
        return "frame-" + i + "-seed-" + seed;
    }

    public static short shortValue(long seed, int i) {
        return (short) (seed + i);
    }

    public static int intValue(long seed, int i) {
        return (int) (seed * (long) (i + 5));
    }

    public static long longValue(long seed, int i) {
        // Java's `seed << i` masks the shift count with & 0x3F; we mirror that
        // behaviour in Go by masking explicitly to keep the two engines aligned.
        return seed << (i & 0x3F);
    }

    /** Helper used by unit tests to inspect the raw bytes Lucene wrote. */
    public static byte[] readAllBytes(Path target) throws IOException {
        Path dir = target.toAbsolutePath();
        try (FSDirectory directory = FSDirectory.open(dir);
             IndexInput in = directory.openInput(FILE_NAME, IOContext.DEFAULT)) {
            long len = in.length();
            byte[] bytes = new byte[Math.toIntExact(len)];
            in.readBytes(bytes, 0, bytes.length);
            return bytes;
        }
    }

    // ---- assertion helpers ----

    private static void expectVInt(ChecksumIndexInput in, int i, int expected) throws IOException {
        int v = in.readVInt();
        if (v != expected) {
            throw new IOException("store-primitives: vInt[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }

    private static void expectVLong(ChecksumIndexInput in, int i, long expected) throws IOException {
        long v = in.readVLong();
        if (v != expected) {
            throw new IOException("store-primitives: vLong[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }

    private static void expectZInt(ChecksumIndexInput in, int i, int expected) throws IOException {
        int v = in.readZInt();
        if (v != expected) {
            throw new IOException("store-primitives: zInt[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }

    private static void expectZLong(ChecksumIndexInput in, int i, long expected) throws IOException {
        long v = in.readZLong();
        if (v != expected) {
            throw new IOException("store-primitives: zLong[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }

    private static void expectString(ChecksumIndexInput in, int i, String expected) throws IOException {
        String v = in.readString();
        if (!expected.equals(v)) {
            throw new IOException("store-primitives: string[" + i + "] mismatch, expected '"
                    + expected + "', got '" + v + "'");
        }
    }

    private static void expectByte(ChecksumIndexInput in, int i, byte expected) throws IOException {
        byte v = in.readByte();
        if (v != expected) {
            throw new IOException("store-primitives: byte[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }

    private static void expectShort(ChecksumIndexInput in, int i, short expected) throws IOException {
        short v = in.readShort();
        if (v != expected) {
            throw new IOException("store-primitives: short[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }

    private static void expectInt(ChecksumIndexInput in, int i, int expected) throws IOException {
        int v = in.readInt();
        if (v != expected) {
            throw new IOException("store-primitives: int[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }

    private static void expectLong(ChecksumIndexInput in, int i, long expected) throws IOException {
        long v = in.readLong();
        if (v != expected) {
            throw new IOException("store-primitives: long[" + i + "] mismatch, expected "
                    + expected + ", got " + v);
        }
    }
}
