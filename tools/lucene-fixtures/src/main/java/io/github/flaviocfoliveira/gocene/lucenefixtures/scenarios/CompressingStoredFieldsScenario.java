package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.codecs.Codec;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.StoredField;

import java.nio.charset.StandardCharsets;

/**
 * Compressing stored-fields block format
 * ({@code Lucene90CompressingStoredFieldsFormat}): exercises the
 * {@code BEST_COMPRESSION} branch (DEFLATE) so the compression-mode bytes
 * differ from the default-mode stored-fields scenario.
 *
 * <p>Pairs with {@link StoredFieldsFormatScenario} (BEST_SPEED / LZ4) to give
 * the codecs audit row "Lucene90 compressing block format (LZ4/Deflate/
 * BEST_SPEED)" cross-engine coverage for both compression modes.
 */
public final class CompressingStoredFieldsScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "compressing-stored-fields";
    }

    @Override
    public String description() {
        return "Compressing stored fields (DEFLATE BEST_COMPRESSION): .fdt/.fdx/.fdm";
    }

    @Override
    protected int numDocs() {
        // Push past one block (Lucene's default block size is small but not
        // tiny). 64 docs with non-trivial bodies trip the DEFLATE path with a
        // reasonable chunk count.
        return 64;
    }

    @Override
    protected Codec codec() {
        return new Lucene104Codec(Lucene104Codec.Mode.BEST_COMPRESSION);
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StoredField("title", "title-" + i + "-" + (seed & 0xFFFF)));
        // Stable, compressible payload (repeats inside one field, varies
        // across docs so DEFLATE has actual work to do).
        StringBuilder sb = new StringBuilder(256);
        long mix = seed ^ ((long) i * 0x9E3779B97F4A7C15L);
        for (int k = 0; k < 32; k++) {
            sb.append("compress-").append(mix & 0xFF).append('-');
        }
        doc.add(new StoredField("body", sb.toString().getBytes(StandardCharsets.UTF_8)));
        doc.add(new StoredField("count", (long) (seed ^ i)));
        return doc;
    }
}
