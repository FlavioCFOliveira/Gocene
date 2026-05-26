package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.search.similarities.BM25Similarity;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CombinedMultiSegmentIndexSearchScenario.NUM_DOCS;

/**
 * Sprint 114 T5 (rmp 4611), S2 {@code combined-reverse-index-search}.
 * Single-segment reference over the SAME deterministic doc set as S1; runs
 * the same query catalogue under BM25 and emits {@value #TSV_NAME}. Hit
 * rows are byte-identical to s1-hits.tsv (modulo the header comment)
 * because Lucene's BM25 accumulates CollectionStatistics across the
 * MultiReader. Doc / query / evaluate logic delegated to
 * {@link CombinedMultiSegmentIndexSearchScenario} to prevent drift.
 */
public final class CombinedReverseIndexSearchScenario implements CorpusScenario {

    public static final String NAME = "combined-reverse-index-search";
    public static final String TSV_NAME = "s2-hits.tsv";

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Single-segment Lucene reference for the same doc set; emits s2-hits.tsv.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setSimilarity(new BM25Similarity())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    writer.addDocument(CombinedMultiSegmentIndexSearchScenario.buildDoc(i, seed));
                }
                // Single commit => single segment.
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                if (reader.leaves().size() != 1) {
                    throw new IOException(NAME + ": expected exactly 1 segment, got "
                            + reader.leaves().size());
                }
                CombinedMultiSegmentIndexSearchScenario.writeTsv(
                        target.resolve(TSV_NAME),
                        CombinedMultiSegmentIndexSearchScenario.evaluate(reader));
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tsv = source.resolve(TSV_NAME);
        if (!Files.isRegularFile(tsv)) {
            throw new IOException(NAME + ": missing " + TSV_NAME);
        }
        List<CombinedMultiSegmentIndexSearchScenario.Row> recorded =
                CombinedMultiSegmentIndexSearchScenario.readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<CombinedMultiSegmentIndexSearchScenario.Row> recomputed =
                    CombinedMultiSegmentIndexSearchScenario.evaluate(reader);
            CombinedMultiSegmentIndexSearchScenario.assertEqualRows(recorded, recomputed);
        }
    }
}
