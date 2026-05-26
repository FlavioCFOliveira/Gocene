package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.search.suggest.InputIterator;
import org.apache.lucene.search.suggest.Lookup.LookupResult;
import org.apache.lucene.search.suggest.analyzing.AnalyzingSuggester;
import org.apache.lucene.store.ByteBuffersDirectory;
import org.apache.lucene.store.InputStreamDataInput;
import org.apache.lucene.store.OutputStreamDataOutput;
import org.apache.lucene.util.BytesRef;

import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Set;

/**
 * Sprint 114 T13 (rmp 4621): {@code completion-fst}. Addresses suggest audit
 * row (verbatim from docs/compat-coverage.tsv): "No round-trip against
 * Lucene-compiled completion FST." Builds an {@link AnalyzingSuggester} from
 * {@value #ENTRY_COUNT} seeded (input, weight) pairs, then persists the
 * suggester via its {@link AnalyzingSuggester#store(org.apache.lucene.store.DataOutput)
 * store()} method into the single file {@value #FILE_NAME}. Verify reloads
 * the suggester via {@code load()} and asserts every input still resolves.
 */
public final class CompletionFstScenario implements CorpusScenario {

    public static final String NAME = "completion-fst";
    public static final String FILE_NAME = "completion.fst";
    public static final int ENTRY_COUNT = 10;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "AnalyzingSuggester FST blob: seeded input/weight set persisted via store().";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        AnalyzingSuggester suggester = buildSuggester(seed);
        suggester.build(new SeededIterator(seededEntries(seed)));
        try (var fos = Files.newOutputStream(target.resolve(FILE_NAME));
             var bos = new BufferedOutputStream(fos);
             var out = new OutputStreamDataOutput(bos)) {
            if (!suggester.store(out)) {
                throw new IOException(NAME + ": store() returned false (empty FST?)");
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path file = source.resolve(FILE_NAME);
        if (!Files.isRegularFile(file)) {
            throw new IOException(NAME + ": missing " + file);
        }
        AnalyzingSuggester suggester = buildSuggester(seed);
        try (var fis = Files.newInputStream(file);
             var bis = new BufferedInputStream(fis);
             var in = new InputStreamDataInput(bis)) {
            if (!suggester.load(in)) {
                throw new IOException(NAME + ": load() returned false");
            }
        }
        long want = seededEntries(seed).size();
        if (suggester.getCount() != want) {
            throw new IOException(NAME + ": count mismatch, got " + suggester.getCount()
                    + " want " + want);
        }
        for (Entry e : seededEntries(seed)) {
            List<LookupResult> hits = suggester.lookup(e.surface, false, ENTRY_COUNT);
            boolean found = false;
            for (LookupResult r : hits) {
                if (r.key.toString().equals(e.surface)) {
                    found = true;
                    break;
                }
            }
            if (!found) {
                throw new IOException(NAME + ": surface '" + e.surface + "' not in hits");
            }
        }
    }

    /** Reusable temp Directory + analyzer; AnalyzingSuggester needs both. */
    private static AnalyzingSuggester buildSuggester(long seed) {
        Analyzer a = new StandardAnalyzer();
        // ByteBuffersDirectory keeps sort spill in-memory => no extra files on
        // disk that would pollute the manifest digest.
        return new AnalyzingSuggester(new ByteBuffersDirectory(), NAME + "-" + seed, a);
    }

    /** Deterministic {@code (surface, weight)} entries. */
    public static List<Entry> seededEntries(long seed) {
        String tag = String.format("%08x", seed & 0xFFFFFFFFL);
        List<Entry> out = new ArrayList<>(ENTRY_COUNT);
        for (int i = 0; i < ENTRY_COUNT; i++) {
            // Mix the seed with i to produce a stable but seed-distinct surface.
            long mix = (seed * 0x9E3779B97F4A7C15L) ^ ((long) i * 0xBF58476D1CE4E5B9L);
            String surface = "term" + i + "-" + tag;
            // Strictly positive weight (AnalyzingSuggester encodes weights as ints).
            int weight = 1 + (int) ((mix >>> 1) & 0x3FFF);
            out.add(new Entry(surface, weight));
        }
        return out;
    }

    public record Entry(String surface, int weight) {}

    /** Minimal {@link InputIterator} that walks a seeded {@link Entry} list. */
    public static final class SeededIterator implements InputIterator {
        private final Iterator<Entry> it;
        private Entry cur;
        public SeededIterator(List<Entry> entries) { this.it = entries.iterator(); }
        @Override public BytesRef next() {
            if (!it.hasNext()) return null;
            cur = it.next();
            return new BytesRef(cur.surface);
        }
        @Override public long weight() { return cur.weight; }
        @Override public BytesRef payload() { return null; }
        @Override public boolean hasPayloads() { return false; }
        @Override public Set<BytesRef> contexts() { return null; }
        @Override public boolean hasContexts() { return false; }
    }
}
