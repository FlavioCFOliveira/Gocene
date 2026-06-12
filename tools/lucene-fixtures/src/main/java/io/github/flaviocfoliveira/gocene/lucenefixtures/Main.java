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
            case "verify-sandbox" -> runVerifySandbox(args, out, err);
            case "verify-misc" -> runVerifyMisc(args, out, err);
            case "verify-memory-flush" -> runVerifyMemoryFlush(args, out, err);
            case "verify-sweetspot" -> runVerifySweetspot(args, out, err);
            case "verify-bwc" -> runVerifyBwc(args, out, err);
            case "verify-diagnostic" -> runVerifyDiagnostic(args, out, err);
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

    /**
     * Sprint 114 T23 (rmp 4631). Re-verifies one of the sandbox scenarios.
     *
     * <p>Usage: {@code verify-sandbox <idversion|quantization> <dir> <seed>}.
     *
     * <p>The {@code idversion} sub-flag targets the
     * {@code sandbox-idversion-postings} scenario and re-runs every
     * (id, version) probe via {@link org.apache.lucene.sandbox.codecs.idversion
     * .IDVersionSegmentTermsEnum#seekExact}.
     *
     * <p>The {@code quantization} sub-flag is intentionally a no-op fast-path
     * that emits a single {@code ok ... status=deferred} line and exits 0.
     * The sandbox audit row "Quantization sampling codec: Pure port without
     * tests, fixtures, or writer parity" is tracked as a DEFERRED row in
     * {@link Manifest#DEFERRED_ROWS} (row name
     * {@code sandbox-quantization-codec}) because Lucene 10.4.0
     * {@code sandbox/codecs/quantization} ships only
     * {@code KMeans} and {@code SampleReader} — there is no
     * {@code KnnVectorsFormat}, {@code PostingsFormat}, or {@code Codec}
     * under that subpackage to produce a separate on-disk artefact. The
     * scalar-quantized HNSW persisted artefact is the production
     * {@code Lucene104HnswScalarQuantizedVectorsFormat}, already covered by
     * the T7 scenario {@code scalar-quantized-knn}.
     */
    private static int runVerifySandbox(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 4) {
            err.println("usage: verify-sandbox <idversion|quantization> <dir> <seed>");
            return 1;
        }
        String which = args[1];
        java.nio.file.Path source = java.nio.file.Path.of(args[2]);
        long seed = parseSeed(args[3], err);
        if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[3])) {
            return 1;
        }
        switch (which) {
            case "idversion" -> {
                try {
                    CorpusScenario scenario = Scenarios.require("sandbox-idversion-postings");
                    scenario.verify(source, seed);
                } catch (IllegalArgumentException e) {
                    err.println(e.getMessage());
                    return 2;
                } catch (IOException e) {
                    err.println("verify-sandbox idversion failed: " + e.getMessage());
                    return 4;
                }
                out.println("ok verify-sandbox variant=idversion dir="
                        + source.toAbsolutePath() + " seed=" + seed);
                return 0;
            }
            case "quantization" -> {
                // Manifest.DEFERRED_ROWS carries the audit footprint
                // (sandbox-quantization-codec). The CLI surfaces a clear
                // deferral status so callers (Go-side tests, CI runners)
                // can branch on it without parsing the manifest.
                out.println("ok verify-sandbox variant=quantization dir="
                        + source.toAbsolutePath() + " seed=" + seed
                        + " status=deferred manifest_row=sandbox-quantization-codec "
                        + "reason=\"Lucene 10.4.0 sandbox/codecs/quantization ships only "
                        + "KMeans+SampleReader; production quantization artefact is "
                        + "Lucene104HnswScalarQuantizedVectorsFormat covered by scalar-quantized-knn\"");
                return 0;
            }
            default -> {
                err.println("invalid sandbox sub-flag (expected 'idversion' or 'quantization'): " + which);
                return 1;
            }
        }
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

    /**
     * Sprint 114 T24 (rmp 4632). Re-verifies one of the misc-module
     * scenarios. Sub-flag selects between {@code splitter}
     * (misc-index-splitter-input) and {@code highfreq}
     * (misc-highfreq-terms-corpus). Seed is mandatory.
     *
     * <p>Usage: {@code verify-misc <splitter|highfreq> <dir> <seed>}.
     */
    private static int runVerifyMisc(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 4) {
            err.println("usage: verify-misc <splitter|highfreq> <dir> <seed>");
            return 1;
        }
        String which = args[1];
        String scenarioName;
        switch (which) {
            case "splitter" -> scenarioName = "misc-index-splitter-input";
            case "highfreq" -> scenarioName = "misc-highfreq-terms-corpus";
            default -> {
                err.println("invalid misc sub-flag (expected 'splitter' or 'highfreq'): " + which);
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
            err.println("verify-misc " + which + " failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-misc variant=" + which + " dir="
                + source.toAbsolutePath() + " seed=" + seed);
        return 0;
    }

    /**
     * Sprint 114 T25 (rmp 4633). Re-verifies the {@code memory-index-flush}
     * scenario in {@code <dir>}: reopens the directory and asserts the single
     * flushed doc plus every token term (with payload bytes) is present.
     * Seed is mandatory because {@link Determinism#idBytes} is seeded and the
     * payload bytes are seed-derived.
     *
     * <p>Usage: {@code verify-memory-flush <dir> <seed>}.
     */
    private static int runVerifyMemoryFlush(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 3) {
            err.println("usage: verify-memory-flush <dir> <seed>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        long seed = parseSeed(args[2], err);
        if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[2])) {
            return 1;
        }
        try {
            CorpusScenario scenario = Scenarios.require("memory-index-flush");
            scenario.verify(source, seed);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        } catch (IOException e) {
            err.println("verify-memory-flush failed: " + e.getMessage());
            return 4;
        }
        out.println("ok verify-memory-flush dir=" + source.toAbsolutePath() + " seed=" + seed);
        return 0;
    }

    /**
     * Sprint 114 T24 (rmp 4632). SweetSpotSimilarity is a runtime
     * {@link org.apache.lucene.search.similarities.Similarity} subclass
     * (no persisted artefact). This sub-command opens the
     * {@code search-scoring-corpus} index in {@code <dir>}, re-scores it
     * under BM25 and under SweetSpotSimilarity, and asserts (a) hit-set
     * parity per query, (b) at least one score differs by more than 1e-3.
     *
     * <p>Usage: {@code verify-sweetspot <dir>}.
     */
    private static int runVerifySweetspot(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 2) {
            err.println("usage: verify-sweetspot <dir>");
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[1]);
        try {
            int compared = SweetspotProbe.run(source);
            out.println("ok verify-sweetspot dir=" + source.toAbsolutePath()
                    + " queries_compared=" + compared);
            return 0;
        } catch (IOException e) {
            err.println("verify-sweetspot failed: " + e.getMessage());
            return 4;
        }
    }

    /**
     * Sprint 114 T26 (rmp 4634). Dispatches verification for one of the
     * backward_codecs scenarios by name. The sub-flag selects which
     * scenario; the seed is mandatory because Determinism.idBytes is seeded.
     *
     * <p>Usage: {@code verify-bwc <scenario> <dir> <seed>} where {@code
     * <scenario>} is one of {@code bwc-packed64-legacy} or
     * {@code bwc-big-endian-store}. The remaining seven backward_codecs
     * audit rows are read-only in Lucene 10.4.0 and tracked as
     * DEFERRED_ROWS in Manifest.java; this sub-command rejects unknown
     * scenario names with exit 1.
     */
    private static int runVerifyBwc(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 4) {
            err.println("usage: verify-bwc <scenario> <dir> <seed>");
            return 1;
        }
        String which = args[1];
        java.nio.file.Path source = java.nio.file.Path.of(args[2]);
        long seed = parseSeed(args[3], err);
        if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[3])) {
            return 1;
        }
        // Scenarios that have real Java-generated fixtures.
        switch (which) {
            case "bwc-packed64-legacy", "bwc-big-endian-store" -> {
                try {
                    CorpusScenario scenario = Scenarios.require(which);
                    scenario.verify(source, seed);
                } catch (IllegalArgumentException e) {
                    err.println(e.getMessage());
                    return 2;
                } catch (IOException e) {
                    err.println("verify-bwc " + which + " failed: " + e.getMessage());
                    return 4;
                }
                out.println("ok verify-bwc variant=" + which + " dir="
                        + source.toAbsolutePath() + " seed=" + seed);
                return 0;
            }
        }
        // Gocene-write -> Java-read scenarios (Sprint 14 T81).  These formats
        // are read-only in Lucene 10.4.0, so there is no Java-generated
        // fixture; verification means running CheckIndex on the Gocene-
        // produced directory.
        switch (which) {
            case "bwc-lucene99-postings", "bwc-lucene99-scalar-quantized",
                    "bwc-lucene103-postings" -> {
                try (org.apache.lucene.store.FSDirectory dir = org.apache.lucene.store.FSDirectory.open(source);
                     java.io.ByteArrayOutputStream captured = new java.io.ByteArrayOutputStream();
                     PrintStream sink = new PrintStream(captured, true, java.nio.charset.StandardCharsets.UTF_8);
                     org.apache.lucene.index.CheckIndex checker = new org.apache.lucene.index.CheckIndex(dir)) {
                    checker.setInfoStream(sink, false);
                    org.apache.lucene.index.CheckIndex.Status status = checker.checkIndex();
                    if (status.clean) {
                        int segs = status.segmentInfos == null ? 0 : status.segmentInfos.size();
                        out.println("ok verify-bwc variant=" + which + " dir=" + source.toAbsolutePath()
                                + " seed=" + seed + " segments=" + segs
                                + " missingSegments=" + status.missingSegments
                                + " mode=gocene-write-java-checkindex");
                        return 0;
                    }
                    err.println(captured.toString(java.nio.charset.StandardCharsets.UTF_8));
                    err.println("CheckIndex reported a non-clean index at " + source.toAbsolutePath());
                    return 4;
                } catch (IOException e) {
                    err.println("verify-bwc " + which + " failed: " + e.getMessage());
                    return 4;
                }
            }
            default -> {
                err.println("invalid verify-bwc scenario (see Manifest.DEFERRED_ROWS): " + which);
                return 1;
            }
        }
    }

    /**
     * Sprint 114 T5 (rmp 4611) acceptance criterion #4: on forced divergence
     * (mutated byte in fixture), report the affected file + byte offset +
     * expected vs actual bytes. This sub-command regenerates the scenario
     * into a fresh temp dir and walks both trees byte-by-byte; on the first
     * mismatch it emits a single-line JSON record to stdout and exits 4.
     *
     * <p>Usage: {@code verify-diagnostic <scenario> <seed> <source>}.
     */
    private static int runVerifyDiagnostic(String[] args, PrintStream out, PrintStream err) {
        if (args.length != 4) {
            err.println("usage: verify-diagnostic <scenario> <seed> <source>");
            return 1;
        }
        String scenarioName = args[1];
        long seed = parseSeed(args[2], err);
        if (seed == Long.MIN_VALUE && !"-9223372036854775808".equals(args[2])) {
            return 1;
        }
        java.nio.file.Path source = java.nio.file.Path.of(args[3]);
        CorpusScenario scenario;
        try {
            scenario = Scenarios.require(scenarioName);
        } catch (IllegalArgumentException e) {
            err.println(e.getMessage());
            return 2;
        }
        java.nio.file.Path tmp;
        try {
            tmp = java.nio.file.Files.createTempDirectory("gocene-diagnostic-");
        } catch (IOException e) {
            err.println("verify-diagnostic: mkdir failed: " + e.getMessage());
            return 3;
        }
        try {
            Determinism.seed(seed);
            scenario.generate(tmp, seed);
            Diagnostic diff = firstDifference(tmp, source);
            if (diff == null) {
                out.println("ok verify-diagnostic scenario=" + scenarioName
                        + " seed=" + seed + " source=" + source.toAbsolutePath());
                return 0;
            }
            // Single-line JSON with the four fields required by AC #4.
            // No external JSON library is wired into the harness; manual
            // serialisation is sufficient because the value set is bounded
            // (path is ASCII-safe relative, ints fit Java long).
            out.println("{\"file\":\"" + escapeJson(diff.file)
                    + "\",\"offset\":" + diff.offset
                    + ",\"expected\":" + (diff.expected & 0xFF)
                    + ",\"actual\":" + (diff.actual & 0xFF) + "}");
            return 4;
        } catch (IOException e) {
            err.println("verify-diagnostic failed: " + e.getMessage());
            return 3;
        } finally {
            try {
                deleteRecursively(tmp);
            } catch (IOException ignored) {
                // best-effort cleanup
            }
        }
    }

    /** Single diagnostic record carried back to {@link #runVerifyDiagnostic}. */
    private record Diagnostic(String file, long offset, byte expected, byte actual) {}

    /**
     * Walks {@code expectedDir} and {@code actualDir} in lexicographic order
     * and returns the first byte-level difference, or {@code null} if every
     * file matches. A missing-or-extra-file is also a difference: the
     * "expected" or "actual" byte is sentinel {@code -1} (0xFF wrap) and the
     * offset is the file's length on the side that exists (or 0).
     */
    private static Diagnostic firstDifference(java.nio.file.Path expectedDir,
                                              java.nio.file.Path actualDir) throws IOException {
        java.util.Set<String> expFiles = walkRelative(expectedDir);
        java.util.Set<String> actFiles = walkRelative(actualDir);
        java.util.TreeSet<String> union = new java.util.TreeSet<>();
        union.addAll(expFiles);
        union.addAll(actFiles);
        for (String rel : union) {
            boolean inExp = expFiles.contains(rel);
            boolean inAct = actFiles.contains(rel);
            if (!inExp) {
                long len = java.nio.file.Files.size(actualDir.resolve(rel));
                return new Diagnostic(rel, 0L, (byte) 0xFF,
                        len > 0 ? java.nio.file.Files.readAllBytes(actualDir.resolve(rel))[0]
                                : (byte) 0);
            }
            if (!inAct) {
                long len = java.nio.file.Files.size(expectedDir.resolve(rel));
                return new Diagnostic(rel, 0L,
                        len > 0 ? java.nio.file.Files.readAllBytes(expectedDir.resolve(rel))[0]
                                : (byte) 0, (byte) 0xFF);
            }
            byte[] exp = java.nio.file.Files.readAllBytes(expectedDir.resolve(rel));
            byte[] act = java.nio.file.Files.readAllBytes(actualDir.resolve(rel));
            int common = Math.min(exp.length, act.length);
            for (int i = 0; i < common; i++) {
                if (exp[i] != act[i]) {
                    return new Diagnostic(rel, i, exp[i], act[i]);
                }
            }
            if (exp.length != act.length) {
                // Length mismatch: report the first byte beyond the shorter side.
                if (exp.length < act.length) {
                    return new Diagnostic(rel, exp.length, (byte) 0xFF, act[exp.length]);
                } else {
                    return new Diagnostic(rel, act.length, exp[act.length], (byte) 0xFF);
                }
            }
        }
        return null;
    }

    /**
     * Walks {@code root} and returns the relative paths of every regular file
     * that participates in byte-determinism. Mirrors
     * {@link Manifest}'s {@code includeForHash}: the {@code .si} segment
     * info files stamp a wall-clock timestamp into their diagnostics map
     * and {@code write.lock} is empty, so both are excluded from the
     * diagnostic byte-compare for the same reason they are excluded from
     * the manifest digest.
     */
    private static java.util.Set<String> walkRelative(java.nio.file.Path root) throws IOException {
        java.util.Set<String> out = new java.util.TreeSet<>();
        try (var stream = java.nio.file.Files.walk(root)) {
            stream.filter(java.nio.file.Files::isRegularFile)
                    .filter(p -> {
                        String name = p.getFileName().toString();
                        return !name.endsWith(".si") && !name.equals("write.lock");
                    })
                    .forEach(p -> out.add(root.relativize(p).toString()));
        }
        return out;
    }

    private static String escapeJson(String s) {
        StringBuilder sb = new StringBuilder(s.length());
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '\\': sb.append("\\\\"); break;
                case '"': sb.append("\\\""); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                default: sb.append(c);
            }
        }
        return sb.toString();
    }

    private static void deleteRecursively(java.nio.file.Path root) throws IOException {
        if (!java.nio.file.Files.exists(root)) return;
        try (var stream = java.nio.file.Files.walk(root)) {
            java.util.List<java.nio.file.Path> all = new java.util.ArrayList<>();
            stream.forEach(all::add);
            all.sort(java.util.Comparator.reverseOrder());
            for (java.nio.file.Path p : all) {
                java.nio.file.Files.deleteIfExists(p);
            }
        }
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
        out.println("  verify-sandbox <idversion|quantization> <dir> <seed>  re-verify a sandbox scenario (quantization is deferred; see Manifest.DEFERRED_ROWS)");
        out.println("  verify-misc <splitter|highfreq> <dir> <seed>  re-verify a misc-module scenario (IndexSplitter/IndexMergeTool input OR HighFreqTerms corpus)");
        out.println("  verify-memory-flush <dir> <seed>     re-verify the memory-index-flush fixture (single segment from a MemoryIndex flush) in <dir>");
        out.println("  verify-sweetspot <dir>               re-score the search-scoring-corpus index under SweetSpotSimilarity and assert (a) hit-set parity with BM25, (b) at least one score differs > 1e-3");
        out.println("  verify-bwc <bwc-packed64-legacy|bwc-big-endian-store> <dir> <seed>  re-verify a backward_codecs scenario (the other 7 backward_codecs rows are DEFERRED — see Manifest.DEFERRED_ROWS)");
        out.println("  verify-diagnostic <scenario> <seed> <source>  regenerate the scenario into a temp dir, byte-compare against <source>, emit one-line JSON {file,offset,expected,actual} on the first mismatch (exit 4) or 'ok' on parity");
    }
}
