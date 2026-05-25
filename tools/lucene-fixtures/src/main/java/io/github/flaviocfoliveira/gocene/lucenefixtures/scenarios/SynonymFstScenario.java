package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.synonym.SynonymMap;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.store.ByteBuffersDataInput;
import org.apache.lucene.store.ByteBuffersDataOutput;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexOutput;
import org.apache.lucene.util.BytesRef;
import org.apache.lucene.util.CharsRefBuilder;
import org.apache.lucene.util.IntsRefBuilder;
import org.apache.lucene.util.fst.ByteSequenceOutputs;
import org.apache.lucene.util.fst.FST;
import org.apache.lucene.util.fst.Util;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

/**
 * Synonym-FST scenario (Sprint 114 T10 / rmp 4618). Addresses audit row
 * "Synonym FST blob (SolrSynonymParser output)" / SynonymMap, gap_notes
 * "No round-trip test against Lucene-compiled synonym maps; format not
 * yet verified." Produces {@code synonym.fst}: CodecUtil header
 * ("GoceneSynonymFst", v0, idFromSeed) + FST meta-and-body (saved via
 * Lucene's FST.save into a single IndexOutput) + CodecUtil footer.
 * Frames manually because SynonymMap has no CodecUtil-aware writer.
 */
public final class SynonymFstScenario implements CorpusScenario {

    public static final String NAME = "synonym-fst";
    public static final String CODEC = "GoceneSynonymFst";
    public static final int VERSION = 0;
    public static final String FILE_NAME = "synonym.fst";
    public static final int PAIR_COUNT = 10;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Synonym FST blob: SynonymMap.Builder output framed by CodecUtil.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        SynonymMap map = buildSynonymMap(seed);
        FST<BytesRef> fst = map.fst;
        if (fst == null) {
            throw new IOException(NAME + ": empty FST (no entries)");
        }
        try (FSDirectory dir = FSDirectory.open(target);
             IndexOutput out = dir.createOutput(FILE_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, Determinism.idBytes(seed), "");
            // Buffer FST meta+body so we can prefix each segment with a vInt length.
            ByteBuffersDataOutput metaBuf = new ByteBuffersDataOutput();
            ByteBuffersDataOutput bodyBuf = new ByteBuffersDataOutput();
            fst.save(metaBuf, bodyBuf);
            byte[] metaBytes = metaBuf.toArrayCopy();
            byte[] bodyBytes = bodyBuf.toArrayCopy();
            out.writeVInt(metaBytes.length);
            out.writeBytes(metaBytes, 0, metaBytes.length);
            out.writeVInt(bodyBytes.length);
            out.writeBytes(bodyBytes, 0, bodyBytes.length);
            CodecUtil.writeFooter(out);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        try (FSDirectory dir = FSDirectory.open(source);
             ChecksumIndexInput cin = dir.openChecksumInput(FILE_NAME)) {
            CodecUtil.checkIndexHeader(cin, CODEC, VERSION, VERSION, Determinism.idBytes(seed), "");
            byte[] metaBytes = new byte[cin.readVInt()];
            cin.readBytes(metaBytes, 0, metaBytes.length);
            byte[] bodyBytes = new byte[cin.readVInt()];
            cin.readBytes(bodyBytes, 0, bodyBytes.length);
            CodecUtil.checkFooter(cin);
            ByteBuffersDataInput metaIn = new ByteBuffersDataInput(
                    java.util.List.of(java.nio.ByteBuffer.wrap(metaBytes)));
            ByteBuffersDataInput bodyIn = new ByteBuffersDataInput(
                    java.util.List.of(java.nio.ByteBuffer.wrap(bodyBytes)));
            FST<BytesRef> fst = new FST<>(FST.readMetadata(metaIn, ByteSequenceOutputs.getSingleton()), bodyIn);
            IntsRefBuilder scratch = new IntsRefBuilder();
            CharsRefBuilder chars = new CharsRefBuilder();
            for (String[] pair : seededPairs(seed)) {
                chars.clear();
                chars.append(pair[0]);
                if (Util.get(fst, Util.toUTF32(chars.get(), scratch)) == null) {
                    throw new IOException(NAME + ": missing FST output for input '" + pair[0] + "'");
                }
            }
        }
    }

    /**
     * Builds a deterministic {@link SynonymMap}: per i in 0..PAIR_COUNT-1
     * adds "word<i><seedHex>" -> "syn<i><seedHex>a" and "...b" in fixed
     * order so the per-input ords list (folded into FST output bytes by
     * Builder.build) is invariant for a given seed.
     */
    public static SynonymMap buildSynonymMap(long seed) throws IOException {
        SynonymMap.Builder builder = new SynonymMap.Builder(true);
        CharsRefBuilder in = new CharsRefBuilder();
        CharsRefBuilder out = new CharsRefBuilder();
        List<String[]> pairs = seededPairs(seed);
        for (String[] pair : pairs) {
            in.clear();
            in.append(pair[0]);
            out.clear();
            out.append(pair[1]);
            builder.add(in.toCharsRef(), out.toCharsRef(), true);
        }
        return builder.build();
    }

    /** Deterministic (input, output) pairs derived from {@code seed}. */
    public static List<String[]> seededPairs(long seed) {
        String seedTag = String.format("%08x", seed & 0xFFFFFFFFL);
        List<String[]> pairs = new ArrayList<>(PAIR_COUNT * 2);
        for (int i = 0; i < PAIR_COUNT; i++) {
            pairs.add(new String[]{"word" + i + seedTag, "syn" + i + seedTag + "a"});
            pairs.add(new String[]{"word" + i + seedTag, "syn" + i + seedTag + "b"});
        }
        return pairs;
    }
}
