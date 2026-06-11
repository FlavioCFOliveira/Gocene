package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.DoubleRangeDocValuesField;
import org.apache.lucene.document.LongRangeDocValuesField;

/**
 * Range doc-values ({@code Lucene90DocValuesFormat}): {@code .dvd/.dvm} with
 * double and long range doc-values fields.
 *
 * <p>Produces a 10-document fixture that exercises:
 * <ul>
 *   <li>{@link DoubleRangeDocValuesField} — 2-dimensional double range as binary DV</li>
 *   <li>{@link LongRangeDocValuesField} — 2-dimensional long range as binary DV</li>
 * </ul>
 *
 * <p>Registered as {@code "document-range-dv-format"} in {@link
 * io.github.flaviocfoliveira.gocene.lucenefixtures.Scenarios}.
 */
public final class DocumentRangeDocValuesScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "document-range-dv-format";
    }

    @Override
    public String description() {
        return "Document-level range doc-values: DoubleRangeDocValuesField, LongRangeDocValuesField";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();

        // 2-dimensional double range
        doc.add(new DoubleRangeDocValuesField("dbl_range",
            new double[]{1.0 + i, 10.0 + i * 2},
            new double[]{5.0 + i, 20.0 + i * 2}));

        // 2-dimensional long range
        doc.add(new LongRangeDocValuesField("long_range",
            new long[]{100L + i, 1000L + i * 10L},
            new long[]{500L + i * 2L, 5000L + i * 20L}));

        return doc;
    }
}
