package io.github.flaviocfoliveira.gocene.lucenefixtures;

import java.io.IOException;
import java.io.PrintStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HexFormat;
import java.util.List;

/**
 * Manifest writer: produces the baseline TSV that records the expected sha256
 * of every registered scenario for a pinned canary seed.
 *
 * <p>The hash is computed over a <em>normalised</em> form:
 * <pre>
 *   sha256(  concat over files sorted by name of:
 *              filename + "\n" + filesize-decimal + "\n" + sha256-hex(file) + "\n"
 *         )
 * </pre>
 * This intentionally hashes the directory shape and per-file checksums rather
 * than every byte of every file: it lets the manifest stay stable across
 * environments where Lucene legitimately injects per-segment IDs or other
 * per-run identifiers, while still detecting any logical drift.
 */
public final class Manifest {

    /** TSV column header, written once at the top of the manifest. */
    public static final String HEADER =
            "scenario\tcanary_seed\tsha256\tfile_count\tnotes";

    /** Scenarios listed in the manifest but not produced by the harness yet. */
    public static final List<String[]> DEFERRED_ROWS = List.of(
            new String[]{"hunspell-blob", "(deferred)", "0", "precompiled third-party asset; covered by later sprint"},
            new String[]{"snowball-blob", "(deferred)", "0", "precompiled third-party asset; covered by later sprint"}
    );

    private Manifest() {}

    /** Print the full TSV to {@code out} using a fresh temp dir per scenario. */
    public static void print(long seed, PrintStream out) throws IOException {
        out.println(HEADER);
        for (var entry : Scenarios.all().entrySet()) {
            String scenarioName = entry.getKey();
            CorpusScenario scenario = entry.getValue();
            Path tmp = Files.createTempDirectory("gocene-manifest-" + scenarioName + "-");
            try {
                Determinism.seed(seed);
                scenario.generate(tmp, seed);
                Snapshot snap = snapshot(tmp);
                out.println(String.join("\t",
                        scenarioName,
                        Long.toString(seed),
                        snap.sha256,
                        Integer.toString(snap.fileCount),
                        ""));
            } finally {
                deleteRecursively(tmp);
            }
        }
        for (String[] row : DEFERRED_ROWS) {
            out.println(String.join("\t",
                    row[0], Long.toString(seed), row[1], row[2], "gap_notes=\"deferred to per-package task\""));
        }
    }

    /** Compute the normalised digest for {@code dir}. */
    public static Snapshot snapshot(Path dir) throws IOException {
        List<Path> files = new ArrayList<>();
        try (var stream = Files.walk(dir)) {
            stream.filter(Files::isRegularFile)
                    .filter(Manifest::includeForHash)
                    .forEach(files::add);
        }
        files.sort(Comparator.comparing(p -> dir.relativize(p).toString()));
        MessageDigest md = newSha256();
        for (Path f : files) {
            String rel = dir.relativize(f).toString();
            long size = Files.size(f);
            String fileHash = HexFormat.of().formatHex(sha256(f));
            md.update(rel.getBytes(java.nio.charset.StandardCharsets.UTF_8));
            md.update((byte) '\n');
            md.update(Long.toString(size).getBytes(java.nio.charset.StandardCharsets.UTF_8));
            md.update((byte) '\n');
            md.update(fileHash.getBytes(java.nio.charset.StandardCharsets.UTF_8));
            md.update((byte) '\n');
        }
        return new Snapshot(HexFormat.of().formatHex(md.digest()), files.size());
    }

    public record Snapshot(String sha256, int fileCount) {}

    /**
     * Filters files that participate in the deterministic digest. The
     * {@code .si} (Lucene99SegmentInfoFormat) file stamps a wall-clock
     * timestamp into its diagnostics map and is therefore excluded; the
     * {@code write.lock} file is empty and unrelated to format compat.
     */
    private static boolean includeForHash(Path file) {
        String name = file.getFileName().toString();
        if (name.endsWith(".si")) return false;
        if (name.equals("write.lock")) return false;
        return true;
    }

    private static byte[] sha256(Path file) throws IOException {
        MessageDigest md = newSha256();
        try (var in = Files.newInputStream(file)) {
            byte[] buf = new byte[8192];
            int n;
            while ((n = in.read(buf)) > 0) {
                md.update(buf, 0, n);
            }
        }
        return md.digest();
    }

    private static MessageDigest newSha256() {
        try {
            return MessageDigest.getInstance("SHA-256");
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 unavailable on this JVM", e);
        }
    }

    private static void deleteRecursively(Path root) throws IOException {
        if (!Files.exists(root)) return;
        try (var stream = Files.walk(root)) {
            List<Path> all = new ArrayList<>();
            stream.forEach(all::add);
            // Delete deepest first.
            all.sort(Comparator.reverseOrder());
            for (Path p : all) {
                Files.deleteIfExists(p);
            }
        }
    }
}
