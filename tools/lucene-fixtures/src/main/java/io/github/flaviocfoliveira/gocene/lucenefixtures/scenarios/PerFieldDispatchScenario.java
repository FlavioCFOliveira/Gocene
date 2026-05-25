package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.codecs.Codec;
import org.apache.lucene.codecs.DocValuesFormat;
import org.apache.lucene.codecs.PostingsFormat;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.codecs.lucene90.Lucene90DocValuesFormat;
import org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat;
import org.apache.lucene.codecs.memory.FSTPostingsFormat;
import org.apache.lucene.document.BinaryDocValuesField;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.NumericDocValuesField;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.util.BytesRef;

import java.nio.charset.StandardCharsets;

/**
 * Per-field dispatch ({@code PerFieldPostingsFormat} +
 * {@code PerFieldDocValuesFormat}): a single index writer run where two
 * different posting formats and two different doc-values formats are routed
 * by field. The resulting segment contains a {@code _0.per_field_*} suffixed
 * postings/dv pair for each format choice.
 *
 * <p>This scenario is the only path that exercises the per-field dispatch
 * metadata stamped into {@code .fnm} attributes and surfaces in the file
 * naming (e.g. {@code _0_Lucene104_0.tim} vs {@code _0_FSTPostingsFormat_0.tim}).
 *
 * <p>Field map:
 * <ul>
 *   <li>{@code title}    → Lucene104PostingsFormat (default)</li>
 *   <li>{@code body}     → FSTPostingsFormat       (alternate)</li>
 *   <li>{@code dv_num}   → Lucene90DocValuesFormat (default)</li>
 *   <li>{@code dv_bin}   → Lucene90DocValuesFormat (default, second field)</li>
 * </ul>
 */
public final class PerFieldDispatchScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "perfield-postings-doc-values";
    }

    @Override
    public String description() {
        return "Per-field PostingsFormat + DocValuesFormat dispatch: multi-suffix .tim/.dvd";
    }

    @Override
    protected int numDocs() {
        return 12;
    }

    @Override
    protected Codec codec() {
        // Anonymous Lucene104Codec subclass: route 'body' to FSTPostingsFormat,
        // everything else to the default. Doc-values stay on the default DV
        // format but the per-field DV writer still emits the dispatch metadata.
        return new Lucene104Codec() {
            private final PostingsFormat alt = new FSTPostingsFormat();
            private final PostingsFormat dflt = new Lucene104PostingsFormat();
            private final DocValuesFormat dv = new Lucene90DocValuesFormat();

            @Override
            public PostingsFormat getPostingsFormatForField(String field) {
                if ("body".equals(field)) {
                    return alt;
                }
                return dflt;
            }

            @Override
            public DocValuesFormat getDocValuesFormatForField(String field) {
                return dv;
            }
        };
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("title", "title-" + i, Field.Store.NO));
        doc.add(new TextField("body", "alpha beta " + (seed ^ i) + " gamma delta", Field.Store.NO));
        doc.add(new NumericDocValuesField("dv_num", seed + i));
        doc.add(new BinaryDocValuesField("dv_bin",
                new BytesRef(("bin-" + (seed + i)).getBytes(StandardCharsets.UTF_8))));
        return doc;
    }
}
