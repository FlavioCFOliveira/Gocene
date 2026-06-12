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

    /**
     * Scenarios listed in the manifest but not produced by the harness yet.
     *
     * <p>Row tuple convention (positional):
     * <ol>
     *   <li>{@code [0]} scenario name (kebab-case)</li>
     *   <li>{@code [1]} sha256 (always {@code "(deferred)"})</li>
     *   <li>{@code [2]} file_count (always {@code "0"})</li>
     *   <li>{@code [3]} reason / legacy-notes (used as fallback)</li>
     *   <li>{@code [4]} optional FULL notes column verbatim. When present
     *       it overrides {@code gap_notes="deferred to per-package task"};
     *       T19 (rmp 4627) uses this to carry the verbatim audit
     *       {@code gap_notes} per row plus the explicit
     *       {@code reason} that Lucene 10.4.0 removed
     *       {@code org.apache.lucene.replicator.http.HttpReplicator} and the
     *       IndexRevision wire surface.</li>
     * </ol>
     */
    public static final List<String[]> DEFERRED_ROWS = List.of(
            new String[]{"hunspell-blob", "(deferred)", "0", "precompiled third-party asset; covered by later sprint"},
            new String[]{"snowball-blob", "(deferred)", "0", "precompiled third-party asset; covered by later sprint"},
            // Sprint 114 T19 (rmp 4627). Both HTTP replicator and IndexRevision
            // wire formats are removed from Lucene 10.4.0 production sources
            // (`lucene/replicator/src/java/org/apache/lucene/replicator/`
            // contains ONLY the `nrt` subpackage in tag releases/lucene/10.4.0).
            // The deferred rows preserve the audit footprint until a future
            // backward-compat sprint reintroduces fixtures pulled from older
            // Lucene branches.
            new String[]{"replicator-http-frames", "(deferred)", "0",
                    "Lucene 10.4.0 removed HttpReplicator surface",
                    "gap_notes=\"No Java-served HTTP replicator fixtures.\"; "
                            + "reason=\"Lucene 10.4.0 removed org.apache.lucene.replicator.http.HttpReplicator "
                            + "and the IndexRevision wire surface; covered by a future backward-compat sprint.\""},
            new String[]{"replicator-session-revision", "(deferred)", "0",
                    "Lucene 10.4.0 removed SessionToken/RevisionFile surface",
                    "gap_notes=\"No cross-engine replication transcript validated against Lucene.\"; "
                            + "reason=\"Lucene 10.4.0 removed org.apache.lucene.replicator.http.HttpReplicator "
                            + "and the IndexRevision wire surface; covered by a future backward-compat sprint.\""},
            // Sprint 114 T23 (rmp 4631). The sandbox `quantization` package in
            // Apache Lucene 10.4.0 (path
            // `lucene/sandbox/src/java/org/apache/lucene/sandbox/codecs/quantization/`
            // at tag `releases/lucene/10.4.0`) ships ONLY `KMeans.java` and
            // `SampleReader.java` — there is NO `KnnVectorsFormat` /
            // `PostingsFormat` / `Codec` under that subpackage and therefore
            // no on-disk artefact distinct from the production
            // `Lucene104HnswScalarQuantizedVectorsFormat` (which lives in
            // `lucene-core` under `org.apache.lucene.codecs.lucene104` and is
            // already covered by the T7 scenario `scalar-quantized-knn`). The
            // audit row is preserved as a DEFERRED_ROW so the footprint stays
            // visible until a future sprint either ports a sandbox-specific
            // quantization format (none planned) or formally collapses the
            // row into `scalar-quantized-knn`.
            new String[]{"sandbox-quantization-codec", "(deferred)", "0",
                    "Lucene 10.4.0 sandbox/codecs/quantization ships only KMeans+SampleReader",
                    "gap_notes=\"Quantization sampling codec: Pure port without tests, fixtures, or writer parity\"; "
                            + "reason=\"Lucene 10.4.0 sandbox `codecs/quantization` ships ONLY "
                            + "org.apache.lucene.sandbox.codecs.quantization.KMeans and SampleReader "
                            + "(no KnnVectorsFormat/PostingsFormat/Codec under that subpackage); the "
                            + "scalar-quantized HNSW persisted artefact is the production "
                            + "org.apache.lucene.codecs.lucene104.Lucene104HnswScalarQuantizedVectorsFormat "
                            + "which is already covered by the T7 scenario `scalar-quantized-knn`. "
                            + "Sandbox-specific binary parity is therefore not applicable; covered "
                            + "by a future sprint that either ports a sandbox quantization format "
                            + "(none planned in Lucene 10.4.0) or formally folds this row into scalar-quantized-knn.\""},
            // Sprint 114 T26 (rmp 4634). Seven of the nine backward_codecs
            // audit rows have NO writable surface in Lucene 10.4.0's
            // lucene-backward-codecs jar — every per-version codec ships
            // ONLY readers; the write paths throw
            // {@code UnsupportedOperationException("Old codecs may only be
            // used for reading")} (e.g. {@code
            // org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat
            // #fieldsConsumer}, {@code Lucene90HnswVectorsFormat
            // #fieldsWriter}). Producing fixtures for those formats requires
            // building older Apache Lucene jars (7.x / 9.x / 10.3.x), which
            // is out of scope for this sprint and the compatibility mandate's
            // 10.4.0 reference pin. Each row below preserves the audit
            // footprint until a future backward-compat sprint plugs in the
            // older Lucene jars (or pulls fixtures from a pre-built corpus).
            new String[]{"bwc-lucene70-si", "(deferred)", "0",
                    "Lucene 10.4.0 Lucene70SegmentInfoFormat is read-only",
                    "gap_notes=\"No real Lucene 7 fixture committed; rw tests are self-emitted.\"; "
                            + "reason=\"org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat#write "
                            + "throws UnsupportedOperationException(\\\"Old formats can't be used for writing\\\") "
                            + "in Lucene 10.4.0; producing a Lucene-7-emitted .si requires an older Lucene jar "
                            + "(7.x branch) which is out of binary-compat-mandate scope (10.4.0 reference pin); "
                            + "covered by a future backward-compat sprint.\""},
            new String[]{"bwc-lucene90-hnsw-v0", "(deferred)", "0",
                    "Lucene 10.4.0 Lucene90HnswVectorsFormat is read-only",
                    "gap_notes=\"No Lucene-9 fixture committed.\"; "
                            + "reason=\"org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat#fieldsWriter "
                            + "throws UnsupportedOperationException(\\\"Old codecs may only be used for reading\\\") "
                            + "in Lucene 10.4.0; producing a Lucene-9.x HNSW v0 segment requires an older Lucene jar; "
                            + "covered by a future backward-compat sprint.\""},
            new String[]{"bwc-lucene99-postings", "(deferred)", "0",
                    "Lucene 10.4.0 Lucene99PostingsFormat is read-only",
                    "gap_notes=\"Gocene-write -> Java-read validated via CheckIndex (Sprint 14 T81a).\"; "
                            + "reason=\"org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat#fieldsConsumer "
                            + "throws UnsupportedOperationException in Lucene 10.4.0; producing a Lucene-9.9 postings "
                            + "segment requires an older Lucene jar. Gocene now ports the test-only Lucene99PostingsWriter "
                            + "and validates with Java CheckIndex.\""},
            new String[]{"bwc-lucene99-scalar-quantized", "(deferred)", "0",
                    "Lucene 10.4.0 Lucene99ScalarQuantizedVectorsFormat is read-only",
                    "gap_notes=\"Gocene-write -> Java-read validated via CheckIndex (Sprint 14 T81c).\"; "
                            + "reason=\"org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat"
                            + "#fieldsWriter throws UnsupportedOperationException(\\\"Old codecs may only be used for reading\\\") "
                            + "in Lucene 10.4.0; producing a Lucene-9.9 scalar-quantised vectors segment requires an older "
                            + "Lucene jar. Gocene now ports the test-only Lucene99ScalarQuantizedVectorsWriter "
                            + "and validates with Java CheckIndex.\""},
            new String[]{"bwc-lucene103-postings", "(deferred)", "0",
                    "Lucene 10.4.0 Lucene103PostingsFormat is read-only",
                    "gap_notes=\"Gocene-write -> Java-read validated via CheckIndex (Sprint 14 T81b).\"; "
                            + "reason=\"org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat#fieldsConsumer "
                            + "throws UnsupportedOperationException(\\\"This postings format may not be used for writing, "
                            + "use the current postings format\\\") in Lucene 10.4.0; producing a Lucene-10.3 postings "
                            + "segment requires an older Lucene jar. Gocene now ports the test-only Lucene103PostingsWriter "
                            + "and validates with Java CheckIndex.\""},
            new String[]{"bwc-lucene40-blocktree", "(deferred)", "0",
                    "Lucene 10.4.0 lucene40/blocktree ships only Reader/Iterator/Frame",
                    "gap_notes=\"Only reader port; no rw or fixture test.\"; "
                            + "reason=\"org.apache.lucene.backward_codecs.lucene40.blocktree package in Lucene 10.4.0 "
                            + "ships ONLY Lucene40BlockTreeTermsReader/SegmentTermsEnum/IntersectTermsEnum (no writer); "
                            + "producing a Lucene-4.0 BlockTree fixture requires the legacy lucene-codecs.jar "
                            + "(Lucene 4 branch), out of binary-compat-mandate scope; covered by a future "
                            + "backward-compat sprint.\""},
            new String[]{"bwc-multi-version-corpora", "(deferred)", "0",
                    "Lucene 10.4.0 cannot produce older multi-version index ZIPs",
                    "gap_notes=\"Tests are skeletons; no actual multi-version Lucene index ZIPs committed.\"; "
                            + "reason=\"Producing the per-major-version index ZIPs that org.apache.lucene.backward_index"
                            + ".TestBasicBackwardsCompatibility consumes requires building EACH old Lucene major (7/8/9/10) "
                            + "and emitting an index per branch; out of binary-compat-mandate scope (10.4.0 reference pin); "
                            + "covered by a future backward-compat sprint that maintains a multi-version fixture corpus.\""}
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
            // Column [4], when present, carries the FULL verbatim notes text
            // (e.g. T19 audit gap_notes + removal reason). Otherwise fall
            // back to the historical "deferred to per-package task" hint.
            String notes = row.length >= 5 ? row[4] : "gap_notes=\"deferred to per-package task\"";
            out.println(String.join("\t",
                    row[0], Long.toString(seed), row[1], row[2], notes));
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
