package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.locationtech.spatial4j.context.SpatialContext;
import org.locationtech.spatial4j.context.SpatialContextFactory;
import org.locationtech.spatial4j.io.GeoJSONWriter;
import org.locationtech.spatial4j.io.WKTWriter;
import org.locationtech.spatial4j.shape.Shape;

import java.io.IOException;
import java.io.Reader;
import java.io.StringWriter;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

/**
 * Sprint 114 T20 (rmp 4628): {@code spatial-wkt-geojson}.
 *
 * <p>Audit row covered (verbatim): "Lacks parity tests against Lucene I/O."
 * for Shape WKT/GeoJSON I/O.
 *
 * <p>Emits two parallel TSV corpora derived from the seed:
 * <ul>
 *   <li>{@value #WKT_FILE} — one row per shape: {@code id\twkt}</li>
 *   <li>{@value #GEOJSON_FILE} — one row per shape: {@code id\tgeojson}</li>
 * </ul>
 * <p>The WKT and GeoJSON serialisations are produced by Spatial4j's
 * {@link WKTWriter} / {@link GeoJSONWriter}, exactly the same writers
 * exposed by Lucene's {@code SpatialContext.getFormats()}. Both files
 * use Unix line endings and UTF-8 encoding so the resulting bytes are
 * stable across platforms.
 */
public final class SpatialWktGeojsonScenario implements CorpusScenario {

    public static final String NAME = "spatial-wkt-geojson";
    public static final int NUM_SHAPES = 8;
    public static final String WKT_FILE = "shapes.wkt.tsv";
    public static final String GEOJSON_FILE = "shapes.geojson.tsv";

    private static final SpatialContext CTX = SpatialContext.GEO;

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "WKT + GeoJSON TSVs for a seeded shape corpus (Spatial4j writers).";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        List<Shape> shapes = buildShapes(seed);
        WKTWriter wkt = new WKTWriter();
        GeoJSONWriter geo = new GeoJSONWriter(CTX, new SpatialContextFactory());
        StringBuilder wktOut = new StringBuilder(NUM_SHAPES * 64);
        StringBuilder geoOut = new StringBuilder(NUM_SHAPES * 128);
        for (int i = 0; i < shapes.size(); i++) {
            Shape s = shapes.get(i);
            wktOut.append("wkt-").append(i).append('\t').append(toWkt(wkt, s)).append('\n');
            geoOut.append("geo-").append(i).append('\t').append(toGeoJson(geo, s)).append('\n');
        }
        Files.writeString(target.resolve(WKT_FILE), wktOut.toString(), StandardCharsets.UTF_8);
        Files.writeString(target.resolve(GEOJSON_FILE), geoOut.toString(), StandardCharsets.UTF_8);
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        List<Shape> shapes = buildShapes(seed);
        WKTWriter wkt = new WKTWriter();
        GeoJSONWriter geo = new GeoJSONWriter(CTX, new SpatialContextFactory());
        String wktExpected;
        String geoExpected;
        {
            StringBuilder wktOut = new StringBuilder();
            StringBuilder geoOut = new StringBuilder();
            for (int i = 0; i < shapes.size(); i++) {
                Shape s = shapes.get(i);
                wktOut.append("wkt-").append(i).append('\t').append(toWkt(wkt, s)).append('\n');
                geoOut.append("geo-").append(i).append('\t').append(toGeoJson(geo, s)).append('\n');
            }
            wktExpected = wktOut.toString();
            geoExpected = geoOut.toString();
        }
        String wktActual = Files.readString(source.resolve(WKT_FILE), StandardCharsets.UTF_8);
        String geoActual = Files.readString(source.resolve(GEOJSON_FILE), StandardCharsets.UTF_8);
        if (!wktExpected.equals(wktActual)) {
            throw new IOException(NAME + ": " + WKT_FILE + " mismatch");
        }
        if (!geoExpected.equals(geoActual)) {
            throw new IOException(NAME + ": " + GEOJSON_FILE + " mismatch");
        }
    }

    /** Deterministic shape catalogue. Reuses the same shape builder as the
     *  SerializedDV scenario so all spatial scenarios index a coherent set. */
    public static List<Shape> buildShapes(long seed) {
        List<Shape> out = new ArrayList<>(NUM_SHAPES);
        for (int i = 0; i < NUM_SHAPES; i++) {
            out.add(SpatialSerializedDvShapeScenario.catalogueShape(i, seed));
        }
        return out;
    }

    private static String toWkt(WKTWriter w, Shape s) {
        StringWriter sw = new StringWriter(64);
        try {
            w.write(sw, s);
        } catch (IOException e) {
            // StringWriter does not throw; rethrow defensively.
            throw new IllegalStateException(e);
        }
        return sw.toString();
    }

    private static String toGeoJson(GeoJSONWriter w, Shape s) {
        StringWriter sw = new StringWriter(128);
        try {
            w.write(sw, s);
        } catch (IOException e) {
            throw new IllegalStateException(e);
        }
        return sw.toString();
    }

    /** Convenience suppressor — silences unused-import warnings if Reader
     *  ever drops out of the implementation. */
    @SuppressWarnings("unused")
    private static String drain(Reader r) { return ""; }
}
