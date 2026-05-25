package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.FieldType;
import org.apache.lucene.index.IndexOptions;

/**
 * Term vectors ({@code Lucene90TermVectorsFormat}): {@code .tvd/.tvx/.tvm}.
 */
public final class TermVectorsFormatScenario extends IndexCorpusScenario {

    private static final FieldType TYPE;

    static {
        TYPE = new FieldType();
        TYPE.setIndexOptions(IndexOptions.DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS);
        TYPE.setTokenized(true);
        TYPE.setStoreTermVectors(true);
        TYPE.setStoreTermVectorPositions(true);
        TYPE.setStoreTermVectorOffsets(true);
        TYPE.setStoreTermVectorPayloads(false);
        TYPE.freeze();
    }

    @Override
    public String name() {
        return "term-vectors-format";
    }

    @Override
    public String description() {
        return "Term vectors (Lucene90TermVectorsFormat): .tvd/.tvx/.tvm";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String body = "alpha beta gamma " + (seed ^ i) + " delta epsilon zeta eta theta";
        doc.add(new Field("body", body, TYPE));
        return doc;
    }
}
