package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;

/**
 * Compound files ({@code Lucene90CompoundFormat}): {@code .cfs/.cfe/.si}.
 *
 * <p>Same field set as {@link PostingsFormatScenario} but with the writer
 * configured to bundle the segment into a compound file.
 */
public final class CompoundFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "compound-format";
    }

    @Override
    public String description() {
        return "Compound files (Lucene90CompoundFormat): .cfs/.cfe/.si";
    }

    @Override
    protected boolean useCompoundFile() {
        return true;
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "id-" + i, Field.Store.NO));
        doc.add(new TextField("body", "alpha beta " + (seed ^ i) + " gamma delta", Field.Store.NO));
        return doc;
    }
}
