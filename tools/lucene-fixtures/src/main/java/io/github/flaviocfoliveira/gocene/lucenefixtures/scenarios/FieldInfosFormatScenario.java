package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.BinaryDocValuesField;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.IntPoint;
import org.apache.lucene.document.KnnFloatVectorField;
import org.apache.lucene.document.NumericDocValuesField;
import org.apache.lucene.document.SortedDocValuesField;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.util.BytesRef;

import java.nio.charset.StandardCharsets;

/**
 * FieldInfos ({@code Lucene94FieldInfosFormat}): {@code .fnm}.
 *
 * <p>Exercises a varied field zoo (≥10 distinct names) so the FieldInfos file
 * carries every flag bit we care about: indexed/text/string, stored,
 * doc-values (numeric/sorted/binary), points, knn vectors.
 */
public final class FieldInfosFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "field-infos-format";
    }

    @Override
    public String description() {
        return "FieldInfos (Lucene94FieldInfosFormat): .fnm";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("f_id", "id-" + i, Field.Store.YES));
        doc.add(new TextField("f_body", "alpha beta gamma " + (seed ^ i), Field.Store.NO));
        doc.add(new StoredField("f_stored_str", "stored-" + i));
        doc.add(new StoredField("f_stored_bin",
                ("bin-" + i).getBytes(StandardCharsets.UTF_8)));
        doc.add(new NumericDocValuesField("f_dv_num", seed + i));
        doc.add(new BinaryDocValuesField("f_dv_bin",
                new BytesRef(("bdv-" + i).getBytes(StandardCharsets.UTF_8))));
        doc.add(new SortedDocValuesField("f_dv_sorted",
                new BytesRef(("sdv-" + (i % 4)).getBytes(StandardCharsets.UTF_8))));
        doc.add(new IntPoint("f_point", i));
        doc.add(new KnnFloatVectorField("f_vec",
                new float[]{(float) i, (float) (i + 1), (float) (i + 2), (float) (i + 3)}));
        doc.add(new TextField("f_aux1", "aux1-" + i, Field.Store.NO));
        doc.add(new TextField("f_aux2", "aux2-" + i, Field.Store.NO));
        return doc;
    }
}
