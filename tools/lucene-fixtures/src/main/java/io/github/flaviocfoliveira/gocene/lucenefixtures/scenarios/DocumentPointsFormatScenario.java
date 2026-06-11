package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.DoublePoint;
import org.apache.lucene.document.FloatPoint;
import org.apache.lucene.document.IntPoint;
import org.apache.lucene.document.LongPoint;

/**
 * Document-level points ({@code Lucene90PointsFormat}): {@code .kdd/.kdi/.kdm}
 * with all four numeric point types (IntPoint, LongPoint, FloatPoint, DoublePoint).
 *
 * <p>Extends the foundational {@code points-format} scenario by adding
 * {@link DoublePoint} coverage so the document binary-compat test can verify
 * all four numeric point types produced by Lucene 10.4.0.
 *
 * <p>Registered as {@code "document-points-format"} in {@link
 * io.github.flaviocfoliveira.gocene.lucenefixtures.Scenarios}.
 */
public final class DocumentPointsFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "document-points-format";
    }

    @Override
    public String description() {
        return "Document-level point fields (Lucene90PointsFormat): IntPoint, LongPoint, FloatPoint, DoublePoint";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        // 2-dimensional IntPoint: (i, i+(int)seed)
        doc.add(new IntPoint("ip", i, i + (int) seed));
        // 1-dimensional LongPoint: i*13 + seed
        doc.add(new LongPoint("lp", (long) i * 13L + seed));
        // 2-dimensional FloatPoint: (i*0.5, (seed&0xFFFF)*0.25)
        doc.add(new FloatPoint("fp", (float) (i * 0.5), (float) (seed & 0xFFFF) * 0.25f));
        // 1-dimensional DoublePoint: i*Math.PI + seed
        doc.add(new DoublePoint("dp", (double) i * Math.PI + seed));
        return doc;
    }
}
