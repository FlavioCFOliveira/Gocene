package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.store.ByteBuffersDataOutput;
import org.apache.lucene.store.IndexOutput;
import org.apache.lucene.util.packed.DirectMonotonicWriter;
import org.apache.lucene.store.Directory;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;

import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Random;

/**
 * Sprint 8 T18 (rmp 140): byte-compatibility fixture for DirectMonotonicWriter
 * and DirectMonotonicReader.
 *
 * <p>This scenario writes seeded monotonic sequences through Lucene 10.4.0's
 * {@link DirectMonotonicWriter} and stores the raw meta and data streams.
 *
 * <p>File: {@value #FILE_NAME}
 * <pre>
 *   bytes "DRMONO\x00" + version int (BE)
 *   int   numCases
 *   for each case:
 *     int  blockShift
 *     int  valueCount
 *     long dataStreamLength (after the writer writes it to its .dat file)
 *     byte[] metaBytes  (the meta IndexOutput content)
 *     byte[] dataBytes  (the data IndexOutput content)
 * </pre>
 */
public final class DirectMonotonicBlockPackedScenario implements CorpusScenario {

    public static final String NAME = "direct-monotonic";
    public static final String FILE_NAME = "direct-monotonic.bin";
    public static final String DATA_FILE_NAME = "direct-monotonic-data.tmp";
    public static final int VERSION = 1;
    public static final byte[] MAGIC = {
            (byte) 'D', (byte) 'R', (byte) 'M', (byte) 'O',
            (byte) 'N', (byte) 'O', (byte) 0};

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "DirectMonotonicWriter meta+data streams for various block shifts and sequences.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Path dir = target.toAbsolutePath();
        Files.createDirectories(dir);
        Path file = dir.resolve(FILE_NAME);
        try (Directory directory = FSDirectory.open(dir)) {
            var cases = createCases(seed);
            ByteBuffersDataOutput buf = ByteBuffersDataOutput.newResettableInstance();
            buf.writeBytes(MAGIC, 0, MAGIC.length);
            buf.writeInt(VERSION);
            buf.writeInt(cases.length);

            for (int ci = 0; ci < cases.length; ci++) {
                var c = cases[ci];
                long[] values = generateValues(seed, c);

                // Write meta+data using Lucene's DirectMonotonicWriter.
                String metaName = "dm-meta-" + ci + ".tmp";
                String dataName = "dm-data-" + ci + ".tmp";
                try (IndexOutput metaOut = directory.createOutput(metaName, IOContext.DEFAULT);
                     IndexOutput dataOut = directory.createOutput(dataName, IOContext.DEFAULT)) {
                    DirectMonotonicWriter w = DirectMonotonicWriter.getInstance(
                            metaOut, dataOut, values.length, c.blockShift);
                    for (long v : values) w.add(v);
                    w.finish();
                }
                byte[] metaBytes = Files.readAllBytes(dir.resolve(metaName));
                byte[] dataBytes = Files.readAllBytes(dir.resolve(dataName));
                Files.deleteIfExists(dir.resolve(metaName));
                Files.deleteIfExists(dir.resolve(dataName));

                buf.writeInt(c.blockShift);
                buf.writeInt(values.length);
                buf.writeInt(metaBytes.length);
                buf.writeBytes(metaBytes, 0, metaBytes.length);
                buf.writeInt(dataBytes.length);
                buf.writeBytes(dataBytes, 0, dataBytes.length);
            }
            Files.write(file, buf.toArrayCopy());
            // Clean up any remaining tmp files from FSDirectory.
            try (var listing = Files.newDirectoryStream(dir, p -> p.getFileName().toString().endsWith(".tmp"))) {
                for (var p : listing) Files.deleteIfExists(p);
            }
        }
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
        if (version != VERSION) throw new IOException(NAME + ": version mismatch");
        int numCases = bb.getInt();
        if (numCases <= 0) throw new IOException(NAME + ": no cases");
        for (int i = 0; i < numCases; i++) {
            int blockShift = bb.getInt();
            int valueCount = bb.getInt();
            int metaLen = bb.getInt();
            bb.position(bb.position() + metaLen);
            int dataLen = bb.getInt();
            bb.position(bb.position() + dataLen);
        }
        if (bb.remaining() != 0) throw new IOException(NAME + ": trailing bytes");
    }

    static long[] generateValues(long seed, Case c) {
        Random rng = new Random(seed ^ (0x9E3779B97F4A7C15L * c.blockShift * (c.factory + 1)));
        long[] out = new long[c.valueCount];
        switch (c.factory) {
            case 0 -> { // Perfectly linear
                long base = rng.nextLong() & 0xFFFFFFFFL;
                long inc = rng.nextInt(100) + 1;
                for (int i = 0; i < c.valueCount; i++) out[i] = base + inc * i;
            }
            case 1 -> { // Slightly noisy linear
                long base = (rng.nextLong() & 0xFFFFFFFFL) - (1L << 31);
                long inc = rng.nextInt(1000) + 1;
                for (int i = 0; i < c.valueCount; i++) {
                    out[i] = base + inc * i + rng.nextInt(5);
                }
            }
            case 2 -> { // All zero
                // already zero
            }
            case 3 -> { // Strictly increasing, varying increments
                long cur = (rng.nextLong() & 0xFFFFFFFFL) - (1L << 31);
                for (int i = 0; i < c.valueCount; i++) {
                    out[i] = cur;
                    cur += rng.nextInt(1 << 20) + 1;
                }
            }
        }
        return out;
    }

    static Case[] createCases(long seed) {
        Case c1 = new Case(4, 64, 0);   // perfectly linear, small block
        Case c2 = new Case(6, 256, 0);  // perfectly linear, medium block
        Case c3 = new Case(8, 300, 1);  // noisy, multi-block
        Case c4 = new Case(2, 8, 2);    // all zero, small
        Case c5 = new Case(5, 128, 3);  // varying inc, larger
        return new Case[]{c1, c2, c3, c4, c5};
    }

    record Case(int blockShift, int valueCount, int factory) {}
}
