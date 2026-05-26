package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.FloatPoint;
import org.apache.lucene.document.IntPoint;
import org.apache.lucene.document.LongPoint;

/**
 * Points / BKD ({@code Lucene90PointsFormat}): {@code .kdd/.kdi/.kdm}.
 */
public final class PointsFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "points-format";
    }

    @Override
    public String description() {
        return "Points / BKD (Lucene90PointsFormat): .kdd/.kdi/.kdm";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new IntPoint("ip", i, i + (int) seed));
        doc.add(new LongPoint("lp", (long) i * 13L + seed));
        doc.add(new FloatPoint("fp", (float) (i * 0.5), (float) (seed & 0xFFFF) * 0.25f));
        return doc;
    }
}
