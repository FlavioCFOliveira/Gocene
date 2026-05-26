package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.document.Document;
import org.apache.lucene.document.KnnByteVectorField;
import org.apache.lucene.document.KnnFloatVectorField;

/**
 * KNN vectors ({@code Lucene99HnswVectorsFormat}): {@code .vec/.vex/.vem}.
 *
 * <p>The HNSW graph construction reads from a {@link java.util.SplittableRandom}
 * seeded from {@code tests.seed} via {@code BaseKnnVectorsFormat}'s internal
 * indexing setup, which is sufficient for byte-determinism under the
 * single-threaded {@code SerialMergeScheduler} the base class installs. If
 * upstream changes that, see {@code CHANGES_FOR_PARENT.md}.
 */
public final class KnnVectorsFormatScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "knn-vectors-format";
    }

    @Override
    public String description() {
        return "KNN vectors (Lucene99HnswVectorsFormat): .vec/.vex/.vem";
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        float[] f = new float[4];
        byte[] b = new byte[4];
        for (int k = 0; k < 4; k++) {
            // Deterministic, well-spread values; avoid zero vectors which trip cosine.
            long mix = (seed * 0x9E3779B97F4A7C15L) ^ (((long) i << 16) | k);
            f[k] = (float) (((mix & 0xFFFF) / 65535.0) + 0.01);
            b[k] = (byte) (mix & 0x7F);
        }
        doc.add(new KnnFloatVectorField("vec_f", f));
        doc.add(new KnnByteVectorField("vec_b", b));
        return doc;
    }
}
