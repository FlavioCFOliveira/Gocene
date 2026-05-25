package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.spatial.prefix.RecursivePrefixTreeStrategy;
import org.apache.lucene.spatial.prefix.tree.GeohashPrefixTree;
import org.apache.lucene.spatial.prefix.tree.SpatialPrefixTree;
import org.locationtech.spatial4j.context.SpatialContext;
import org.locationtech.spatial4j.shape.Shape;

/**
 * Sprint 114 T20 (rmp 4628): {@code spatial-prefix-tree}.
 *
 * <p>Audit row covered (verbatim): "No Lucene-emitted prefix-tree corpus."
 * for {@link RecursivePrefixTreeStrategy}.
 *
 * <p>Indexes {@value #NUM_DOCS} documents through
 * {@link RecursivePrefixTreeStrategy} backed by a
 * {@link GeohashPrefixTree} at {@value #MAX_LEVELS} levels. The strategy
 * emits one indexed term per cell along the geohash trie; the resulting
 * .tim/.tip/.doc files capture the prefix-tree cell tokens whose byte
 * layout this scenario freezes.
 */
public final class SpatialPrefixTreeScenario extends IndexCorpusScenario {

    public static final String NAME = "spatial-prefix-tree";
    public static final String FIELD = "geo";
    public static final int NUM_DOCS = 5;
    public static final int MAX_LEVELS = 6;

    private static final SpatialContext CTX = SpatialContext.GEO;

    private final SpatialPrefixTree grid = new GeohashPrefixTree(CTX, MAX_LEVELS);
    private final RecursivePrefixTreeStrategy strategy =
            new RecursivePrefixTreeStrategy(grid, FIELD);

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "RecursivePrefixTreeStrategy + GeohashPrefixTree(maxLevels=" + MAX_LEVELS
                + ") cell-token postings.";
    }
    @Override protected int numDocs() { return NUM_DOCS; }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "spt-" + i, Field.Store.YES));
        Shape s = SpatialSerializedDvShapeScenario.catalogueShape(i, seed);
        for (Field f : strategy.createIndexableFields(s)) {
            doc.add(f);
        }
        return doc;
    }
}
