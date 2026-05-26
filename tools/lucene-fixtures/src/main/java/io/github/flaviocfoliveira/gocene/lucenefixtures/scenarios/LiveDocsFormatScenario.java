package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.Term;

import java.io.IOException;

/**
 * Live docs ({@code Lucene90LiveDocsFormat}): {@code .liv}.
 *
 * <p>Indexes 10 docs and deletes two of them so the segment carries a
 * deletion bitset that must be persisted.
 */
public final class LiveDocsFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "live-docs-format";
    }

    @Override
    public String description() {
        return "Live docs (Lucene90LiveDocsFormat): .liv";
    }

    @Override
    protected int numDocs() {
        return 10;
    }

    @Override
    protected int expectedLiveDocs(long seed) {
        return numDocs() - 2;
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "id-" + i, Field.Store.NO));
        doc.add(new TextField("body", "live-docs payload " + (seed ^ i), Field.Store.NO));
        return doc;
    }

    @Override
    protected void afterAdd(IndexWriter writer, long seed) throws IOException {
        writer.deleteDocuments(new Term("id", "id-3"));
        writer.deleteDocuments(new Term("id", "id-7"));
    }
}
