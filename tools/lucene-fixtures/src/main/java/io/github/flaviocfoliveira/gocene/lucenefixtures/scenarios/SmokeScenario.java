package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexInput;
import org.apache.lucene.store.IndexOutput;

import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.file.Path;

/**
 * Smoke scenario: a single {@code smoke.dat} file written through Lucene's
 * {@link CodecUtil} envelope.
 *
 * <p>Layout (Lucene 10.4.0 {@code CodecUtil} framing):
 * <pre>
 *   IndexHeader( codec="GoceneSmoke", version=0, id=16B(seed), suffix="" )
 *   int32   count = 4
 *   int64   value0 = seed
 *   int64   value1 = seed * 2
 *   int64   value2 = seed * 3
 *   int64   value3 = seed * 4
 *   Footer  ( FOOTER_MAGIC, algorithmId=0, CRC64 of preceding bytes )
 * </pre>
 *
 * <p>The scenario is bit-deterministic for a fixed seed: both Lucene and Gocene
 * MUST produce byte-identical output when given the same seed.
 */
public final class SmokeScenario implements CorpusScenario {

    public static final String NAME = "smoke";
    public static final String CODEC = "GoceneSmoke";
    public static final int VERSION = 0;
    public static final int COUNT = 4;
    public static final String FILE_NAME = "smoke.dat";

    @Override
    public String name() {
        return NAME;
    }

    @Override
    public String description() {
        return "Smoke fixture: 4-int64 payload wrapped in CodecUtil header/footer.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        if (target == null) {
            throw new IllegalArgumentException("target must not be null");
        }
        Path dir = target.toAbsolutePath();
        java.nio.file.Files.createDirectories(dir);
        try (FSDirectory directory = FSDirectory.open(dir);
             IndexOutput out = directory.createOutput(FILE_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, idFromSeed(seed), "");
            out.writeInt(COUNT);
            for (int i = 0; i < COUNT; i++) {
                out.writeLong(payloadValue(seed, i));
            }
            CodecUtil.writeFooter(out);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        if (source == null) {
            throw new IllegalArgumentException("source must not be null");
        }
        Path dir = source.toAbsolutePath();
        try (FSDirectory directory = FSDirectory.open(dir);
             ChecksumIndexInput in = directory.openChecksumInput(FILE_NAME)) {
            CodecUtil.checkIndexHeader(in, CODEC, VERSION, VERSION, idFromSeed(seed), "");
            int count = in.readInt();
            if (count != COUNT) {
                throw new IOException("smoke: count mismatch, expected "
                        + COUNT + ", got " + count);
            }
            for (int i = 0; i < COUNT; i++) {
                long v = in.readLong();
                long expected = payloadValue(seed, i);
                if (v != expected) {
                    throw new IOException("smoke: payload[" + i + "] mismatch, expected "
                            + expected + ", got " + v);
                }
            }
            CodecUtil.checkFooter(in);
        }
    }

    /** Deterministic 16-byte id derived from seed. */
    public static byte[] idFromSeed(long seed) {
        ByteBuffer buf = ByteBuffer.allocate(16);
        buf.putLong(seed);
        buf.putLong(~seed);
        return buf.array();
    }

    /** Deterministic payload generator: value[i] = seed * (i+1). */
    public static long payloadValue(long seed, int i) {
        return seed * (long) (i + 1);
    }

    /** Helper used by tests to read the raw bytes Lucene wrote (header/footer included). */
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
}
