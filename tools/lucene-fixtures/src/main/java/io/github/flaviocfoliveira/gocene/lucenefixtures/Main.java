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
    }
}
