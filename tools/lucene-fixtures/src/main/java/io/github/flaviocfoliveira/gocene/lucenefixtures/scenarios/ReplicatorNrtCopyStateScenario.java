package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.replicator.nrt.CopyState;
import org.apache.lucene.replicator.nrt.FileMetaData;
import org.apache.lucene.store.ByteArrayDataInput;
import org.apache.lucene.store.ByteArrayDataOutput;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.DataInput;
import org.apache.lucene.store.DataOutput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexOutput;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.Locale;
import java.util.Map;
import java.util.Set;

/**
 * Sprint 114 T19 (rmp 4627): {@code replicator-nrt-copystate}. Addresses the
 * NRT-replicator audit row (verbatim from docs/compat-coverage.tsv):
 * "No interop frame captured from a Java Lucene replicator peer.".
 *
 * <p>Emits a single {@code nrt-copystate.bin} file, framed by
 * {@link CodecUtil}, whose payload is the canonical NRT {@link CopyState} wire
 * layout copied verbatim from Lucene 10.4.0's
 * {@code org.apache.lucene.replicator.nrt.SimplePrimaryNode#writeCopyState}
 * and {@code TestSimpleServer#writeFilesMetaData}. The layout is:
 *
 * <pre>
 *   IndexHeader( codec="GoceneReplicatorNrtCopyState", v0, id=16B(seed), suffix="" )
 *   // -- begin Lucene SimplePrimaryNode.writeCopyState payload --
 *   vInt    infosBytes.length
 *   bytes   infosBytes
 *   vLong   gen
 *   vLong   version
 *   // writeFilesMetaData:
 *   vInt    files.size()
 *   for each (name, FileMetaData) entry (LinkedHashMap insertion order):
 *     String name                      // out.writeString
 *     vLong  length
 *     vLong  checksum
 *     vInt   header.length
 *     bytes  header
 *     vInt   footer.length
 *     bytes  footer
 *   vInt    completedMergeFiles.size()
 *   for each: String fileName
 *   vLong   primaryGen
 *   // -- end SimplePrimaryNode.writeCopyState payload --
 *   Footer  ( CodecUtil )
 * </pre>
 *
 * <p>The {@link CopyState} fields are derived deterministically from the
 * seed; the {@code Map<String,FileMetaData>} is a {@link LinkedHashMap} so
 * the insertion order — and therefore the resulting byte stream — is
 * identical across runs at the same seed (Lucene's
 * {@code SimplePrimaryNode#writeCopyState} preserves the iteration order of
 * the supplied map). {@code completedMergeFiles} is a {@link LinkedHashSet}
 * for the same reason.
 *
 * <p>The {@code verify} entrypoint deserialises the file with the inverse
 * routines from {@code TestSimpleServer#readFilesMetaData} /
 * {@code readCopyState}, then asserts every {@link FileMetaData} round-trips
 * byte-for-byte against the expectation.
 */
public final class ReplicatorNrtCopyStateScenario implements CorpusScenario {

    public static final String NAME = "replicator-nrt-copystate";
    public static final String CODEC = "GoceneReplicatorNrtCopyState";
    public static final int VERSION = 0;
    public static final String FILE_NAME = "nrt-copystate.bin";

    /** Number of FileMetaData entries packed into the CopyState payload. */
    public static final int FILE_COUNT = 3;
    /** Number of names in completedMergeFiles. */
    public static final int COMPLETED_MERGE_FILE_COUNT = 2;

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "NRT replicator CopyState wire frame (SimplePrimaryNode.writeCopyState layout): "
                + "infosBytes + gen + version + FileMetaData[] + completedMergeFiles + primaryGen, "
                + "framed by CodecUtil.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        CopyState expected = buildCopyState(seed);
        try (FSDirectory dir = FSDirectory.open(target);
             IndexOutput out = dir.createOutput(FILE_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, Determinism.idBytes(seed), "");
            writeCopyState(expected, out);
            CodecUtil.writeFooter(out);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        CopyState expected = buildCopyState(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             ChecksumIndexInput in = dir.openChecksumInput(FILE_NAME)) {
            CodecUtil.checkIndexHeader(in, CODEC, VERSION, VERSION, Determinism.idBytes(seed), "");
            CopyState decoded = readCopyState(in);
            assertEqualCopyState(expected, decoded);
            CodecUtil.checkFooter(in);
        }
    }

    /**
     * Mirrors {@code SimplePrimaryNode#writeCopyState} from Lucene 10.4.0
     * line for line. Exposed package-public so the Go side can reach the
     * same expectation via a thin helper if needed.
     */
    public static void writeCopyState(CopyState state, DataOutput out) throws IOException {
        out.writeVInt(state.infosBytes().length);
        out.writeBytes(state.infosBytes(), 0, state.infosBytes().length);
        out.writeVLong(state.gen());
        out.writeVLong(state.version());
        writeFilesMetaData(out, state.files());
        out.writeVInt(state.completedMergeFiles().size());
        for (String name : state.completedMergeFiles()) {
            out.writeString(name);
        }
        out.writeVLong(state.primaryGen());
    }

    /** Mirrors {@code TestSimpleServer#writeFilesMetaData} line for line. */
    public static void writeFilesMetaData(DataOutput out, Map<String, FileMetaData> files)
            throws IOException {
        out.writeVInt(files.size());
        for (Map.Entry<String, FileMetaData> ent : files.entrySet()) {
            out.writeString(ent.getKey());
            FileMetaData fmd = ent.getValue();
            out.writeVLong(fmd.length());
            out.writeVLong(fmd.checksum());
            out.writeVInt(fmd.header().length);
            out.writeBytes(fmd.header(), 0, fmd.header().length);
            out.writeVInt(fmd.footer().length);
            out.writeBytes(fmd.footer(), 0, fmd.footer().length);
        }
    }

    /** Mirrors {@code TestSimpleServer#readCopyState}. */
    public static CopyState readCopyState(DataInput in) throws IOException {
        byte[] infosBytes = new byte[in.readVInt()];
        in.readBytes(infosBytes, 0, infosBytes.length);
        long gen = in.readVLong();
        long version = in.readVLong();
        Map<String, FileMetaData> files = readFilesMetaData(in);
        int count = in.readVInt();
        Set<String> completedMergeFiles = new LinkedHashSet<>();
        for (int i = 0; i < count; i++) {
            completedMergeFiles.add(in.readString());
        }
        long primaryGen = in.readVLong();
        return new CopyState(files, version, gen, infosBytes, completedMergeFiles, primaryGen, null);
    }

    /** Mirrors {@code TestSimpleServer#readFilesMetaData}, but uses a
     *  LinkedHashMap to preserve insertion order (Lucene's reader uses a
     *  plain HashMap because the caller never re-emits the map; the
     *  scenario verifier compares against an ordered expectation). */
    public static Map<String, FileMetaData> readFilesMetaData(DataInput in) throws IOException {
        int fileCount = in.readVInt();
        Map<String, FileMetaData> files = new LinkedHashMap<>();
        for (int i = 0; i < fileCount; i++) {
            String fileName = in.readString();
            long length = in.readVLong();
            long checksum = in.readVLong();
            byte[] header = new byte[in.readVInt()];
            in.readBytes(header, 0, header.length);
            byte[] footer = new byte[in.readVInt()];
            in.readBytes(footer, 0, footer.length);
            files.put(fileName, new FileMetaData(header, footer, length, checksum));
        }
        return files;
    }

    /**
     * Builds the canonical CopyState for {@code seed}. Three FileMetaData
     * entries (segments_N, _0.cfe, _0.cfs), two completedMergeFiles, fixed
     * gen/version/primaryGen offsets from the seed. The {@code infosBytes}
     * payload is a stable seed-derived byte sequence — its content is
     * opaque on the wire (Lucene treats it as raw bytes).
     */
    public static CopyState buildCopyState(long seed) {
        Map<String, FileMetaData> files = new LinkedHashMap<>();
        files.put("segments_1", fileMetaData(seed, 1, 64));
        files.put("_0.cfe", fileMetaData(seed, 2, 128));
        files.put("_0.cfs", fileMetaData(seed, 3, 4096));
        Set<String> completed = new LinkedHashSet<>();
        completed.add("_0.cfs");
        completed.add("_0.cfe");
        byte[] infosBytes = seedBytes(seed, "infos", 96);
        long gen = seed | 0x10L;
        long version = seed | 0x20L;
        long primaryGen = seed | 0x40L;
        return new CopyState(files, version, gen, infosBytes, completed, primaryGen, null);
    }

    private static FileMetaData fileMetaData(long seed, int idx, long length) {
        byte[] header = seedBytes(seed, "h" + idx, 16);
        byte[] footer = seedBytes(seed, "f" + idx, 16);
        long checksum = (seed ^ (0xA5A5A5A5L * idx)) & 0x7FFFFFFFFFFFFFFFL;
        return new FileMetaData(header, footer, length, checksum);
    }

    /** Deterministic pseudo-random byte sequence: SplitMix64 over (seed, salt). */
    private static byte[] seedBytes(long seed, String salt, int len) {
        long state = seed;
        for (int i = 0; i < salt.length(); i++) {
            state ^= salt.charAt(i);
            state = mix(state);
        }
        byte[] out = new byte[len];
        for (int i = 0; i < len; i++) {
            state = mix(state);
            out[i] = (byte) (state & 0xFF);
        }
        return out;
    }

    private static long mix(long z) {
        z = (z ^ (z >>> 30)) * 0xBF58476D1CE4E5B9L;
        z = (z ^ (z >>> 27)) * 0x94D049BB133111EBL;
        return z ^ (z >>> 31);
    }

    /** Verifier helper: compare two CopyState payloads field-by-field. */
    private static void assertEqualCopyState(CopyState exp, CopyState got) throws IOException {
        if (!Arrays.equals(exp.infosBytes(), got.infosBytes())) {
            throw new IOException(NAME + ": infosBytes mismatch (lenExp=" + exp.infosBytes().length
                    + " lenGot=" + got.infosBytes().length + ")");
        }
        if (exp.gen() != got.gen()) {
            throw new IOException(NAME + ": gen mismatch, expected " + exp.gen() + ", got " + got.gen());
        }
        if (exp.version() != got.version()) {
            throw new IOException(NAME + ": version mismatch, expected " + exp.version()
                    + ", got " + got.version());
        }
        if (exp.primaryGen() != got.primaryGen()) {
            throw new IOException(NAME + ": primaryGen mismatch, expected " + exp.primaryGen()
                    + ", got " + got.primaryGen());
        }
        if (exp.files().size() != got.files().size()) {
            throw new IOException(NAME + ": files.size() mismatch, expected " + exp.files().size()
                    + ", got " + got.files().size());
        }
        for (Map.Entry<String, FileMetaData> ent : exp.files().entrySet()) {
            FileMetaData g = got.files().get(ent.getKey());
            if (g == null) {
                throw new IOException(NAME + ": missing file " + ent.getKey());
            }
            FileMetaData e = ent.getValue();
            if (e.length() != g.length() || e.checksum() != g.checksum()) {
                throw new IOException(String.format(Locale.ROOT,
                        "%s: FileMetaData[%s] mismatch: length=%d/%d checksum=%d/%d",
                        NAME, ent.getKey(), e.length(), g.length(), e.checksum(), g.checksum()));
            }
            if (!Arrays.equals(e.header(), g.header()) || !Arrays.equals(e.footer(), g.footer())) {
                throw new IOException(NAME + ": FileMetaData[" + ent.getKey()
                        + "] header/footer mismatch");
            }
        }
        if (!new HashSet<>(exp.completedMergeFiles()).equals(new HashSet<>(got.completedMergeFiles()))) {
            throw new IOException(NAME + ": completedMergeFiles mismatch, expected "
                    + exp.completedMergeFiles() + ", got " + got.completedMergeFiles());
        }
    }

    /** Convenience for callers that want the serialised payload (no CodecUtil framing). */
    public static byte[] encodePayload(CopyState state) throws IOException {
        // 1 KiB scratch is comfortably above the 256-byte upper bound for the
        // canonical batch (3 FileMetaData with 16-byte header+footer each).
        byte[] scratch = new byte[1024];
        ByteArrayDataOutput out = new ByteArrayDataOutput(scratch);
        writeCopyState(state, out);
        return Arrays.copyOf(scratch, out.getPosition());
    }

    /** Convenience for callers that want to decode a payload (no CodecUtil framing). */
    public static CopyState decodePayload(byte[] payload) throws IOException {
        return readCopyState(new ByteArrayDataInput(payload));
    }
}
