package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.TokenFilter;
import org.apache.lucene.analysis.TokenStream;
import org.apache.lucene.analysis.Tokenizer;
import org.apache.lucene.analysis.core.WhitespaceTokenizer;
import org.apache.lucene.analysis.tokenattributes.CharTermAttribute;
import org.apache.lucene.analysis.tokenattributes.PayloadAttribute;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.FieldType;
import org.apache.lucene.document.StringField;
import org.apache.lucene.index.IndexOptions;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.MultiTerms;
import org.apache.lucene.index.PostingsEnum;
import org.apache.lucene.index.Terms;
import org.apache.lucene.index.TermsEnum;
import org.apache.lucene.util.BytesRef;

import java.io.IOException;

/**
 * Token-payload scenario (Sprint 114 T10 / rmp 4618). Addresses audit
 * row "Token payload byte serialisation" / PayloadHelper, gap_notes
 * "No Lucene-side parity test for payload byte layout."
 *
 * <p>Indexes NUM_DOCS x TOKENS_PER_DOC tokens, each carrying a
 * deterministic 4-byte LE payload (seed XOR (docId*31 + tokIdx))
 * persisted into the segment's .pos/.pay files. verify reopens with
 * PostingsEnum.PAYLOADS and asserts every byte.
 */
public final class TokenPayloadScenario extends IndexCorpusScenario {

    public static final String NAME = "token-payload-bytes";
    public static final int NUM_DOCS = 5;
    public static final int TOKENS_PER_DOC = 6;
    public static final String FIELD = "body";
    public static final int PAYLOAD_LEN = 4;

    private static final FieldType PAYLOAD_FIELD_TYPE;

    static {
        PAYLOAD_FIELD_TYPE = new FieldType();
        PAYLOAD_FIELD_TYPE.setStored(false);
        PAYLOAD_FIELD_TYPE.setTokenized(true);
        PAYLOAD_FIELD_TYPE.setIndexOptions(IndexOptions.DOCS_AND_FREQS_AND_POSITIONS);
        PAYLOAD_FIELD_TYPE.freeze();
    }

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Token payload bytes: per-token 4B payload persisted into .pos/.pay.";
    }
    @Override protected int numDocs() { return NUM_DOCS; }
    @Override protected Analyzer analyzer() { return new PayloadAnalyzer(); }

    @Override
    protected Document buildDoc(int i, long seed) {
        // Analyzer pipeline reads the seed via thread-local; refresh per doc.
        setCurrentSeed(seed);
        Document doc = new Document();
        doc.add(new StringField("id", "tp-" + i, Field.Store.YES));
        // Token text shape "tk<doc>_<tok>" is parsed by PayloadFilter so the
        // payload bytes are deterministic without analyzer state.
        StringBuilder text = new StringBuilder(TOKENS_PER_DOC * 8);
        for (int t = 0; t < TOKENS_PER_DOC; t++) {
            if (t > 0) text.append(' ');
            text.append("tk").append(i).append('_').append(t);
        }
        doc.add(new Field(FIELD, text.toString(), PAYLOAD_FIELD_TYPE));
        return doc;
    }

    @Override
    protected void verifyReader(IndexReader reader, long seed) throws IOException {
        super.verifyReader(reader, seed);
        Terms terms = MultiTerms.getTerms(reader, FIELD);
        if (terms == null) throw new IOException(NAME + ": no terms for field '" + FIELD + "'");
        TermsEnum it = terms.iterator();
        BytesRef term;
        int checked = 0;
        while ((term = it.next()) != null) {
            String t = term.utf8ToString();
            int underscore = t.indexOf('_');
            if (!t.startsWith("tk") || underscore < 3) {
                throw new IOException(NAME + ": unexpected term shape '" + t + "'");
            }
            int docId = Integer.parseInt(t.substring(2, underscore));
            int tokIdx = Integer.parseInt(t.substring(underscore + 1));
            PostingsEnum posts = it.postings(null, PostingsEnum.PAYLOADS);
            if (posts.nextDoc() == PostingsEnum.NO_MORE_DOCS) {
                throw new IOException(NAME + ": term '" + t + "' has no docs");
            }
            posts.nextPosition();
            BytesRef payload = posts.getPayload();
            byte[] want = expectedPayload(seed, docId, tokIdx);
            if (payload == null || payload.length != want.length) {
                throw new IOException(NAME + ": term '" + t + "' payload length mismatch (got "
                        + (payload == null ? "null" : payload.length) + " want " + want.length + ")");
            }
            for (int b = 0; b < want.length; b++) {
                if (payload.bytes[payload.offset + b] != want[b]) {
                    throw new IOException(NAME + ": term '" + t + "' payload[" + b + "] mismatch");
                }
            }
            checked++;
        }
        if (checked != NUM_DOCS * TOKENS_PER_DOC) {
            throw new IOException(NAME + ": checked " + checked + " terms, expected "
                    + (NUM_DOCS * TOKENS_PER_DOC));
        }
    }

    /** 4B LE word = (int)(seed XOR (docId*31 + tokIdx)). */
    public static byte[] expectedPayload(long seed, int docId, int tokIdx) {
        int word = (int) (seed ^ ((long) docId * 31L + (long) tokIdx));
        byte[] out = new byte[PAYLOAD_LEN];
        out[0] = (byte) (word & 0xFF);
        out[1] = (byte) ((word >>> 8) & 0xFF);
        out[2] = (byte) ((word >>> 16) & 0xFF);
        out[3] = (byte) ((word >>> 24) & 0xFF);
        return out;
    }

    /** Analyzer = WhitespaceTokenizer + PayloadFilter (seed via thread-local). */
    private static final class PayloadAnalyzer extends Analyzer {
        @Override protected TokenStreamComponents createComponents(String fieldName) {
            Tokenizer src = new WhitespaceTokenizer();
            return new TokenStreamComponents(src, new PayloadFilter(src));
        }
    }

    /** Parses "tk<doc>_<tok>" and attaches the seeded payload via PayloadAttribute. */
    private static final class PayloadFilter extends TokenFilter {
        private final CharTermAttribute termAtt = addAttribute(CharTermAttribute.class);
        private final PayloadAttribute payAtt = addAttribute(PayloadAttribute.class);
        PayloadFilter(TokenStream in) { super(in); }
        @Override public boolean incrementToken() throws IOException {
            if (!input.incrementToken()) return false;
            String t = termAtt.toString();
            int underscore = t.indexOf('_');
            if (!t.startsWith("tk") || underscore < 3) {
                payAtt.setPayload(null);
                return true;
            }
            int docId = Integer.parseInt(t.substring(2, underscore));
            int tokIdx = Integer.parseInt(t.substring(underscore + 1));
            payAtt.setPayload(new BytesRef(expectedPayload(currentSeed(), docId, tokIdx)));
            return true;
        }
    }

    private static final ThreadLocal<Long> CURRENT_SEED = new ThreadLocal<>();

    static void setCurrentSeed(long seed) { CURRENT_SEED.set(seed); }

    static long currentSeed() {
        Long s = CURRENT_SEED.get();
        if (s == null) throw new IllegalStateException("TokenPayloadScenario: seed not initialised");
        return s;
    }
}
