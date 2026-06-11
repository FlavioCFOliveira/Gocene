package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.LatLonDocValuesField;
import org.apache.lucene.document.LatLonShape;
import org.apache.lucene.document.XYDocValuesField;
import org.apache.lucene.document.XYShape;
import org.apache.lucene.geo.Polygon;
import org.apache.lucene.geo.XYPolygon;

/**
 * Shape doc-values ({@code Lucene90DocValuesFormat}): {@code .dvd/.dvm} with
 * geographic and cartesian shape doc-values fields.
 *
 * <p>Produces a 10-document fixture that exercises:
 * <ul>
 *   <li>{@link LatLonDocValuesField} — single geographic point as sorted-numeric DV</li>
 *   <li>{@link XYDocValuesField} — single cartesian point as sorted-numeric DV</li>
 *   <li>{@link LatLonShape#createDocValueField(String, Polygon)} — tessellated
 *       geographic polygon as binary DV</li>
 *   <li>{@link XYShape#createDocValueField(String, XYPolygon)} — tessellated
 *       cartesian polygon as binary DV</li>
 * </ul>
 *
 * <p>Registered as {@code "document-shape-dv-format"} in {@link
 * io.github.flaviocfoliveira.gocene.lucenefixtures.Scenarios}.
 */
public final class DocumentShapeDocValuesScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "document-shape-dv-format";
    }

    @Override
    public String description() {
        return "Document-level shape doc-values: LatLonDocValuesField, XYDocValuesField, LatLonShape, XYShape";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();

        // Geographic point as sorted-numeric DV: (20+i, -100-i)
        double lat = 20.0 + i;
        double lon = -100.0 - i;
        doc.add(new LatLonDocValuesField("latlon_dv", lat, lon));

        // Cartesian point as sorted-numeric DV: (10+i, 20+i)
        float x = 10.0f + i;
        float y = 20.0f + i;
        doc.add(new XYDocValuesField("xy_dv", x, y));

        // Geographic shape (triangle) as binary DV
        Polygon triangle = new Polygon(
            new double[]{30.0, 40.0, 35.0},
            new double[]{-120.0, -110.0, -115.0}
        );
        doc.add(LatLonShape.createDocValueField("latlon_shape", triangle));

        // Cartesian shape (triangle) as binary DV
        XYPolygon xyTriangle = new XYPolygon(
            new float[]{10.0f, 20.0f, 15.0f},
            new float[]{10.0f, 10.0f, 20.0f}
        );
        doc.add(XYShape.createDocValueField("xy_shape", xyTriangle));

        return doc;
    }
}
