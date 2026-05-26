package io.github.flaviocfoliveira.gocene.lucenefixtures;

import java.io.IOException;
import java.io.PrintStream;
import java.nio.file.Path;

/**
 * Entry point for the {@code lucene-fixtures} harness.
 *
 * <pre>
 *   java -jar lucene-fixtures.jar gen      &lt;scenario&gt; &lt;seed&gt; &lt;target&gt;
 *   java -jar lucene-fixtures.jar verify   &lt;scenario&gt; &lt;seed&gt; &lt;source&gt;
 *   java -jar lucene-fixtures.jar list
 *   java -jar lucene-fixtures.jar manifest [seed]
 *   java -jar lucene-fixtures.jar check    &lt;dir&gt;
 * </pre>
 *
 * <p>Exit codes:
 * <ul>
 *   <li>{@code 0}  success</li>
 *   <li>{@code 1}  argument / usage error</li>
 *   <li>{@code 2}  unknown scenario</li>
 *   <li>{@code 3}  IO error</li>
 *   <li>{@code 4}  verification failure (artefact does not match the scenario contract)</li>
 * </ul>
 */
public final class Main {

    /** Canary seed used by the manifest subcommand and by CI baseline checks. */
    public static final long CANARY_SEED = 0xC0FFEEL;

    private Main() {}

    public static void main(String[] args) {
        try {
            int code = run(args, System.out, System.err);
            System.exit(code);
        } catch (Throwable t) {
            t.printStackTrace(System.err);
            System.exit(3);
        }
    }

    /** Test-friendly entry point. */
    public static int run(String[] args, PrintStream out, PrintStream err) {
        if (args.length == 0) {
            usage(err);
            return 1;
        }
        String cmd = args[0];
        return switch (cmd) {
            case "gen" -> runGen(args, out, err);
            case "verify" -> runVerify(args, out, err);
            case "list" -> runList(out);
            case "manifest" -> runManifest(args, out, err);
            case "check" -> runCheck(args, out, err);
            case "verify-scoring" -> runVerifyScoring(args, out, err);
            case "verify-knn-hits" -> runVerifyKnnHits(args, out, err);
            case "verify-queries-hits" -> runVerifyQueriesHits(args, out, err);
            case "verify-highlight-offsets" -> runVerifyHighlightOffsets(args, out, err);
            case "verify-fvh-phrases" -> runVerifyFvhPhrases(args, out, err);
            case "verify-join-hits" -> runVerifyJoinHits(args, out, err);
            case "verify-grouping-results" -> runVerifyGroupingResults(args, out, err);
            case "verify-classifier-labels" -> runVerifyClassifierLabels(args, out, err);
            case "verify-monitor" -> runVerifyMonitor(args, out, err);
            case "verify-replicator" -> runVerifyReplicator(args, out, err);
            case "verify-expressions-eval" -> runVerifyExpressionsEval(args, out, err);
            case "verify-queryparser" -> runVerifyQueryparser(args, out, err);
            case "-h", "--help", "help" -> {
                usage(out);
                yield 0;
            }
            default -> {
                err.println("unknown command: " + cmd);
                usage(err);
                yield 1;
            }
        };
    }

    private static int runGen(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 4) {
            err.println("usage: gen <scenario> <seed> <target>");
            return 1;
        }
        String scenarioName = args[1];
        long seed = parseSeed(args[2], err);
        if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[2])) {
            return 1;
        }
        Determinism.seed(seed);
        Path target = Path.of(args[3]);
        CorpusScenario scenario;
        try {
            scenario = Scenarios.require(scenarioName);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        }
        try {
            scenario.generate(target, seed);
        } catch (IOException e) {
            err.println("gen failed: " + e.getMessage());
            return 3;
        }
        out.println("ok scenario=" + scenarioName + " seed=" + seed + " target=" + target.toAbsolutePath());
        return 0;
    }

    private static int runVerify(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 4) {
            err.println("usage: verify <scenario> <seed> <source>");
            return 1;
        }
        String scenarioName = args[1];
        long seed = parseSeed(args[2], err);
        if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[2])) {
            return 1;
        }
        Determinism.seed(seed);
        Path source = Path.of(args[3]);
        CorpusScenario scenario;
        try {
            scenario = Scenarios.require(scenarioName);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        }
        try {
            scenario.verify(source, seed);
        } catch (IOException e) {
            err.println("verify failed: " + e.getMessage());
            return 4;
        }
        out.println("ok scenario=" + scenarioName + " seed=" + seed + " source=" + source.toAbsolutePath());
        return 0;
    }

    private static int runList(PrintStream out) {
        // List emits scenario names on stdout, one per line (descriptions go to nowhere
        // so the output is consumable by the Makefile loop). Keep this format stable.
        Scenarios.all().keySet().forEach(out::println);
        return 0;
    }

    private static int runManifest(String[] args, PrintStream out, PrintStream err) {
        long seed = CANARY_SEED;
        if (args.length >= 2) {
            seed = parseSeed(args[1], err);
            if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[1])) {
                return 1;
            }
        }
        try {
            Manifest.print(seed, out);
        } catch (IOException e) {
            err.println("manifest failed: " + e.getMessage());
            return 3;
        }
        return 0;
    }

    /**
     * Runs Lucene's {@link org.apache.lucene.index.CheckIndex} over a
     * directory and reports {@code clean} on success.
     *
     * <p>Output format on success (stdout, single line):
     * <pre>ok check dir=&lt;absolute-path&gt; segments=&lt;n&gt; missingSegments=false</pre>
     *
     * <p>On failure the CheckIndex textual report is forwarded to stderr
     * and the exit code is 4 (matching the {@code verify} contract).
     */
    private static int runCheck(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: check <dir>");
            return 1;
        }
        Path source = Path.of(args[1]);
        try (org.apache.lucene.store.FSDirectory dir = org.apache.lucene.store.FSDirectory.open(source);
             java.io.ByteArrayOutputStream captured = new java.io.ByteArrayOutputStream();
             PrintStream sink = new PrintStream(captured, true, java.nio.charset.StandardCharsets.UTF_8);
             org.apache.lucene.index.CheckIndex checker = new org.apache.lucene.index.CheckIndex(dir)) {
            checker.setInfoStream(sink, false);
            org.apache.lucene.index.CheckIndex.Status status = checker.checkIndex();
            if (status.clean) {
                int segs = status.segmentInfos == null ? 0 : status.segmentInfos.size();
                out.println("ok check dir=" + source.toAbsolutePath()
                        + " segments=" + segs
                        + " missingSegments=" + status.missingSegments);
                return 0;
            }
            err.println(captured.toString(java.nio.charset.StandardCharsets.UTF_8));
            err.println("CheckIndex reported a non-clean index at " + source.toAbsolutePath());
            return 4;
        } catch (IOException e) {
            err.println("check failed: " + e.getMessage());
            return 3;
        }
    }

    /**
     * Runs the {@code search-scoring-corpus} verifier over an externally
     * supplied directory that MUST contain both the Lucene index segments
     * and a {@code scoring.tsv} written by the same scenario (or by a
     * Gocene port). On success a single {@code ok} line is emitted; on
     * mismatch the scenario raises an IOException and the harness exits
     * with code 4.
     */
    private static int runVerifyScoring(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-scoring <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("search-scoring-corpus");
            // Seed is irrelevant for verify (the TSV pins the expected
            // values); pass 0 so Determinism.seed() runs deterministically.
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-scoring failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-scoring dir=" + source.toAbsolutePath());
        return 0;
    }

    /** Verifies {@code knn-hits.tsv} mirrors {@link #runVerifyScoring}. */
    private static int runVerifyKnnHits(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-knn-hits <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("knn-hit-ordering");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-knn-hits failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-knn-hits dir=" + source.toAbsolutePath());
        return 0;
    }

    /** Verifies {@code highlights.tsv} mirrors {@link #runVerifyScoring}. */
    private static int runVerifyHighlightOffsets(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-highlight-offsets <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("highlight-offset-corpus");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-highlight-offsets failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-highlight-offsets dir=" + source.toAbsolutePath());
        return 0;
    }

    /** Verifies {@code fvh-phrases.tsv} mirrors {@link #runVerifyScoring}. */
    private static int runVerifyFvhPhrases(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-fvh-phrases <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("fast-vector-highlight-phrases");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-fvh-phrases failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-fvh-phrases dir=" + source.toAbsolutePath());
        return 0;
    }

    /**
     * Re-runs the parent-block join queries in {@code <dir>} and asserts
     * the two TSVs (join-to-parent-hits.tsv and join-to-child-hits.tsv)
     * match within {@code 1e-6}. Mirrors {@link #runVerifyScoring}.
     */
    private static int runVerifyJoinHits(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-join-hits <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("parent-block-corpus");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-join-hits failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-join-hits dir=" + source.toAbsolutePath());
        return 0;
    }

    /** Verifies the two grouping TSVs mirrors {@link #runVerifyScoring}. */
    private static int runVerifyGroupingResults(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-grouping-results <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("grouping-result-corpus");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-grouping-results failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-grouping-results dir=" + source.toAbsolutePath());
        return 0;
    }

    /** Verifies {@code classifier-labels.tsv}. The held-out test bodies are
     *  derived from the seed, so this verifier takes an explicit seed
     *  (mirrors {@code gen}/{@code verify}). */
    private static int runVerifyClassifierLabels(String[] args, PrintStream out, PrintStream err) {
        if (args.length < 2 || args.length > 3) {
            err.println("usage: verify-classifier-labels <dir> [seed]");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        long seed = 0L;
        if (args.length == 3) {
            seed = parseSeed(args[2], err);
            if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[2])) {
                return 1;
            }
        }
        try {
            CorpusScenario scenario = Scenarios.require("classifier-label-corpus");
            scenario.verify(source, seed);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-classifier-labels failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-classifier-labels dir=" + source.toAbsolutePath() + " seed=" + seed);
        return 0;
    }

    /** Verifies one of the two monitor-module scenarios. The sub-flag
     *  ("blob"|"segment") picks the scenario; the seed gate is identical to
     *  {@link #runVerifyClassifierLabels} because Determinism.idBytes (and
     *  therefore the segment id stamped in the file headers) is seeded.
     *
     *  <p>Usage: {@code verify-monitor <blob|segment> <dir> [seed]}.
     */
    private static int runVerifyMonitor(String[] args, PrintStream out, PrintStream err) {
        if (args.length < 3 || args.length > 4) {
            err.println("usage: verify-monitor <blob|segment> <dir> [seed]");
            return 1;
        }
        String which = args[1];
        String scenarioName;
        switch (which) {
            case "blob" -> scenarioName = "monitor-query-blob";
            case "segment" -> scenarioName = "monitor-index-segment";
            default -> {
                err.println("invalid monitor sub-flag (expected 'blob' or 'segment'): " + which);
                return 1;
            }
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[2]);
        long seed = 0L;
        if (args.length == 4) {
            seed = parseSeed(args[3], err);
            if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[3])) {
                return 1;
            }
        }
        try {
            CorpusScenario scenario = Scenarios.require(scenarioName);
            scenario.verify(source, seed);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-monitor failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-monitor variant=" + which + " dir="
                + source.toAbsolutePath() + " seed=" + seed);
        return 0;
    }

    /** Re-verifies a replicator scenario. Sprint 114 T19 only ships one
     *  sub-flag, {@code copystate}, which targets the
     *  {@code replicator-nrt-copystate} scenario. Future replicator
     *  scenarios (HTTP frames, session/revision) are currently
     *  represented as Manifest.DEFERRED_ROWS — when they land they will
     *  plug in here with additional sub-flags. The seed is mandatory
     *  because Determinism.idBytes is seeded.
     *
     *  <p>Usage: {@code verify-replicator copystate <dir> <seed>}. */
    private static int runVerifyReplicator(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 4) {
            err.println("usage: verify-replicator <copystate> <dir> <seed>");
            return 1;
        }
        String which = args[1];
        String scenarioName;
        switch (which) {
            case "copystate" -> scenarioName = "replicator-nrt-copystate";
            default -> {
                err.println("invalid replicator sub-flag (expected 'copystate'): " + which);
                return 1;
            }
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[2]);
        long seed = parseSeed(args[3], err);
        if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[3])) {
            return 1;
        }
        try {
            CorpusScenario scenario = Scenarios.require(scenarioName);
            scenario.verify(source, seed);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-replicator failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-replicator variant=" + which + " dir="
                + source.toAbsolutePath() + " seed=" + seed);
        return 0;
    }

    /**
     * Re-evaluates the {@code expressions-eval-corpus} catalogue against the
     * Lucene index in {@code <dir>}, recompiles every JavaScript expression
     * from source, and asserts the {@code expressions-eval.tsv} row values
     * match the recomputed values within the scenario's relative tolerance.
     * Mirrors {@link #runVerifyScoring}. The seed is irrelevant for verify
     * (the TSV pins the expected values).
     */
    private static int runVerifyExpressionsEval(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-expressions-eval <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("expressions-eval-corpus");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-expressions-eval failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-expressions-eval dir=" + source.toAbsolutePath());
        return 0;
    }

    /**
     * Sprint 114 T22 (rmp 4630). Re-verifies the
     * {@code queryparser-trees-and-hits} scenario in {@code <dir>}: it
     * re-parses every (parser_id, query_id) in the catalogue, asserts the
     * recorded {@code qp-trees.tsv} {@code toString()} matches, re-executes
     * each Query, and asserts the recorded {@code qp-hits.tsv} rows match
     * within ±1e-6. Single sub-command intentionally handles BOTH TSVs.
     */
    private static int runVerifyQueryparser(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-queryparser <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("queryparser-trees-and-hits");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-queryparser failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-queryparser dir=" + source.toAbsolutePath());
        return 0;
    }

    /** Verifies {@code queries-hits.tsv} mirrors {@link #runVerifyScoring}. */
    private static int runVerifyQueriesHits(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-queries-hits <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            CorpusScenario scenario = Scenarios.require("queries-hit-corpus");
            scenario.verify(source, 0L);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-queries-hits failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-queries-hits dir=" + source.toAbsolutePath());
        return 0;
    }

    private static long parseSeed(String s, PrintStream err) {
        try {
            // Allow 0x-prefixed hex for ergonomic CLI usage.
            if (s.startsWith("0x") || s.startsWith("0X")) {
                return Long.parseUnsignedLong(s.substring(2), 16);
            }
            return Long.parseLong(s);
        } catch (NumberFormatException e) {
            err.println("invalid seed (expected int64 or 0x-prefixed hex): " + s);
            return Long.MIN_VALUE;
        }
    }

    private static void usage(PrintStream out) {
        out.println("lucene-fixtures - Gocene binary-compatibility harness");
        out.println();
        out.println("Commands:");
        out.println("  gen      <scenario> <seed> <target>   generate the Lucene fixture for <scenario>");
        out.println("  verify   <scenario> <seed> <source>   verify <source> against <scenario>");
        out.println("  list                                  list registered scenario names (one per line)");
        out.println("  manifest [seed]                       print the baseline TSV manifest (seed defaults to 0xC0FFEE)");
        out.println("  check    <dir>                        run Lucene's CheckIndex on <dir>; exit 0 iff clean");
        out.println("  verify-scoring  <dir>                 re-run BM25 queries in <dir> and compare to scoring.tsv");
        out.println("  verify-knn-hits <dir>                 re-run KNN queries in <dir> and compare to knn-hits.tsv");
        out.println("  verify-queries-hits <dir>             re-run the queries-module catalogue in <dir> and compare to queries-hits.tsv");
        out.println("  verify-highlight-offsets <dir>        re-run UnifiedHighlighter in <dir> and compare to highlights.tsv");
        out.println("  verify-fvh-phrases <dir>              re-run FastVectorHighlighter in <dir> and compare to fvh-phrases.tsv");
        out.println("  verify-join-hits <dir>                re-run ToParent/ToChildBlockJoinQuery in <dir> and compare to join-*.tsv");
        out.println("  verify-grouping-results <dir>         re-run the grouping collectors in <dir> and compare to grouping-*.tsv");
        out.println("  verify-classifier-labels <dir> [seed] re-run the classifiers in <dir> (held-out built from seed) and compare to classifier-labels.tsv");
        out.println("  verify-monitor <blob|segment> <dir> [seed] re-verify the monitor-query-blob OR monitor-index-segment fixture in <dir>");
        out.println("  verify-replicator <copystate> <dir> <seed> re-verify the replicator-nrt-copystate fixture in <dir>");
        out.println("  verify-expressions-eval <dir>        recompile + re-evaluate the expressions catalogue in <dir> and compare to expressions-eval.tsv");
        out.println("  verify-queryparser <dir>             re-parse and re-execute the queryparser catalogue in <dir> and compare to qp-trees.tsv + qp-hits.tsv");
    }
}
