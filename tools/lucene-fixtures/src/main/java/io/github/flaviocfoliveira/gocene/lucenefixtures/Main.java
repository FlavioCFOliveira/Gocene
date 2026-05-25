package io.github.flaviocfoliveira.gocene.lucenefixtures;

import java.io.IOException;
import java.io.PrintStream;
import java.nio.file.Path;

/**
 * Entry point for the {@code lucene-fixtures} harness.
 *
 * <pre>
 *   java -jar lucene-fixtures.jar gen    &lt;scenario&gt; &lt;seed&gt; &lt;target&gt;
 *   java -jar lucene-fixtures.jar verify &lt;scenario&gt; &lt;seed&gt; &lt;source&gt;
 *   java -jar lucene-fixtures.jar list
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
        Scenarios.all().forEach((k, v) -> out.println(k + "\t" + v.description()));
        return 0;
    }

    private static long parseSeed(String s, PrintStream err) {
        try {
            return Long.parseLong(s);
        } catch (NumberFormatException e) {
            err.println("invalid seed (expected int64): " + s);
            return Long.MIN_VALUE;
        }
    }

    private static void usage(PrintStream out) {
        out.println("lucene-fixtures - Gocene binary-compatibility harness");
        out.println();
        out.println("Commands:");
        out.println("  gen    <scenario> <seed> <target>   generate the Lucene fixture for <scenario>");
        out.println("  verify <scenario> <seed> <source>   verify <source> against <scenario>");
        out.println("  list                                list registered scenarios");
    }
}
