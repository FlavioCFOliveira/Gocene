package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.store.ByteArrayDataOutput;
import org.apache.lucene.store.ByteBuffersDataOutput;
import org.apache.lucene.store.DataInput;
import org.apache.lucene.store.DataOutput;
import org.apache.lucene.util.packed.PackedInts;
import org.apache.lucene.util.packed.PackedInts.Format;
import org.apache.lucene.util.packed.PackedInts.Writer;
import org.apache.lucene.util.packed.PackedInts.ReaderIterator;

import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Random;

/**
 * Sprint 8 T18 (rmp 140): byte-compatibility fixture for Packed64 and
 * Packed64SingleBlock formats.
 *
 * <p>This scenario writes seeded arrays at every supported bitsPerValue
 * through Lucene 10.4.0's {@link PackedInts#getWriterNoHeader} for both
 * {@link Format#PACKED} and {@link Format#PACKED_SINGLE_BLOCK} and
 * writes the raw encoded bytes to {@value #FILE_NAME}.
 *
 * <p>The file layout is:
 * <pre>
 *   bytes "PKD64FMT\x00" + version int (BE)
 *   int   numFormats (2: PACKED=0, PACKED_SINGLE_BLOCK=1)
 *   for each format:
 *     int  formatId
 *     int  numBpvValues
 *     for each bpv:
 *       int    bitsPerValue
 *       int    valueCount
 *       long[] encodedPayload (raw PackedWriter bytes)
 * </pre>
 */
public final class PackedIntsPacked64Scenario implements CorpusScenario {

    public static final String NAME = "packed-ints-packed64";
    public static final String FILE_NAME = "packed-ints-packed64.bin";
    public static final int VERSION = 1;
    public static final byte[] MAGIC = {
            (byte) 'P', (byte) 'K', (byte) 'D', (byte) '6',
            (byte) '4', (byte) 'F', (byte) 'M', (byte) 'T', (byte) 0};

    /** Bits-per-value spectrum for PACKED format: 1..64 stepping through
     *  the canonical test spectrum. */
    static final int[] PACKED_BPV = {
            1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 15, 16, 20, 24, 32, 40, 48, 56, 63, 64};

    /** Bits-per-value spectrum for PACKED_SINGLE_BLOCK: only the supported set. */
    static final int[] SINGLE_BLOCK_BPV = {
            1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 16, 21, 32};

    /** Number of values written per bpv. */
    static final int VALUES_PER_BPV = 128;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Packed64 and Packed64SingleBlock byte-exact encoded payloads "
                + "at every supported bitsPerValue from Lucene PackedInts.";
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
        buf.writeInt(2); // two formats

        // Format.PACKED
        buf.writeInt(Format.PACKED.getId());
        buf.writeInt(PACKED_BPV.length);
        for (int bpv : PACKED_BPV) {
            long[] values = deterministicValues(seed, bpv, VALUES_PER_BPV);
            byte[] payload = encodeViaWriter(Format.PACKED, values);
            buf.writeInt(bpv);
            buf.writeInt(VALUES_PER_BPV);
            buf.writeInt(payload.length);
            buf.writeBytes(payload, 0, payload.length);
        }

        // Format.PACKED_SINGLE_BLOCK
        buf.writeInt(Format.PACKED_SINGLE_BLOCK.getId());
        buf.writeInt(SINGLE_BLOCK_BPV.length);
        for (int bpv : SINGLE_BLOCK_BPV) {
            long[] values = deterministicValues(seed, bpv, VALUES_PER_BPV);
            byte[] payload = encodeViaWriter(Format.PACKED_SINGLE_BLOCK, values);
            buf.writeInt(bpv);
            buf.writeInt(VALUES_PER_BPV);
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

        // Read magic
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
        int numFormats = bb.getInt();
        if (numFormats != 2) {
            throw new IOException(NAME + ": expected 2 formats, got " + numFormats);
        }

        for (int f = 0; f < numFormats; f++) {
            int formatId = bb.getInt();
            Format format = Format.byId(formatId);
            int[] bpvSpectrum = (format == Format.PACKED_SINGLE_BLOCK) ? SINGLE_BLOCK_BPV : PACKED_BPV;
            int numBpv = bb.getInt();
            if (numBpv != bpvSpectrum.length) {
                throw new IOException(NAME + ": format " + format + " bpv count mismatch");
            }
            for (int i = 0; i < numBpv; i++) {
                int bpv = bb.getInt();
                int valueCount = bb.getInt();
                int payloadLen = bb.getInt();
                byte[] payload = new byte[payloadLen];
                bb.get(payload);
                // Skip: check format id and bpv are consistent
                // (the Go test side does full round-trip verification)
                boolean found = false;
                for (int expected : bpvSpectrum) {
                    if (expected == bpv) { found = true; break; }
                }
                if (!found) {
                    throw new IOException(NAME + ": unexpected bpv " + bpv + " in format " + format);
                }
                if (valueCount != VALUES_PER_BPV) {
                    throw new IOException(NAME + ": valueCount mismatch");
                }
            }
        }
        if (bb.remaining() != 0) {
            throw new IOException(NAME + ": trailing bytes");
        }
    }

    /** Encodes values through PackedInts.getWriterNoHeader and returns
     *  the raw byte payload (headerless big-endian). */
    static byte[] encodeViaWriter(Format format, long[] values) throws IOException {
        ByteBuffersDataOutput buf = ByteBuffersDataOutput.newResettableInstance();
        Writer writer = PackedInts.getWriterNoHeader(buf, format, values.length,
                PackedInts.bitsRequired(max(values)), PackedInts.DEFAULT_BUFFER_SIZE);
        for (long v : values) writer.add(v);
        writer.finish();
        return buf.toArrayCopy();
    }

    static long max(long[] values) {
        long m = Long.MIN_VALUE;
        for (long v : values) if (v > m) m = v;
        return m;
    }

    /** Deterministic generator: produces non-negative values in [0, 2^bpv).
     *  For bpv==64 uses the full long range. */
    static long[] deterministicValues(long seed, int bpv, int n) {
        Random rng = new Random(seed ^ (0x9E3779B97F4A7C15L * bpv));
        long[] out = new long[n];
        if (bpv == 64) {
            for (int i = 0; i < n; i++) out[i] = rng.nextLong();
            return out;
        }
        long mask = (1L << bpv) - 1L;
        for (int i = 0; i < n; i++) {
            out[i] = rng.nextLong() & mask;
        }
        return out;
    }
}
