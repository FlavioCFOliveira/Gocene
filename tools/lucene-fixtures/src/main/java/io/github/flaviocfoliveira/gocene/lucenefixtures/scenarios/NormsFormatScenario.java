package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.TextField;

/**
 * Norms ({@code Lucene90NormsFormat}): {@code .nvd/.nvm}.
 *
 * <p>{@link TextField} indexes with norms enabled by default; the per-document
 * field length varies so the norms file actually carries information.
 */
public final class NormsFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "norms-format";
    }

    @Override
    public String description() {
        return "Norms (Lucene90NormsFormat): .nvd/.nvm";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        // Repeat a token (i + 1) times so each doc has a distinct field length,
        // forcing the norms encoder to emit varying values.
        StringBuilder sb = new StringBuilder();
        for (int k = 0; k <= i; k++) {
            if (k > 0) sb.append(' ');
            sb.append("word").append(seed ^ k);
        }
        doc.add(new TextField("body", sb.toString(), Field.Store.NO));
        return doc;
    }
}
