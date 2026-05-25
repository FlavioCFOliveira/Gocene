package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.geo.GeoEncodingUtils;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexOutput;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T20 (rmp 4628): {@code geo-encoded-points}.
 *
 * <p>Audit row covered (verbatim): "No fixture comparing encoded points
 * emitted by Lucene; relies on algorithmic equivalence." for
 * {@link GeoEncodingUtils}.
 *
 * <p>Emits a single CodecUtil-framed {@value #FILE_NAME} containing
 * {@value #POINT_COUNT} (lat,lon) tuples encoded through
 * {@link GeoEncodingUtils#encodeLatitude(double)} and
 * {@link GeoEncodingUtils#encodeLongitude(double)}.
 *
 * <p>Wire layout (after CodecUtil IndexHeader):
 * <pre>
 *   vInt count
 *   for each point:
 *     int32 encodedLat   (Little-Endian, IndexOutput.writeInt convention since Lucene 10)
 *     int32 encodedLon   (Little-Endian)
 *   CodecUtil footer
 * </pre>
 *
 * <p>Deterministic seed mixing keeps the tuples inside the legal
 * latitude/longitude bounds.
 */
public final class GeoEncodedPointsScenario implements CorpusScenario {

    public static final String NAME = "geo-encoded-points";
    public static final String CODEC = "GoceneGeoEncodedPoints";
    public static final int VERSION = 0;
    public static final String FILE_NAME = "geo-encoded-points.bin";

    /** Number of (lat,lon) tuples emitted by the scenario. */
    public static final int POINT_COUNT = 32;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "GeoEncodingUtils encodeLatitude/encodeLongitude pairs framed by CodecUtil.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        int[][] pts = buildPoints(seed);
        try (FSDirectory dir = FSDirectory.open(target);
             IndexOutput out = dir.createOutput(FILE_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, Determinism.idBytes(seed), "");
            out.writeVInt(pts.length);
            for (int[] p : pts) {
                out.writeInt(p[0]);
                out.writeInt(p[1]);
            }
            CodecUtil.writeFooter(out);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        int[][] expected = buildPoints(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             ChecksumIndexInput in = dir.openChecksumInput(FILE_NAME)) {
            CodecUtil.checkIndexHeader(in, CODEC, VERSION, VERSION, Determinism.idBytes(seed), "");
            int n = in.readVInt();
            if (n != expected.length) {
                throw new IOException(NAME + ": count mismatch, expected " + expected.length
                        + ", got " + n);
            }
            for (int i = 0; i < n; i++) {
                int lat = in.readInt();
                int lon = in.readInt();
                if (lat != expected[i][0] || lon != expected[i][1]) {
                    throw new IOException(NAME + ": tuple[" + i + "] mismatch, expected ("
                            + expected[i][0] + "," + expected[i][1]
                            + "), got (" + lat + "," + lon + ")");
                }
            }
            CodecUtil.checkFooter(in);
        }
    }

    /** Build the deterministic encoded-points catalogue for {@code seed}. */
    public static int[][] buildPoints(long seed) {
        int[][] out = new int[POINT_COUNT][2];
        for (int i = 0; i < POINT_COUNT; i++) {
            double lat = seedToLat(seed, i);
            double lon = seedToLon(seed, i);
            out[i][0] = GeoEncodingUtils.encodeLatitude(lat);
            out[i][1] = GeoEncodingUtils.encodeLongitude(lon);
        }
        return out;
    }

    /** Maps a seed/index into a latitude in [-85, 85]. */
    public static double seedToLat(long seed, int i) {
        long u = mix(seed ^ ((long) i * 0xD1B54A32D192ED03L)) & 0xFFFFFFFFL;
        double frac = u / (double) 0x100000000L;
        return -85.0 + frac * 170.0;
    }

    /** Maps a seed/index into a longitude in [-179, 179]. */
    public static double seedToLon(long seed, int i) {
        long u = mix(seed ^ ((long) i * 0xAAAAAAAA55555555L)) & 0xFFFFFFFFL;
        double frac = u / (double) 0x100000000L;
        return -179.0 + frac * 358.0;
    }

    private static long mix(long z) {
        z = (z ^ (z >>> 30)) * 0xBF58476D1CE4E5B9L;
        z = (z ^ (z >>> 27)) * 0x94D049BB133111EBL;
        return z ^ (z >>> 31);
    }
}
