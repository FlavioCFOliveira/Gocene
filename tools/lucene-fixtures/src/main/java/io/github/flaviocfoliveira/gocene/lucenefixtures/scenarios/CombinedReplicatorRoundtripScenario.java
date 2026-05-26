package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.replicator.nrt.CopyState;
import org.apache.lucene.replicator.nrt.FileMetaData;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexOutput;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Sprint 114 T5 (rmp 4611), S4 {@code combined-replicator-roundtrip}.
 * Captures a primary->replica wire transcript: {@value #FRAMES_NAME} carries the
 * CodecUtil-framed CopyState payload (via {@link ReplicatorNrtCopyStateScenario}'s
 * canonical writer) and {@value #FILES_NAME} lists path/length/checksum per
 * FileMetaData. Gocene-write leg deferred (no SimplePrimaryNode equivalent
 * in Gocene's replicator/nrt port yet).
 */
public final class CombinedReplicatorRoundtripScenario implements CorpusScenario {

    public static final String NAME = "combined-replicator-roundtrip";
    public static final String FRAMES_NAME = "s4-frames.bin";
    public static final String FILES_NAME = "s4-files.tsv";
    public static final String CODEC = "GoceneCombinedReplicatorRoundtrip";
    public static final int VERSION = 0;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Primary->replica NRT wire transcript (1 CopyState frame) + per-file metadata TSV.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        CopyState state = ReplicatorNrtCopyStateScenario.buildCopyState(seed);
        try (FSDirectory dir = FSDirectory.open(target);
             IndexOutput out = dir.createOutput(FRAMES_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, Determinism.idBytes(seed), "");
            ReplicatorNrtCopyStateScenario.writeCopyState(state, out);
            CodecUtil.writeFooter(out);
        }
        // Emit the per-file metadata TSV, sorted by path for stability.
        List<FileRow> rows = filesToRows(state.files());
        rows.sort((a, b) -> a.path().compareTo(b.path()));
        writeFilesTsv(target.resolve(FILES_NAME), rows);
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        CopyState expected = ReplicatorNrtCopyStateScenario.buildCopyState(seed);
        // (1) Decode and assert the wire frame round-trips.
        try (FSDirectory dir = FSDirectory.open(source);
             ChecksumIndexInput in = dir.openChecksumInput(FRAMES_NAME)) {
            CodecUtil.checkIndexHeader(in, CODEC, VERSION, VERSION,
                    Determinism.idBytes(seed), "");
            CopyState decoded = ReplicatorNrtCopyStateScenario.readCopyState(in);
            if (decoded.gen() != expected.gen()
                    || decoded.version() != expected.version()
                    || decoded.primaryGen() != expected.primaryGen()) {
                throw new IOException(NAME + ": header field drift after wire round-trip");
            }
            if (decoded.files().size() != expected.files().size()) {
                throw new IOException(NAME + ": file count drift, expected="
                        + expected.files().size() + " got=" + decoded.files().size());
            }
            CodecUtil.checkFooter(in);
        }
        // (2) Re-read the TSV and confirm it matches the canonical file list.
        Path tsv = source.resolve(FILES_NAME);
        if (!Files.isRegularFile(tsv)) {
            throw new IOException(NAME + ": missing " + FILES_NAME);
        }
        List<FileRow> recorded = readFilesTsv(tsv);
        List<FileRow> expectedRows = filesToRows(expected.files());
        expectedRows.sort((a, b) -> a.path().compareTo(b.path()));
        if (recorded.size() != expectedRows.size()) {
            throw new IOException(NAME + ": TSV row count drift recorded="
                    + recorded.size() + " expected=" + expectedRows.size());
        }
        for (int i = 0; i < recorded.size(); i++) {
            if (!recorded.get(i).equals(expectedRows.get(i))) {
                throw new IOException(NAME + ": TSV row " + i + " drift: "
                        + recorded.get(i) + " vs " + expectedRows.get(i));
            }
        }
    }

    private static List<FileRow> filesToRows(Map<String, FileMetaData> files) {
        List<FileRow> rows = new ArrayList<>(files.size());
        for (Map.Entry<String, FileMetaData> e : files.entrySet()) {
            rows.add(new FileRow(e.getKey(), e.getValue().length(),
                    HexFormat.of().toHexDigits(e.getValue().checksum())));
        }
        return rows;
    }

    private static void writeFilesTsv(Path file, List<FileRow> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# path\tlength\tchecksum_hex16\n");
        for (FileRow r : rows) {
            sb.append(r.path()).append('\t').append(r.length()).append('\t')
                    .append(r.checksumHex()).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<FileRow> readFilesTsv(Path file) throws IOException {
        List<FileRow> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 3) throw new IOException("malformed row: " + line);
                rows.add(new FileRow(cols[0], Long.parseLong(cols[1]), cols[2]));
            }
        }
        return rows;
    }

    /** Single TSV row: path, length, checksum-as-hex16. */
    public record FileRow(String path, long length, String checksumHex) {
        @Override public String toString() {
            return String.format(Locale.ROOT, "%s@%d#%s", path, length, checksumHex);
        }
    }
}
