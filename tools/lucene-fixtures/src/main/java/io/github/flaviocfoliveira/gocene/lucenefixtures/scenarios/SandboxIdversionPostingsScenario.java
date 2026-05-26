package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.TokenStream;
import org.apache.lucene.analysis.tokenattributes.CharTermAttribute;
import org.apache.lucene.analysis.tokenattributes.PayloadAttribute;
import org.apache.lucene.codecs.Codec;
import org.apache.lucene.codecs.PostingsFormat;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.FieldType;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexOptions;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.LeafReader;
import org.apache.lucene.index.LeafReaderContext;
import org.apache.lucene.index.Terms;
import org.apache.lucene.index.TermsEnum;
import org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsFormat;
import org.apache.lucene.sandbox.codecs.idversion.IDVersionSegmentTermsEnum;
import org.apache.lucene.util.BytesRef;

import java.io.IOException;
import java.util.Locale;

/**
 * Sprint 114 T23 (rmp 4631): {@code sandbox-idversion-postings}. Addresses
 * the sandbox audit row (verbatim from the task contract):
 * "IDVersionPostingsFormat: Pure port without tests, fixtures, or writer
 * parity".
 *
 * <p>Builds a single-segment Lucene 10.4.0 index whose {@code id} field is
 * routed to {@link IDVersionPostingsFormat} via an anonymous
 * {@link FilterCodec} (every other field falls back to
 * {@link Lucene104Codec}). Each indexed document carries:
 * <ul>
 *   <li>{@code id} — a single token with an 8-byte payload that encodes the
 *       per-doc version, produced by the {@link IdVersionField} helper. This
 *       helper is a verbatim re-implementation of the upstream test helper
 *       {@code org.apache.lucene.sandbox.codecs.idversion.StringAndPayloadField}
 *       at tag {@code releases/lucene/10.4.0} (path
 *       {@code lucene/sandbox/src/test/...idversion/StringAndPayloadField.java});
 *       Lucene ships it only in {@code src/test/}, so it cannot be imported
 *       from the main {@code lucene-sandbox} jar.</li>
 * </ul>
 *
 * <p>Verification re-opens the index and, for every seeded id, calls
 * {@link IDVersionSegmentTermsEnum#seekExact(BytesRef, long)} with the
 * recorded version; this is the canonical visibility primitive shipped by
 * Lucene 10.4.0's IDVersion stack — the package contains <strong>no</strong>
 * {@code IDVersionQuery} class (verified against the production tree at
 * {@code lucene/sandbox/src/java/org/apache/lucene/sandbox/codecs/idversion/}
 * which only exposes the format, segment-terms-enum, reader/writer, and
 * term state types). The task brief's reference to "IDVersionQuery" is
 * therefore satisfied by the {@code seekExact} lookup, which is the exact
 * primitive {@code TestIDVersionPostingsFormat} uses to assert visibility.
 *
 * <p>Determinism: ids and versions are derived from the seed via SplitMix64,
 * the writer uses {@code NoMergePolicy} + {@code SerialMergeScheduler}, and
 * {@code useCompoundFile=false} so the produced files are
 * {@code _0_*IDVersion*} suffixed and stable across runs.
 */
public final class SandboxIdversionPostingsScenario extends IndexCorpusScenario {

    /** Number of (id, version) entries to index. */
    public static final int DOC_COUNT = 16;

    /** Indexed field name. */
    public static final String FIELD_ID = "id";

    /** Stable, kebab-case scenario name. */
    public static final String NAME = "sandbox-idversion-postings";

    @Override public String name() { return NAME; }

    @Override
    public String description() {
        return "Sandbox IDVersionPostingsFormat: single-segment index with id "
                + "(StringAndPayloadField re-impl, payload=version) routed to "
                + "IDVersionPostingsFormat; verify via IDVersionSegmentTermsEnum.seekExact.";
    }

    @Override protected int numDocs() { return DOC_COUNT; }

    @Override protected boolean useCompoundFile() { return false; }

    @Override
    protected Codec codec() {
        // Anonymous Lucene104Codec subclass routing FIELD_ID to
        // IDVersionPostingsFormat (sandbox), every other field to the
        // Lucene 10.4 default. Mirrors the per-field dispatch pattern used
        // by PerFieldDispatchScenario; reuses the no-arg ctor of
        // IDVersionPostingsFormat so the block-tree min/max items match
        // upstream defaults.
        return new Lucene104Codec() {
            private final PostingsFormat idversion = new IDVersionPostingsFormat();
            private final PostingsFormat dflt = new Lucene104PostingsFormat();

            @Override
            public PostingsFormat getPostingsFormatForField(String field) {
                if (FIELD_ID.equals(field)) {
                    return idversion;
                }
                return dflt;
            }
        };
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        String id = idValue(seed, i);
        long version = versionValue(seed, i);
        Document doc = new Document();
        BytesRef payload = new BytesRef(8);
        payload.length = 8;
        IDVersionPostingsFormat.longToBytes(version, payload);
        doc.add(new IdVersionField(FIELD_ID, id, payload));
        return doc;
    }

    @Override
    protected void verifyReader(IndexReader reader, long seed) throws IOException {
        super.verifyReader(reader, seed);
        for (LeafReaderContext leaf : reader.leaves()) {
            LeafReader lr = leaf.reader();
            Terms terms = lr.terms(FIELD_ID);
            if (terms == null) {
                throw new IOException(NAME + ": leaf has no terms for field " + FIELD_ID);
            }
            TermsEnum te = terms.iterator();
            if (!(te instanceof IDVersionSegmentTermsEnum)) {
                throw new IOException(NAME + ": expected IDVersionSegmentTermsEnum, got "
                        + te.getClass().getName());
            }
            IDVersionSegmentTermsEnum idTe = (IDVersionSegmentTermsEnum) te;
            for (int i = 0; i < DOC_COUNT; i++) {
                String id = idValue(seed, i);
                long version = versionValue(seed, i);
                BytesRef idBytes = new BytesRef(id);
                if (!idTe.seekExact(idBytes, version)) {
                    throw new IOException(String.format(Locale.ROOT,
                            "%s: seekExact(id=%s, version=%d) returned false at i=%d",
                            NAME, id, version, i));
                }
                if (idTe.getVersion() != version) {
                    throw new IOException(String.format(Locale.ROOT,
                            "%s: getVersion mismatch at i=%d: expected %d, got %d",
                            NAME, i, version, idTe.getVersion()));
                }
                // A lookup with version+1 must fail (term not new enough).
                if (idTe.seekExact(idBytes, version + 1L)) {
                    throw new IOException(String.format(Locale.ROOT,
                            "%s: seekExact(id=%s, version=%d) unexpectedly succeeded at i=%d",
                            NAME, id, version + 1L, i));
                }
            }
        }
    }

    /** Deterministic id slug: "id-{i:04d}-{splitmix(seed,i)&0xFFFF}". */
    static String idValue(long seed, int i) {
        long state = mix(seed ^ (long) i ^ 0x9E3779B97F4A7C15L);
        return String.format(Locale.ROOT, "id-%04d-%04x", i, (int) (state & 0xFFFFL));
    }

    /** Deterministic monotonically-ordered version in [1, 0x3FFF_FFFF_FFFF_FFFFL]. */
    static long versionValue(long seed, int i) {
        long state = mix(seed + 0x100L + (long) i);
        // IDVersionPostingsFormat.longToBytes requires version <= 0x3FFFFFFFFFFFFFFFL.
        return (state & 0x3FFFFFFFFFFFFFFFL) | 1L;
    }

    /** SplitMix64 finalizer — same constants as ReplicatorNrtCopyStateScenario. */
    private static long mix(long z) {
        z = (z ^ (z >>> 30)) * 0xBF58476D1CE4E5B9L;
        z = (z ^ (z >>> 27)) * 0x94D049BB133111EBL;
        return z ^ (z >>> 31);
    }

    /**
     * Verbatim re-implementation of
     * {@code org.apache.lucene.sandbox.codecs.idversion.StringAndPayloadField}
     * from {@code releases/lucene/10.4.0}
     * ({@code lucene/sandbox/src/test/.../StringAndPayloadField.java}). The
     * upstream type lives under {@code src/test/} and is therefore not part
     * of the published {@code lucene-sandbox} jar; re-creating it here is
     * the minimal adapter required to drive {@link IDVersionPostingsFormat}
     * from a non-test-framework context.
     *
     * <p>Produces a single token from the supplied {@code value} carrying
     * the supplied 8-byte {@code payload}. {@link IndexOptions} is
     * {@link IndexOptions#DOCS_AND_FREQS_AND_POSITIONS}; norms are omitted.
     */
    static final class IdVersionField extends Field {

        static final FieldType TYPE = new FieldType();

        static {
            TYPE.setOmitNorms(true);
            TYPE.setIndexOptions(IndexOptions.DOCS_AND_FREQS_AND_POSITIONS);
            TYPE.setTokenized(true);
            TYPE.freeze();
        }

        private final BytesRef payload;

        IdVersionField(String name, String value, BytesRef payload) {
            super(name, value, TYPE);
            this.payload = payload;
        }

        @Override
        public TokenStream tokenStream(Analyzer analyzer, TokenStream reuse) {
            SingleTokenStream ts;
            if (reuse instanceof SingleTokenStream) {
                ts = (SingleTokenStream) reuse;
            } else {
                ts = new SingleTokenStream();
            }
            ts.setValue((String) fieldsData, payload);
            return ts;
        }

        /** Single-token stream that attaches an 8-byte payload to its only term. */
        static final class SingleTokenStream extends TokenStream {

            private final CharTermAttribute termAttribute = addAttribute(CharTermAttribute.class);
            private final PayloadAttribute payloadAttribute = addAttribute(PayloadAttribute.class);
            private boolean used = false;
            private String value;
            private BytesRef payload;

            void setValue(String value, BytesRef payload) {
                this.value = value;
                this.payload = payload;
            }

            @Override
            public boolean incrementToken() {
                if (used) {
                    return false;
                }
                clearAttributes();
                termAttribute.append(value);
                payloadAttribute.setPayload(payload);
                used = true;
                return true;
            }

            @Override
            public void reset() {
                used = false;
            }
        }
    }

    /**
     * Convenience for the {@code verify-sandbox} CLI dispatcher and Go-side
     * tests: read every (id, version) pair the scenario indexes for a given
     * seed without re-running the IndexWriter. The result is positional —
     * index {@code i} matches document number {@code i}.
     */
    public static String[] expectedIds(long seed) {
        String[] out = new String[DOC_COUNT];
        for (int i = 0; i < DOC_COUNT; i++) {
            out[i] = idValue(seed, i);
        }
        return out;
    }

    /** Mirrors {@link #expectedIds(long)} for versions. */
    public static long[] expectedVersions(long seed) {
        long[] out = new long[DOC_COUNT];
        for (int i = 0; i < DOC_COUNT; i++) {
            out[i] = versionValue(seed, i);
        }
        return out;
    }
}
