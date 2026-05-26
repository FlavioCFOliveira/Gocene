package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.util.BytesRef;
import org.apache.lucene.util.IntsRefBuilder;
import org.apache.lucene.util.fst.FST;
import org.apache.lucene.util.fst.FSTCompiler;
import org.apache.lucene.util.fst.PositiveIntOutputs;
import org.apache.lucene.util.fst.Util;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;

/**
 * FST blob: a deterministic, standalone FST file written via
 * {@link FSTCompiler} + {@link FST#save(Path)}.
 *
 * <p>Independent of {@link IndexCorpusScenario}: the output is a single file
 * {@code fst.bin} containing a {@code (BYTE1, PositiveIntOutputs)} FST built
 * from a sorted list of (input, output) entries derived from the seed.
 */
public final class FstBlobScenario implements CorpusScenario {

    public static final String FILE_NAME = "fst.bin";
    private static final int ENTRY_COUNT = 32;

    @Override
    public String name() {
        return "fst-blob";
    }

    @Override
    public String description() {
        return "FST blob (org.apache.lucene.util.fst): standalone fst.bin";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        Path file = target.resolve(FILE_NAME);

        Map<String, Long> entries = entries(seed);
        PositiveIntOutputs outputs = PositiveIntOutputs.getSingleton();
        FSTCompiler<Long> compiler = new FSTCompiler.Builder<>(FST.INPUT_TYPE.BYTE1, outputs).build();
        IntsRefBuilder ints = new IntsRefBuilder();
        for (Map.Entry<String, Long> e : entries.entrySet()) {
            BytesRef key = new BytesRef(e.getKey().getBytes(StandardCharsets.UTF_8));
            Util.toIntsRef(key, ints);
            compiler.add(ints.get(), e.getValue());
        }
        FST.FSTMetadata<Long> metadata = compiler.compile();
        FST<Long> fst = FST.fromFSTReader(metadata, compiler.getFSTReader());
        if (fst == null) {
            throw new IOException("fst-blob: empty FST (no entries)");
        }
        fst.save(file);
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path file = source.resolve(FILE_NAME);
        if (!Files.isRegularFile(file)) {
            throw new IOException("fst-blob: missing " + file);
        }
        PositiveIntOutputs outputs = PositiveIntOutputs.getSingleton();
        FST<Long> fst = FST.read(file, outputs);
        Map<String, Long> expected = entries(seed);
        IntsRefBuilder ints = new IntsRefBuilder();
        for (Map.Entry<String, Long> e : expected.entrySet()) {
            BytesRef key = new BytesRef(e.getKey().getBytes(StandardCharsets.UTF_8));
            Util.toIntsRef(key, ints);
            Long got = Util.get(fst, key);
            if (got == null || !got.equals(e.getValue())) {
                throw new IOException("fst-blob: lookup mismatch for '" + e.getKey()
                        + "' expected=" + e.getValue() + " got=" + got);
            }
        }
    }

    /** Deterministic sorted (input, output) entries derived from {@code seed}. */
    private static Map<String, Long> entries(long seed) {
        // Generate ENTRY_COUNT 4-char ASCII keys, sorted by natural byte order
        // (TreeMap). Outputs are derived from the seed via a simple mix.
        List<String> keys = new ArrayList<>(ENTRY_COUNT);
        for (int i = 0; i < ENTRY_COUNT; i++) {
            long mix = (seed * 0x9E3779B97F4A7C15L) ^ ((long) i * 0xBF58476D1CE4E5B9L);
            char a = (char) ('a' + (int) ((mix >>> 0) & 0x0F));
            char b = (char) ('a' + (int) ((mix >>> 8) & 0x0F));
            char c = (char) ('a' + (int) ((mix >>> 16) & 0x0F));
            char d = (char) ('a' + (int) ((mix >>> 24) & 0x0F));
            keys.add("" + a + b + c + d);
        }
        // Deduplicate while preserving determinism.
        Collections.sort(keys);
        Map<String, Long> out = new TreeMap<>();
        int idx = 0;
        for (String k : keys) {
            if (out.containsKey(k)) continue;
            // PositiveIntOutputs requires NO_OUTPUT (=0) to be reserved, so
            // always emit a strictly positive value.
            out.put(k, (long) (idx + 1));
            idx++;
        }
        return out;
    }
}
