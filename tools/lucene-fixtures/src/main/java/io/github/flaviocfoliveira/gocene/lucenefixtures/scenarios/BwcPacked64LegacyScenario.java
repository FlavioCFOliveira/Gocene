package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.backward_codecs.packed.LegacyDirectReader;
import org.apache.lucene.backward_codecs.packed.LegacyDirectWriter;
import org.apache.lucene.backward_codecs.store.EndiannessReverserUtil;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexInput;
import org.apache.lucene.store.IndexOutput;
import org.apache.lucene.util.LongValues;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Random;

/**
 * Sprint 114 T26 (rmp 4634): {@code bwc-packed64-legacy}. Addresses the
 * backward_codecs audit row (verbatim):
 *
 * <pre>
 *   backward_codecs  Legacy Packed64 / Packed64SingleBlock
 *       lucene_class: org.apache.lucene.backward_codecs.packed.LegacyPacked64
 *       gap_notes:    "No Lucene fixture; covered by self-roundtrip only."
 * </pre>
 *
 * <p>{@link LegacyDirectWriter} and {@link LegacyDirectReader} ARE writable
 * surfaces in Lucene 10.4.0's {@code lucene-backward-codecs} jar (unlike the
 * per-version codec FieldsConsumer paths which throw
 * {@code UnsupportedOperationException}). The scenario writes a seeded array
 * of {@code (long)} values at every supported {@code bitsPerValue} into a
 * single file {@value #FILE_NAME} via the legacy writer, then re-reads it
 * back through {@link LegacyDirectReader#getInstance} and asserts every value
 * round-trips.
 *
 * <p>The Legacy Packed/Direct format predates Lucene's universal LE flip
 * (Lucene 8.6), so the entire payload is written through
 * {@link EndiannessReverserUtil#createOutput} in BIG-endian byte order —
 * mirrored from {@code TestLegacyDirectPacked} in the Lucene 10.4.0 source
 * tree. No {@code CodecUtil} header/footer is used: pre-8.6 Lucene corpora
 * predate the current {@code CodecUtil} envelope (which is LE-only).
 *
 * <p>File layout (single file {@value #FILE_NAME}, all BE on disk):
 * <pre>
 *   bytes "BWCPKD64\x00" + version int                // 9-byte ASCII tag + raw BE int
 *   vInt   bpvCount = 14                              // length of BITS_PER_VALUE
 *   for bpv in {1, 2, 4, 8, 12, 16, 20, 24, 28, 32, 40, 48, 56, 64}:
 *       vInt   bpv
 *       vInt   numValues (= 16)
 *       (LegacyDirectWriter byte payload + 3 trailing pad bytes)
 * </pre>
 */
public final class BwcPacked64LegacyScenario implements CorpusScenario {

    public static final String NAME = "bwc-packed64-legacy";
    public static final String FILE_NAME = "bwc-packed64-legacy.dat";
    public static final int VERSION = 1;
    public static final byte[] MAGIC = {
            (byte) 'B', (byte) 'W', (byte) 'C', (byte) 'P', (byte) 'K',
            (byte) 'D', (byte) '6', (byte) '4', (byte) 0};

    /** Supported bitsPerValue per LegacyDirectWriter.SUPPORTED_BITS_PER_VALUE. */
    static final int[] BITS_PER_VALUE = {1, 2, 4, 8, 12, 16, 20, 24, 28, 32, 40, 48, 56, 64};

    /** Number of values written at each bitsPerValue. Keep modest: this is a
     *  determinism gate, not a stress test. */
    static final int NUM_VALUES_PER_BPV = 16;

    /** Pad bytes LegacyDirectWriter.finish appends after the packed payload. */
    static final int PAD_BYTES = 3;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Legacy Packed64 / DirectWriter at every supported bitsPerValue, "
                + "round-tripped through LegacyDirectReader (BE on disk).";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Path dir = target.toAbsolutePath();
        Files.createDirectories(dir);
        try (FSDirectory directory = FSDirectory.open(dir);
             IndexOutput out = EndiannessReverserUtil.createOutput(directory, FILE_NAME, IOContext.DEFAULT)) {
            out.writeBytes(MAGIC, 0, MAGIC.length);
            out.writeInt(VERSION);
            out.writeVInt(BITS_PER_VALUE.length);
            for (int bpv : BITS_PER_VALUE) {
                long[] values = deterministicValues(seed, bpv, NUM_VALUES_PER_BPV);
                out.writeVInt(bpv);
                out.writeVInt(NUM_VALUES_PER_BPV);
                LegacyDirectWriter writer = LegacyDirectWriter.getInstance(out, NUM_VALUES_PER_BPV, bpv);
                for (long v : values) writer.add(v);
                writer.finish();
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path dir = source.toAbsolutePath();
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
            int bpvCount = in.readVInt();
            if (bpvCount != BITS_PER_VALUE.length) {
                throw new IOException(NAME + ": bpvCount mismatch, expected "
                        + BITS_PER_VALUE.length + ", got " + bpvCount);
            }
            for (int bpv : BITS_PER_VALUE) {
                int readBpv = in.readVInt();
                int readCount = in.readVInt();
                if (readBpv != bpv || readCount != NUM_VALUES_PER_BPV) {
                    throw new IOException(NAME + ": header mismatch at bpv=" + bpv
                            + " readBpv=" + readBpv + " readCount=" + readCount);
                }
                long startFp = in.getFilePointer();
                long payloadBytes = payloadBytes(bpv, NUM_VALUES_PER_BPV);
                // The Lucene DirectPackedReaders read int/long-aligned chunks
                // from random-access slices, so the slice must include the 3
                // trailing pad bytes finish() writes. Hand the entire
                // remaining tail to the reader; the BPV-aware reader indexes
                // by element so the padding does not corrupt readback.
                long sliceLen = payloadBytes + PAD_BYTES;
                LongValues legacy = LegacyDirectReader.getInstance(
                        in.randomAccessSlice(startFp, sliceLen), bpv);
                long[] want = deterministicValues(seed, bpv, NUM_VALUES_PER_BPV);
                for (int i = 0; i < NUM_VALUES_PER_BPV; i++) {
                    long got = legacy.get(i);
                    if (got != want[i]) {
                        throw new IOException(NAME + ": value mismatch bpv=" + bpv
                                + " i=" + i + " want=" + want[i] + " got=" + got);
                    }
                }
                in.seek(startFp + payloadBytes + PAD_BYTES);
            }
            if (in.getFilePointer() != in.length()) {
                throw new IOException(NAME + ": trailing bytes after last bpv section: fp="
                        + in.getFilePointer() + " length=" + in.length());
            }
        }
    }

    /** Returns ceil(numValues * bpv / 8). */
    static long payloadBytes(int bpv, int numValues) {
        return ((long) numValues * bpv + 7L) / 8L;
    }

    /** Deterministic generator: a per-(seed,bpv) seeded {@link Random} stream
     *  produces non-negative values in {@code [0, 2^bpv)}. {@code bpv==64}
     *  uses the full long range via {@code Random.nextLong()}. */
    public static long[] deterministicValues(long seed, int bpv, int n) {
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
