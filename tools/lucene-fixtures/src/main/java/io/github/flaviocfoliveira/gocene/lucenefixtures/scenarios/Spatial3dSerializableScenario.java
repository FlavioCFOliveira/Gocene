package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.spatial3d.geom.GeoBBox;
import org.apache.lucene.spatial3d.geom.GeoBBoxFactory;
import org.apache.lucene.spatial3d.geom.GeoCircle;
import org.apache.lucene.spatial3d.geom.GeoCircleFactory;
import org.apache.lucene.spatial3d.geom.GeoPoint;
import org.apache.lucene.spatial3d.geom.GeoPointShape;
import org.apache.lucene.spatial3d.geom.GeoPointShapeFactory;
import org.apache.lucene.spatial3d.geom.PlanetModel;
import org.apache.lucene.spatial3d.geom.SerializableObject;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexOutput;
import org.apache.lucene.store.OutputStreamIndexOutput;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T20 (rmp 4628): {@code spatial3d-serializable}.
 *
 * <p>Audit row covered (verbatim): "No cross-engine fixture for
 * spatial3d serialised geometry." for
 * {@code org.apache.lucene.spatial3d.geom.SerializableObject}.
 *
 * <p>Emits a single {@value #FILE_NAME} file framed by
 * {@link CodecUtil}. The payload, in order, contains:
 * <ol>
 *   <li>vInt {@code count} — number of geometries that follow.</li>
 *   <li>For each geometry: vInt {@code blobLen}, then {@code blobLen}
 *       bytes produced by {@link SerializableObject#writePlanetObject}
 *       over a fixed catalogue (GeoPointShape, GeoCircle, GeoBBox).
 *       Each blob therefore starts with the PlanetModel header and
 *       contains a single Geo3D object the Java side can round-trip via
 *       {@link SerializableObject#readPlanetObject}.</li>
 * </ol>
 *
 * <p>The catalogue is parameterised by the seed but bounded to keep
 * coordinates well inside the legal range for {@code PlanetModel.SPHERE}.
 * Determinism is enforced by {@link Determinism#seed(long)}.
 */
public final class Spatial3dSerializableScenario implements CorpusScenario {

    public static final String NAME = "spatial3d-serializable";
    public static final String CODEC = "GoceneSpatial3dSerializable";
    public static final int VERSION = 0;
    public static final String FILE_NAME = "spatial3d-serializable.bin";

    /** Number of geometries packed into the payload. */
    public static final int GEOMETRY_COUNT = 3;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Spatial3d SerializableObject.writePlanetObject blobs (GeoPointShape/GeoCircle/GeoBBox), "
                + "framed by CodecUtil.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        byte[][] blobs = buildBlobs(seed);
        try (FSDirectory dir = FSDirectory.open(target);
             IndexOutput out = dir.createOutput(FILE_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, Determinism.idBytes(seed), "");
            out.writeVInt(blobs.length);
            for (byte[] b : blobs) {
                out.writeVInt(b.length);
                out.writeBytes(b, 0, b.length);
            }
            CodecUtil.writeFooter(out);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        byte[][] expected = buildBlobs(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             org.apache.lucene.store.ChecksumIndexInput in = dir.openChecksumInput(FILE_NAME)) {
            CodecUtil.checkIndexHeader(in, CODEC, VERSION, VERSION, Determinism.idBytes(seed), "");
            int n = in.readVInt();
            if (n != expected.length) {
                throw new IOException(NAME + ": geometry count mismatch, expected "
                        + expected.length + ", got " + n);
            }
            for (int i = 0; i < n; i++) {
                int len = in.readVInt();
                byte[] got = new byte[len];
                in.readBytes(got, 0, len);
                if (got.length != expected[i].length
                        || !java.util.Arrays.equals(got, expected[i])) {
                    throw new IOException(NAME + ": blob[" + i + "] mismatch (expectedLen="
                            + expected[i].length + " gotLen=" + len + ")");
                }
            }
            CodecUtil.checkFooter(in);
        }
    }

    /**
     * Build the deterministic catalogue of writePlanetObject byte blobs for
     * {@code seed}. The PlanetModel is fixed to {@link PlanetModel#SPHERE}
     * so the resulting bytes are exactly byte-equivalent to what a Java
     * caller would produce.
     */
    public static byte[][] buildBlobs(long seed) throws IOException {
        PlanetModel pm = PlanetModel.SPHERE;
        // GeoPointShape: a single seeded point in radians, clipped well inside the legal range.
        double lat0 = clamp(seedToRadians(seed, 0xA1L), Math.PI / 2 - 0.01);
        double lon0 = clamp(seedToRadians(seed, 0xA2L), Math.PI - 0.01);
        GeoPointShape pt = GeoPointShapeFactory.makeGeoPointShape(pm, lat0, lon0);
        // GeoCircle: seeded centre + a small cutoff angle so the circle is well-formed.
        double latC = clamp(seedToRadians(seed, 0xB1L), Math.PI / 2 - 0.05);
        double lonC = clamp(seedToRadians(seed, 0xB2L), Math.PI - 0.05);
        double cutoffAngle = 0.1 + Math.abs(seedToRadians(seed, 0xB3L)) * 0.1;
        if (cutoffAngle <= 0.0) cutoffAngle = 0.1;
        GeoCircle circle = GeoCircleFactory.makeGeoCircle(pm, latC, lonC, cutoffAngle);
        // GeoBBox: seeded centre + symmetric half-extents kept well inside bounds.
        double latB = clamp(seedToRadians(seed, 0xC1L), Math.PI / 2 - 0.2);
        double lonB = clamp(seedToRadians(seed, 0xC2L), Math.PI - 0.2);
        double halfLat = 0.1;
        double halfLon = 0.15;
        GeoBBox bbox = GeoBBoxFactory.makeGeoBBox(pm,
                Math.min(latB + halfLat, Math.PI / 2 - 0.01),
                Math.max(latB - halfLat, -Math.PI / 2 + 0.01),
                lonB - halfLon, lonB + halfLon);

        // Use GeoPoint (which is itself a PlanetObject) only for header-stability checks
        // — keep it in the catalogue for variety.
        GeoPoint gp = new GeoPoint(pm, lat0, lon0);
        return new byte[][]{
                encode(pm, pt),
                encode(pm, circle),
                encode(pm, bbox),
                encode(pm, gp),
        };
    }

    /** Serialise {@code obj} via writePlanetObject into a heap byte array. */
    public static byte[] encode(PlanetModel pm, SerializableObject obj) throws IOException {
        ByteArrayOutputStream baos = new ByteArrayOutputStream(64);
        // Mirror writePlanetObject manually to keep the PlanetModel header
        // explicit (avoid the static helper's classloader interaction).
        pm.write((OutputStream) baos);
        SerializableObject.writeObject(baos, obj);
        return baos.toByteArray();
    }

    /** Wraps an int64-ish derivative of seed into a radian-scale double in [-1, 1]. */
    private static double seedToRadians(long seed, long salt) {
        long mixed = mix(seed ^ salt);
        // map low 32 bits of the mixed value into [-1, 1)
        long u = mixed & 0xFFFFFFFFL;
        double frac = u / (double) 0x100000000L;
        return (frac * 2.0) - 1.0;
    }

    private static double clamp(double v, double bound) {
        if (v > bound) return bound;
        if (v < -bound) return -bound;
        return v;
    }

    private static long mix(long z) {
        z = (z ^ (z >>> 30)) * 0xBF58476D1CE4E5B9L;
        z = (z ^ (z >>> 27)) * 0x94D049BB133111EBL;
        return z ^ (z >>> 31);
    }

    @Override public String toString() { return NAME; }

    /** Number of blobs the scenario writes (kept in sync with buildBlobs). */
    public static int blobCount() { return GEOMETRY_COUNT + 1 /* +GeoPoint */; }

    /** Unused helper retained for symmetry with sibling scenarios. */
    @SuppressWarnings("unused")
    private static OutputStreamIndexOutput unusedShim() { return null; }
}
