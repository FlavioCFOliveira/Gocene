package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.StoredField;

/**
 * Stored fields ({@code Lucene90StoredFieldsFormat}): {@code .fdt/.fdx/.fdm}.
 */
public final class StoredFieldsFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "stored-fields-format";
    }

    @Override
    public String description() {
        return "Stored fields (Lucene90StoredFieldsFormat): .fdt/.fdx/.fdm";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StoredField("title", "title-" + i + "-" + (seed & 0xFFFF)));
        doc.add(new StoredField("payload",
                ("payload-" + i + "-" + seed).getBytes(java.nio.charset.StandardCharsets.UTF_8)));
        doc.add(new StoredField("count", (long) (seed ^ i)));
        return doc;
    }
}
