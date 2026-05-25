package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.BinaryDocValuesField;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.NumericDocValuesField;
import org.apache.lucene.document.SortedDocValuesField;
import org.apache.lucene.document.SortedNumericDocValuesField;
import org.apache.lucene.document.SortedSetDocValuesField;
import org.apache.lucene.util.BytesRef;

/**
 * Doc values ({@code Lucene90DocValuesFormat}): {@code .dvd/.dvm}.
 *
 * <p>Indexes one document with every doc-values flavour so the produced
 * segment exercises NUMERIC, BINARY, SORTED, SORTED_NUMERIC and SORTED_SET.
 */
public final class DocValuesFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "doc-values-format";
    }

    @Override
    public String description() {
        return "Doc values (Lucene90DocValuesFormat): .dvd/.dvm";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        long base = seed + i;
        doc.add(new NumericDocValuesField("dv_num", base));
        doc.add(new BinaryDocValuesField("dv_bin",
                new BytesRef(("bin-" + base).getBytes(java.nio.charset.StandardCharsets.UTF_8))));
        doc.add(new SortedDocValuesField("dv_sorted",
                new BytesRef(("s-" + (base % 4)).getBytes(java.nio.charset.StandardCharsets.UTF_8))));
        doc.add(new SortedNumericDocValuesField("dv_sn", base));
        doc.add(new SortedNumericDocValuesField("dv_sn", base * 2));
        doc.add(new SortedSetDocValuesField("dv_ss",
                new BytesRef(("a-" + (base % 3)).getBytes(java.nio.charset.StandardCharsets.UTF_8))));
        doc.add(new SortedSetDocValuesField("dv_ss",
                new BytesRef(("b-" + (base % 5)).getBytes(java.nio.charset.StandardCharsets.UTF_8))));
        return doc;
    }
}
