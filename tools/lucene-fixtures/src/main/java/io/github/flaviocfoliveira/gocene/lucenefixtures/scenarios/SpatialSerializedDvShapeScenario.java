package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.spatial.serialized.SerializedDVStrategy;
import org.locationtech.spatial4j.context.SpatialContext;
import org.locationtech.spatial4j.shape.Point;
import org.locationtech.spatial4j.shape.Rectangle;
import org.locationtech.spatial4j.shape.Shape;

import java.io.IOException;

/**
 * Sprint 114 T20 (rmp 4628): {@code spatial-serialized-dv-shape}.
 *
 * <p>Audit row covered (verbatim from docs/compat-coverage.tsv): "No
 * Lucene-produced shape blob is decoded by Gocene tests." for
 * {@link SerializedDVStrategy}.
 *
 * <p>Emits a single-segment Lucene 10.4 index containing
 * {@value #NUM_DOCS} documents. Each document carries a deterministic
 * seed-derived Spatial4j shape (a Point or a Rectangle, alternating)
 * serialised by {@link SerializedDVStrategy} into BinaryDocValues using
 * Spatial4j's {@code BinaryCodec.writeShape}.
 *
 * <p>The shape geometry is selected from a fixed catalogue so the
 * resulting BinaryDocValues blob is byte-deterministic across runs at the
 * same seed (the seed only controls which subset of the catalogue lands
 * in which doc).
 */
public final class SpatialSerializedDvShapeScenario extends IndexCorpusScenario {

    public static final String NAME = "spatial-serialized-dv-shape";
    public static final String FIELD = "shape";
    public static final int NUM_DOCS = 6;

    private static final SpatialContext CTX = SpatialContext.GEO;
    private final SerializedDVStrategy strategy = new SerializedDVStrategy(CTX, FIELD);

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "SerializedDVStrategy BinaryDocValues blob over Spatial4j shapes "
                + "(Point/Rectangle catalogue, deterministic).";
    }
    @Override protected int numDocs() { return NUM_DOCS; }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "sdv-" + i, Field.Store.YES));
        Shape s = catalogueShape(i, seed);
        for (Field f : strategy.createIndexableFields(s)) {
            doc.add(f);
        }
        return doc;
    }

    /**
     * Deterministic shape catalogue. Even-indexed docs receive a Point
     * derived from the seed; odd-indexed docs receive a Rectangle. All
     * coordinates stay strictly inside the geo bounds [-180,180] / [-90,90].
     */
    public static Shape catalogueShape(int i, long seed) {
        double lat = scaleLat((seed ^ (i * 0xD1B54A32D192ED03L)) >>> 32, i);
        double lon = scaleLon((seed ^ (i * 0xAAAAAAAA55555555L)) >>> 32, i);
        if ((i & 1) == 0) {
            return CTX.getShapeFactory().pointXY(lon, lat);
        }
        double halfW = 0.5 + ((i & 7) * 0.25);
        double halfH = 0.25 + ((i & 3) * 0.5);
        double minX = clampLon(lon - halfW);
        double maxX = clampLon(lon + halfW);
        double minY = clampLat(lat - halfH);
        double maxY = clampLat(lat + halfH);
        Rectangle r = CTX.getShapeFactory().rect(minX, maxX, minY, maxY);
        return r;
    }

    /** Map an unsigned 32-bit derivative of the seed into [-85, 85] (avoid the geo poles). */
    private static double scaleLat(long u, int i) {
        double frac = (u & 0xFFFFFFFFL) / (double) 0x100000000L;
        return -85.0 + frac * 170.0 + (i * 0.001);
    }

    /** Map an unsigned 32-bit derivative of the seed into [-179, 179]. */
    private static double scaleLon(long u, int i) {
        double frac = (u & 0xFFFFFFFFL) / (double) 0x100000000L;
        return -179.0 + frac * 358.0 + (i * 0.001);
    }

    private static double clampLat(double v) { return Math.max(-89.5, Math.min(89.5, v)); }
    private static double clampLon(double v) { return Math.max(-179.9, Math.min(179.9, v)); }

    /** Exposed for tests that want to compute Point geometry without the strategy. */
    public static Point seedPoint(long seed) {
        return CTX.getShapeFactory().pointXY(scaleLon(seed & 0xFFFFFFFFL, 0),
                scaleLat((seed >>> 32) & 0xFFFFFFFFL, 0));
    }

    /** Convenience: the field name written by this scenario. */
    public static String fieldName() { return FIELD; }

    /** No additional verification beyond the standard live-doc count. */
    @Override
    protected void verifyReader(org.apache.lucene.index.IndexReader reader, long seed) throws IOException {
        super.verifyReader(reader, seed);
    }
}
