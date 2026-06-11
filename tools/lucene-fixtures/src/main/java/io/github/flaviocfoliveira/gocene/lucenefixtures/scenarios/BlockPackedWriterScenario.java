package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.store.ByteBuffersDataOutput;
import org.apache.lucene.util.packed.BlockPackedWriter;
import org.apache.lucene.util.packed.MonotonicBlockPackedWriter;

import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Random;

/**
 * Sprint 8 T18 (rmp 140): byte-compatibility fixture for BlockPackedWriter
 * and MonotonicBlockPackedWriter.
 *
 * <p>This scenario writes seeded arrays at various block sizes through
 * Lucene 10.4.0's {@link BlockPackedWriter} and
 * {@link MonotonicBlockPackedWriter} and stores the raw encoded bytes.
 *
 * <p>File: {@value #FILE_NAME}
 * <pre>
 *   bytes "BLKPCK\x00" + version int (BE)
 *   int   numCases
 *   for each case:
 *     byte writerType (0=BlockPackedWriter, 1=MonotonicBlockPackedWriter)
 *     int  blockSize
 *     int  valueCount
 *     byte[] encodedPayload (raw writer bytes)
 * </pre>
 */
public final class BlockPackedWriterScenario implements CorpusScenario {

    public static final String NAME = "block-packed-writer";
    public static final String FILE_NAME = "block-packed-writer.bin";
    public static final int VERSION = 1;
    public static final byte[] MAGIC = {
            (byte) 'B', (byte) 'L', (byte) 'K', (byte) 'P',
            (byte) 'C', (byte) 'K', (byte) 0};

    /** Test case: (writerType, blockSize, numValues, valueFactory). */
    static final int WRITER_BLOCK_PACKED = 0;
    static final int WRITER_MONOTONIC_BLOCK_PACKED = 1;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "BlockPackedWriter and MonotonicBlockPackedWriter at various block sizes.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Path dir = target.toAbsolutePath();
        Files.createDirectories(dir);
        Path file = dir.resolve(FILE_NAME);
        ByteBuffersDataOutput buf = ByteBuffersDataOutput.newResettableInstance();
        buf.writeBytes(MAGIC, 0, MAGIC.length);
        buf.writeInt(VERSION);

        // Encode each test case and count them first.
        var cases = createCases(seed);
        buf.writeInt(cases.length);
        for (var c : cases) {
            long[] values = generateValues(seed, c);
            byte[] payload = encode(c.writerType, c.blockSize, values);
            buf.writeByte((byte) c.writerType);
            buf.writeInt(c.blockSize);
            buf.writeInt(values.length);
            buf.writeInt(payload.length);
            buf.writeBytes(payload, 0, payload.length);
        }
        Files.write(file, buf.toArrayCopy());
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path dir = source.toAbsolutePath();
        Path file = dir.resolve(FILE_NAME);
        byte[] all = Files.readAllBytes(file);
        ByteBuffer bb = ByteBuffer.wrap(all).order(ByteOrder.BIG_ENDIAN);

        byte[] magic = new byte[MAGIC.length];
        bb.get(magic);
        for (int i = 0; i < MAGIC.length; i++) {
            if (magic[i] != MAGIC[i]) {
                throw new IOException(NAME + ": magic mismatch at byte " + i);
            }
        }
        int version = bb.getInt();
        if (version != VERSION) {
            throw new IOException(NAME + ": version mismatch");
        }
        int numCases = bb.getInt();
        if (numCases <= 0) {
            throw new IOException(NAME + ": no cases");
        }
        var cases = createCases(seed);
        if (numCases != cases.length) {
            throw new IOException(NAME + ": case count mismatch");
        }
        for (int i = 0; i < numCases; i++) {
            int writerType = Byte.toUnsignedInt(bb.get());
            int blockSize = bb.getInt();
            int valueCount = bb.getInt();
            int payloadLen = bb.getInt();
            bb.position(bb.position() + payloadLen); // skip payload
            if (writerType != cases[i].writerType
                    || blockSize != cases[i].blockSize
                    || valueCount != cases[i].valueCount) {
                throw new IOException(NAME + ": case " + i + " header mismatch");
            }
        }
        if (bb.remaining() != 0) {
            throw new IOException(NAME + ": trailing bytes");
        }
    }

    static byte[] encode(int writerType, int blockSize, long[] values) throws IOException {
        ByteBuffersDataOutput buf = ByteBuffersDataOutput.newResettableInstance();
        switch (writerType) {
            case WRITER_BLOCK_PACKED -> {
                BlockPackedWriter w = new BlockPackedWriter(buf, blockSize);
                for (long v : values) w.add(v);
                w.finish();
            }
            case WRITER_MONOTONIC_BLOCK_PACKED -> {
                MonotonicBlockPackedWriter w = new MonotonicBlockPackedWriter(buf, blockSize);
                for (long v : values) w.add(v);
                w.finish();
            }
            default -> throw new IOException("unknown writer type: " + writerType);
        }
        return buf.toArrayCopy();
    }

    static long[] generateValues(long seed, Case c) {
        Random rng = new Random(seed ^ (0x9E3779B97F4A7C15L * c.blockSize * (c.writerType + 1)));
        long[] out = new long[c.valueCount];
        switch (c.factory) {
            case 0 -> { // Small signed values
                for (int i = 0; i < c.valueCount; i++) {
                    out[i] = rng.nextInt(2000) - 1000;
                }
            }
            case 1 -> { // Quadratic ramp
                for (int i = 0; i < c.valueCount; i++) {
                    out[i] = (long) i * i;
                }
            }
            case 2 -> { // Monotonic tight range
                long base = rng.nextLong() & 0xFFFFFFFFL;
                for (int i = 0; i < c.valueCount; i++) {
                    out[i] = base + rng.nextInt(100);
                }
            }
            case 3 -> { // Strictly monotonic large span
                long base = (rng.nextLong() & 0xFFFFFFFFL) - (1L << 31);
                for (int i = 0; i < c.valueCount; i++) {
                    out[i] = base + i * 37L;
                }
            }
            case 4 -> { // All identical
                java.util.Arrays.fill(out, 42L);
            }
        }
        return out;
    }

    static Case[] createCases(long seed) {
        // BlockPackedWriter cases
        Case c1 = new Case(WRITER_BLOCK_PACKED, 64, 10, 0);
        Case c2 = new Case(WRITER_BLOCK_PACKED, 64, 200, 1);
        Case c3 = new Case(WRITER_BLOCK_PACKED, 128, 300, 2);
        Case c4 = new Case(WRITER_BLOCK_PACKED, 256, 500, 0);
        Case c5 = new Case(WRITER_BLOCK_PACKED, 512, 4, 4);
        // MonotonicBlockPackedWriter cases
        Case c6 = new Case(WRITER_MONOTONIC_BLOCK_PACKED, 64, 200, 3);
        Case c7 = new Case(WRITER_MONOTONIC_BLOCK_PACKED, 128, 320, 2);
        Case c8 = new Case(WRITER_MONOTONIC_BLOCK_PACKED, 64, 128, 4); // all zero/identical
        return new Case[]{c1, c2, c3, c4, c5, c6, c7, c8};
    }

    record Case(int writerType, int blockSize, int valueCount, int factory) {}
}
