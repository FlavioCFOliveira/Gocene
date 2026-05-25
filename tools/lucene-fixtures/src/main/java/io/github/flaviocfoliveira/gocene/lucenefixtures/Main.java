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
    }
}
