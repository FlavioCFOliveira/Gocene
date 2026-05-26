package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.search.suggest.Lookup.LookupResult;
import org.apache.lucene.search.suggest.analyzing.AnalyzingSuggester;
import org.apache.lucene.store.ByteBuffersDirectory;
import org.apache.lucene.store.InputStreamDataInput;
import org.apache.lucene.store.OutputStreamDataOutput;

import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

/**
 * Sprint 114 T5 (rmp 4611), S5 {@code combined-suggester-fst}.
 * Reuses CompletionFstScenario.seededEntries to build an AnalyzingSuggester,
 * persists it via store(), then runs 5 prefix queries and emits
 * {@value #TSV_NAME} (prefix, rank, suggestion) sorted by (prefix asc, rank asc).
 */
public final class CombinedSuggesterFstScenario implements CorpusScenario {

    public static final String NAME = "combined-suggester-fst";
    public static final String FST_NAME = "s5-completion.fst";
    public static final String TSV_NAME = "s5-suggestions.tsv";
    /** Number of distinct prefixes queried. */
    public static final int PREFIX_COUNT = 5;
    /** TopN suggestions per prefix lookup. */
    public static final int TOPN = 3;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "AnalyzingSuggester FST + 5 prefix queries; emits s5-suggestions.tsv.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        AnalyzingSuggester suggester = buildSuggester(seed);
        suggester.build(new CompletionFstScenario.SeededIterator(
                CompletionFstScenario.seededEntries(seed)));
        try (var fos = Files.newOutputStream(target.resolve(FST_NAME));
             var bos = new BufferedOutputStream(fos);
             var out = new OutputStreamDataOutput(bos)) {
            if (!suggester.store(out)) {
                throw new IOException(NAME + ": store() returned false");
            }
        }
        List<Row> rows = lookupAll(suggester, seed);
        writeTsv(target.resolve(TSV_NAME), rows);
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path fst = source.resolve(FST_NAME);
        Path tsv = source.resolve(TSV_NAME);
        if (!Files.isRegularFile(fst) || !Files.isRegularFile(tsv)) {
            throw new IOException(NAME + ": missing " + FST_NAME + " or " + TSV_NAME);
        }
        AnalyzingSuggester suggester = buildSuggester(seed);
        try (var fis = Files.newInputStream(fst);
             var bis = new BufferedInputStream(fis);
             var in = new InputStreamDataInput(bis)) {
            if (!suggester.load(in)) {
                throw new IOException(NAME + ": load() returned false");
            }
        }
        List<Row> recorded = readTsv(tsv);
        List<Row> recomputed = lookupAll(suggester, seed);
        if (recorded.size() != recomputed.size()) {
            throw new IOException(NAME + ": row count drift recorded="
                    + recorded.size() + " recomputed=" + recomputed.size());
        }
        for (int i = 0; i < recorded.size(); i++) {
            if (!recorded.get(i).equals(recomputed.get(i))) {
                throw new IOException(NAME + ": row " + i + " drift: "
                        + recorded.get(i) + " vs " + recomputed.get(i));
            }
        }
    }

    private static AnalyzingSuggester buildSuggester(long seed) {
        Analyzer a = new StandardAnalyzer();
        return new AnalyzingSuggester(new ByteBuffersDirectory(), NAME + "-" + seed, a);
    }

    /** Five DISTINCT prefixes derived from the seeded entries. Each surface
     *  is shaped "termN-xxxxxxxx" so the first 5 chars (termN) are unique
     *  per entry; this guarantees PREFIX_COUNT distinct prefixes. */
    public static List<String> seededPrefixes(long seed) {
        List<CompletionFstScenario.Entry> entries = CompletionFstScenario.seededEntries(seed);
        List<String> out = new ArrayList<>(PREFIX_COUNT);
        for (int i = 0; i < PREFIX_COUNT && i < entries.size(); i++) {
            String surf = entries.get(i).surface();
            // Take the "termN" stem (5 chars: 't','e','r','m', digit).
            int plen = Math.min(5, surf.length());
            out.add(surf.substring(0, plen));
        }
        return out;
    }

    private static List<Row> lookupAll(AnalyzingSuggester suggester, long seed) throws IOException {
        List<Row> rows = new ArrayList<>();
        for (String prefix : seededPrefixes(seed)) {
            List<LookupResult> hits = suggester.lookup(prefix, false, TOPN);
            for (int rank = 0; rank < hits.size(); rank++) {
                rows.add(new Row(prefix, rank, hits.get(rank).key.toString()));
            }
        }
        rows.sort((a, b) -> {
            int c = a.prefix().compareTo(b.prefix());
            if (c != 0) return c;
            return Integer.compare(a.rank(), b.rank());
        });
        return rows;
    }

    private static void writeTsv(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# prefix\trank\tsuggestion\n");
        for (Row r : rows) {
            sb.append(r.prefix()).append('\t').append(r.rank()).append('\t')
                    .append(r.suggestion()).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<Row> readTsv(Path file) throws IOException {
        List<Row> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 3) throw new IOException("malformed row: " + line);
                rows.add(new Row(cols[0], Integer.parseInt(cols[1]), cols[2]));
            }
        }
        return rows;
    }

    /** Single TSV row. */
    public record Row(String prefix, int rank, String suggestion) {
        @Override public String toString() {
            return String.format(Locale.ROOT, "%s#%d=%s", prefix, rank, suggestion);
        }
    }
}
