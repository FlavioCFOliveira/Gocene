package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;

/**
 * Postings ({@code Lucene104PostingsFormat}): {@code .doc/.pos/.pay/.tim/.tip/.tmd}.
 */
public final class PostingsFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "postings-format";
    }

    @Override
    public String description() {
        return "Postings (Lucene104PostingsFormat): .doc/.pos/.pay/.tim/.tip/.tmd";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "id-" + i, Field.Store.NO));
        // Stable phrase whose tokens cross the postings-flush boundaries.
        String body = "alpha beta gamma delta " + (seed ^ i) + " epsilon zeta";
        doc.add(new TextField("body", body, Field.Store.NO));
        return doc;
    }
}
