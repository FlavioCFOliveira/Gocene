package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.search.suggest.Lookup.LookupResult;
import org.apache.lucene.search.suggest.analyzing.AnalyzingInfixSuggester;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

/**
 * Sprint 114 T13 (rmp 4621): {@code analyzing-infix-sidecar}. Addresses
 * suggest audit row (verbatim): "No tests for this writer; data files never
 * validated." {@link AnalyzingInfixSuggester} persists its state into a
 * sidecar Lucene index (segments_N + per-segment files), not a single blob.
 * The scenario builds the suggester against an {@link FSDirectory} rooted at
 * {@value #SIDECAR_SUBDIR} under the harness target, then verify reopens the
 * sidecar and asserts every seeded input surfaces in lookup().
 */
public final class AnalyzingInfixSidecarScenario implements CorpusScenario {

    public static final String NAME = "analyzing-infix-sidecar";
    public static final String SIDECAR_SUBDIR = "infix";
    public static final int MIN_PREFIX_CHARS = 2;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "AnalyzingInfixSuggester sidecar: Lucene index persisted under " + SIDECAR_SUBDIR + "/.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        Path sidecar = target.resolve(SIDECAR_SUBDIR);
        Files.createDirectories(sidecar);
        Analyzer a = new StandardAnalyzer();
        try (FSDirectory dir = FSDirectory.open(sidecar);
             AnalyzingInfixSuggester suggester = new AnalyzingInfixSuggester(
                     dir, a, a, MIN_PREFIX_CHARS, true)) {
            suggester.build(new CompletionFstScenario.SeededIterator(CompletionFstScenario.seededEntries(seed)));
            suggester.commit();
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path sidecar = source.resolve(SIDECAR_SUBDIR);
        if (!Files.isDirectory(sidecar)) {
            throw new IOException(NAME + ": missing sidecar dir " + sidecar);
        }
        Analyzer a = new StandardAnalyzer();
        try (FSDirectory dir = FSDirectory.open(sidecar);
             AnalyzingInfixSuggester suggester = new AnalyzingInfixSuggester(
                     dir, a, a, MIN_PREFIX_CHARS, true)) {
            long want = CompletionFstScenario.seededEntries(seed).size();
            if (suggester.getCount() != want) {
                throw new IOException(NAME + ": count mismatch, got " + suggester.getCount()
                        + " want " + want);
            }
            for (CompletionFstScenario.Entry e : CompletionFstScenario.seededEntries(seed)) {
                // The first MIN_PREFIX_CHARS of the seeded surface form ("term") are
                // identical across all entries, so lookup with a prefix that includes
                // the index suffix to disambiguate.
                String prefix = e.surface().substring(0,
                        Math.min(MIN_PREFIX_CHARS + 2, e.surface().length()));
                List<LookupResult> hits = suggester.lookup(prefix, (java.util.Set<org.apache.lucene.util.BytesRef>) null, 100, true, true);
                boolean found = false;
                for (LookupResult r : hits) {
                    if (r.key.toString().equalsIgnoreCase(e.surface())) {
                        found = true;
                        break;
                    }
                }
                if (!found) {
                    throw new IOException(NAME + ": surface '" + e.surface()
                            + "' missing from lookup hits for prefix '" + prefix + "'");
                }
            }
        }
    }

}
