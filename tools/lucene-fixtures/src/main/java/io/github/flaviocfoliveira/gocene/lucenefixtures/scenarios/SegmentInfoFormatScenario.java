package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;

/**
 * Segment infos ({@code Lucene99SegmentInfoFormat}): {@code .si} plus
 * {@code segments_N}.
 *
 * <p>Minimal indexing so the only files of interest are the segment metadata
 * and the commit pointer.
 */
public final class SegmentInfoFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "segment-info-format";
    }

    @Override
    public String description() {
        return "Segment infos (Lucene99SegmentInfoFormat): .si + segments_N";
    }

    @Override
    protected int numDocs() {
        return 3;
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "id-" + i + "-" + seed, Field.Store.NO));
        return doc;
    }
}
