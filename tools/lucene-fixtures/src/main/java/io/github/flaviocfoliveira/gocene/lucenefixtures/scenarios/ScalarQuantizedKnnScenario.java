package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.codecs.Codec;
import org.apache.lucene.codecs.KnnVectorsFormat;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.codecs.lucene104.Lucene104HnswScalarQuantizedVectorsFormat;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.KnnFloatVectorField;

/**
 * Scalar-quantized HNSW vectors
 * ({@code Lucene104HnswScalarQuantizedVectorsFormat}): {@code .veq/.vex/.vemf}
 * plus the flat {@code .vec}.
 *
 * <p>This is the only scenario that exercises the {@code .veq}
 * (scalar-quantized vector data) wire format. The audit row notes
 * "No writer, no cross-engine fixture, no isolated test"; this scenario fills
 * the fixture gap.
 *
 * <p>Single-threaded determinism is inherited from
 * {@link IndexCorpusScenario}: {@code NoMergePolicy} +
 * {@code SerialMergeScheduler}. HNSW graph order under those conditions is
 * deterministic across runs at the same seed.
 */
public final class ScalarQuantizedKnnScenario extends IndexCorpusScenario {

    @Override
    public String name() {
        return "scalar-quantized-knn";
    }

    @Override
    public String description() {
        return "Scalar-quantized HNSW vectors (Lucene104HnswScalarQuantizedVectorsFormat): .veq/.vex/.vemf";
    }

    @Override
    protected int numDocs() {
        // Enough vectors to populate level-0 of the HNSW graph and ship the
        // quantization side-tables; small enough for deterministic, fast runs.
        return 16;
    }

    @Override
    protected Codec codec() {
        // Default constructor selects ScalarEncoding.SEVEN_BIT and the HNSW
        // index defaults (M=16, beamWidth=100), all of which are
        // single-thread-deterministic.
        return new Lucene104Codec() {
            private final KnnVectorsFormat sq = new Lucene104HnswScalarQuantizedVectorsFormat();

            @Override
            public KnnVectorsFormat getKnnVectorsFormatForField(String field) {
                return sq;
            }
        };
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        float[] f = new float[8];
        for (int k = 0; k < f.length; k++) {
            // Deterministic, well-spread float values in [0.01, 1.0]; avoids
            // zero vectors which trip cosine similarity in the quantizer.
            long mix = (seed * 0x9E3779B97F4A7C15L) ^ (((long) i << 16) | k);
            f[k] = (float) (((mix & 0xFFFF) / 65535.0) + 0.01);
        }
        doc.add(new KnnFloatVectorField("vec_q", f));
        return doc;
    }
}
