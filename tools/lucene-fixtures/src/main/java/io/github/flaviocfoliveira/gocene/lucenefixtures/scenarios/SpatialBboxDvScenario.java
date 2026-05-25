package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.FieldType;
import org.apache.lucene.document.StringField;
import org.apache.lucene.index.DocValuesType;
import org.apache.lucene.spatial.bbox.BBoxStrategy;
import org.locationtech.spatial4j.context.SpatialContext;
import org.locationtech.spatial4j.shape.Rectangle;
import org.locationtech.spatial4j.shape.Shape;

/**
 * Sprint 114 T20 (rmp 4628): {@code spatial-bbox-dv}.
 *
 * <p>Audit row covered (verbatim): "No fixture from Lucene to verify
 * byte exactness." for {@link BBoxStrategy} doc-values encoding.
 *
 * <p>Drives a {@link BBoxStrategy} configured with a {@link FieldType}
 * that enables ONLY numeric doc values for the four corner coordinates
 * (minX/maxX/minY/maxY). PointValues + Stored are disabled so the
 * resulting segment files exercise the doc-values byte path in
 * isolation. The XDL ("crosses dateline") side-channel is also
 * suppressed because it is gated on PointValues.
 */
public final class SpatialBboxDvScenario extends IndexCorpusScenario {

    public static final String NAME = "spatial-bbox-dv";
    public static final String FIELD = "bbox";
    public static final int NUM_DOCS = 5;

    private static final SpatialContext CTX = SpatialContext.GEO;

    private static final FieldType DV_ONLY_FIELDTYPE;
    static {
        FieldType t = new FieldType();
        t.setStored(false);
        t.setDocValuesType(DocValuesType.NUMERIC);
        t.freeze();
        DV_ONLY_FIELDTYPE = t;
    }

    private final BBoxStrategy strategy =
            new BBoxStrategy(CTX, FIELD, DV_ONLY_FIELDTYPE);

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "BBoxStrategy with NUMERIC DV only (4 corner coords).";
    }
    @Override protected int numDocs() { return NUM_DOCS; }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "bbox-" + i, Field.Store.YES));
        // Build a deterministic rectangle from the catalogue. If the
        // catalogue would return a Point, expand it into a tiny envelope.
        Shape s = SpatialSerializedDvShapeScenario.catalogueShape(i, seed);
        Rectangle bbox;
        if (s instanceof Rectangle r) {
            bbox = r;
        } else {
            // Expand the point into a 0.5-degree envelope clamped inside
            // the geo bounds; deterministic in i and seed.
            double cx = ((org.locationtech.spatial4j.shape.Point) s).getX();
            double cy = ((org.locationtech.spatial4j.shape.Point) s).getY();
            double minX = Math.max(-179.5, cx - 0.25);
            double maxX = Math.min(179.5, cx + 0.25);
            double minY = Math.max(-89.0, cy - 0.25);
            double maxY = Math.min(89.0, cy + 0.25);
            bbox = CTX.getShapeFactory().rect(minX, maxX, minY, maxY);
        }
        for (Field f : strategy.createIndexableFields(bbox)) {
            doc.add(f);
        }
        return doc;
    }
}
