package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.spatial.composite.CompositeSpatialStrategy;
import org.apache.lucene.spatial.prefix.RecursivePrefixTreeStrategy;
import org.apache.lucene.spatial.prefix.tree.GeohashPrefixTree;
import org.apache.lucene.spatial.serialized.SerializedDVStrategy;
import org.locationtech.spatial4j.context.SpatialContext;
import org.locationtech.spatial4j.shape.Shape;

/**
 * Sprint 114 T20 (rmp 4628): {@code spatial-composite}.
 *
 * <p>Audit row covered (verbatim): "No tests for the composite strategy
 * port." for {@link CompositeSpatialStrategy}.
 *
 * <p>Indexes {@value #NUM_DOCS} documents through a
 * {@link CompositeSpatialStrategy} combining
 * {@link RecursivePrefixTreeStrategy} (over a
 * {@link GeohashPrefixTree}, {@value #MAX_LEVELS} levels) with a
 * {@link SerializedDVStrategy}. Both legs share the same logical field
 * name "geo"; the composite-internal suffixes ("__rpt", "__dv") are
 * appended by the contained strategies via their explicit field names.
 */
public final class SpatialCompositeScenario extends IndexCorpusScenario {

    public static final String NAME = "spatial-composite";
    public static final String FIELD = "geo";
    public static final int NUM_DOCS = 5;
    public static final int MAX_LEVELS = 6;

    private static final SpatialContext CTX = SpatialContext.GEO;

    private final RecursivePrefixTreeStrategy rpt =
            new RecursivePrefixTreeStrategy(new GeohashPrefixTree(CTX, MAX_LEVELS), FIELD + "__rpt");
    private final SerializedDVStrategy dv = new SerializedDVStrategy(CTX, FIELD + "__dv");
    private final CompositeSpatialStrategy strategy =
            new CompositeSpatialStrategy(FIELD, rpt, dv);

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "CompositeSpatialStrategy(RPT[geohash,levels=" + MAX_LEVELS + "] + SerializedDVStrategy).";
    }
    @Override protected int numDocs() { return NUM_DOCS; }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "cmp-" + i, Field.Store.YES));
        Shape s = SpatialSerializedDvShapeScenario.catalogueShape(i, seed);
        for (Field f : strategy.createIndexableFields(s)) {
            doc.add(f);
        }
        return doc;
    }
}
