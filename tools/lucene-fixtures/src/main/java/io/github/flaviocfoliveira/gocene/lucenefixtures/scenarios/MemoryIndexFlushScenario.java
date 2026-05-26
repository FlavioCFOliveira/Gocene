package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.TokenStream;
import org.apache.lucene.analysis.core.KeywordAnalyzer;
import org.apache.lucene.analysis.tokenattributes.CharTermAttribute;
import org.apache.lucene.analysis.tokenattributes.OffsetAttribute;
import org.apache.lucene.analysis.tokenattributes.PayloadAttribute;
import org.apache.lucene.analysis.tokenattributes.PositionIncrementAttribute;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.index.CodecReader;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.LeafReader;
import org.apache.lucene.index.LeafReaderContext;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.PostingsEnum;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.SlowCodecReaderWrapper;
import org.apache.lucene.index.Terms;
import org.apache.lucene.index.TermsEnum;
import org.apache.lucene.index.memory.MemoryIndex;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.util.BytesRef;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T25 (rmp 4633): {@code memory-index-flush}. Addresses the
 * memory audit row (verbatim from {@code docs/compat-coverage.tsv}):
 * "No persisted binary artefact; gap is the absence of byte-for-byte parity
 * tests vs Lucene MemoryIndex internal layout (where applicable to merges)."
 *
 * <p>Builds a {@link MemoryIndex} from a fixed token stream derived from
 * {@code seed} (~10 tokens, each with a 4-byte LE payload and explicit
 * offsets), wraps the in-memory leaf as a {@link CodecReader} via
 * {@link SlowCodecReaderWrapper}, then flushes it into a Directory-backed
 * {@link IndexWriter} using {@code addIndexes(CodecReader...)}. A
 * {@code forceMerge(1)} call collapses the result to a single segment.
 *
 * <p>useCompoundFile=false; NoMergePolicy with the explicit {@code forceMerge}
 * is what produces the single segment without the segment-id wobble compound
 * files introduce. {@link #verify} reopens the directory and asserts the
 * single doc plus every token term is present.
 */
public final class MemoryIndexFlushScenario implements CorpusScenario {

    public static final String NAME = "memory-index-flush";
    public static final String FIELD = "body";
    public static final int TOKEN_COUNT = 10;
    public static final int PAYLOAD_LEN = 4;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "MemoryIndex flushed via addIndexes(CodecReader) into a Directory-backed IndexWriter (single segment, force-merged).";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        MemoryIndex mem = buildMemoryIndex(seed);
        IndexSearcher searcher = mem.createSearcher();
        LeafReaderContext ctx = searcher.getIndexReader().leaves().get(0);
        LeafReader leaf = ctx.reader();
        CodecReader cr = SlowCodecReaderWrapper.wrap(leaf);

        // Directory-backed IndexWriter: useCompoundFile=false, NoMergePolicy
        // so only the explicit forceMerge(1) below produces the single segment.
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new KeywordAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                writer.addIndexes(cr);
                writer.forceMerge(1);
                writer.commit();
            }
        }
        // Sanity check: reopen and assert the doc/token presence (same shape as verify()).
        assertReopen(target, seed);
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        assertReopen(source, seed);
    }

    /** Build the deterministic MemoryIndex used by both generate() and verify(). */
    private static MemoryIndex buildMemoryIndex(long seed) {
        // MemoryIndex(storeOffsets=true, storePayloads=true): the constructor pins
        // the IndexOptions to DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS and
        // honours PayloadAttribute on the supplied TokenStream.
        MemoryIndex mem = new MemoryIndex(true, true);
        mem.addField(FIELD, new SeededTokenStream(seed));
        return mem;
    }

    /** Reopens the directory and verifies every expected token term is indexed. */
    private static void assertReopen(Path dir, long seed) throws IOException {
        try (FSDirectory fs = FSDirectory.open(dir);
             IndexReader reader = DirectoryReader.open(fs)) {
            if (reader.numDocs() != 1) {
                throw new IOException(NAME + ": expected exactly 1 doc, got " + reader.numDocs());
            }
            if (reader.leaves().size() != 1) {
                throw new IOException(NAME + ": expected exactly 1 leaf, got " + reader.leaves().size());
            }
            LeafReader leaf = reader.leaves().get(0).reader();
            Terms terms = leaf.terms(FIELD);
            if (terms == null) {
                throw new IOException(NAME + ": no terms for field '" + FIELD + "'");
            }
            TermsEnum it = terms.iterator();
            int seen = 0;
            BytesRef term;
            while ((term = it.next()) != null) {
                String t = term.utf8ToString();
                if (!t.startsWith("tok")) {
                    throw new IOException(NAME + ": unexpected term '" + t + "'");
                }
                int idx = Integer.parseInt(t.substring(3));
                PostingsEnum posts = it.postings(null, PostingsEnum.PAYLOADS | PostingsEnum.OFFSETS);
                if (posts.nextDoc() == PostingsEnum.NO_MORE_DOCS) {
                    throw new IOException(NAME + ": term '" + t + "' has no docs");
                }
                posts.nextPosition();
                BytesRef payload = posts.getPayload();
                byte[] want = expectedPayload(seed, idx);
                if (payload == null || payload.length != PAYLOAD_LEN) {
                    throw new IOException(NAME + ": term '" + t + "' payload length mismatch");
                }
                for (int b = 0; b < PAYLOAD_LEN; b++) {
                    if (payload.bytes[payload.offset + b] != want[b]) {
                        throw new IOException(NAME + ": term '" + t + "' payload[" + b + "] mismatch");
                    }
                }
                seen++;
            }
            if (seen != TOKEN_COUNT) {
                throw new IOException(NAME + ": expected " + TOKEN_COUNT + " terms, got " + seen);
            }
        }
    }

    /** 4B LE word = (int)(seed XOR (idx * 0x9E3779B1L)). */
    public static byte[] expectedPayload(long seed, int idx) {
        int word = (int) (seed ^ ((long) idx * 0x9E3779B1L));
        byte[] out = new byte[PAYLOAD_LEN];
        out[0] = (byte) (word & 0xFF);
        out[1] = (byte) ((word >>> 8) & 0xFF);
        out[2] = (byte) ((word >>> 16) & 0xFF);
        out[3] = (byte) ((word >>> 24) & 0xFF);
        return out;
    }

    /** Deterministic TokenStream emitting TOKEN_COUNT tokens with offsets + payloads. */
    private static final class SeededTokenStream extends TokenStream {
        private final CharTermAttribute termAtt = addAttribute(CharTermAttribute.class);
        private final OffsetAttribute offAtt = addAttribute(OffsetAttribute.class);
        private final PayloadAttribute payAtt = addAttribute(PayloadAttribute.class);
        private final PositionIncrementAttribute posAtt = addAttribute(PositionIncrementAttribute.class);
        private final long seed;
        private int idx;
        private int charCursor;

        SeededTokenStream(long seed) {
            this.seed = seed;
        }

        @Override
        public boolean incrementToken() {
            if (idx >= TOKEN_COUNT) return false;
            clearAttributes();
            String tok = "tok" + idx;
            termAtt.setEmpty().append(tok);
            int start = charCursor;
            int end = start + tok.length();
            offAtt.setOffset(start, end);
            charCursor = end + 1; // single space between tokens
            posAtt.setPositionIncrement(idx == 0 ? 1 : 1);
            payAtt.setPayload(new BytesRef(expectedPayload(seed, idx)));
            idx++;
            return true;
        }

        @Override
        public void reset() {
            idx = 0;
            charCursor = 0;
        }
    }
}
