package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.search.suggest.Lookup.LookupResult;
import org.apache.lucene.search.suggest.fst.WFSTCompletionLookup;
import org.apache.lucene.store.ByteBuffersDirectory;
import org.apache.lucene.store.InputStreamDataInput;
import org.apache.lucene.store.OutputStreamDataOutput;

import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

/**
 * Sprint 114 T13 (rmp 4621): {@code wfst-blob}. Addresses suggest audit row
 * (verbatim): "No combined test; no Lucene fixture." (WFSTCompletionLookup).
 * Builds a {@link WFSTCompletionLookup} from the same seeded input set as
 * {@link CompletionFstScenario}, then persists the suggester via its
 * {@link WFSTCompletionLookup#store(org.apache.lucene.store.DataOutput) store()}
 * method into a single file {@value #FILE_NAME}. WFST has no payloads so the
 * Entry record is reused as-is (weight only).
 */
public final class WfstScenario implements CorpusScenario {

    public static final String NAME = "wfst-blob";
    public static final String FILE_NAME = "wfst.bin";

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "WFSTCompletionLookup blob: seeded input/weight set persisted via store().";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        WFSTCompletionLookup lookup = new WFSTCompletionLookup(
                new ByteBuffersDirectory(), NAME + "-" + seed);
        lookup.build(new CompletionFstScenario.SeededIterator(CompletionFstScenario.seededEntries(seed)));
        try (var fos = Files.newOutputStream(target.resolve(FILE_NAME));
             var bos = new BufferedOutputStream(fos);
             var out = new OutputStreamDataOutput(bos)) {
            if (!lookup.store(out)) {
                throw new IOException(NAME + ": store() returned false (empty FST?)");
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path file = source.resolve(FILE_NAME);
        if (!Files.isRegularFile(file)) {
            throw new IOException(NAME + ": missing " + file);
        }
        WFSTCompletionLookup lookup = new WFSTCompletionLookup(
                new ByteBuffersDirectory(), NAME + "-" + seed);
        try (var fis = Files.newInputStream(file);
             var bis = new BufferedInputStream(fis);
             var in = new InputStreamDataInput(bis)) {
            if (!lookup.load(in)) {
                throw new IOException(NAME + ": load() returned false");
            }
        }
        long want = CompletionFstScenario.seededEntries(seed).size();
        if (lookup.getCount() != want) {
            throw new IOException(NAME + ": count mismatch, got " + lookup.getCount()
                    + " want " + want);
        }
        for (CompletionFstScenario.Entry e : CompletionFstScenario.seededEntries(seed)) {
            List<LookupResult> hits = lookup.lookup(e.surface(), false,
                    CompletionFstScenario.ENTRY_COUNT);
            boolean found = false;
            for (LookupResult r : hits) {
                if (r.key.toString().equals(e.surface())) {
                    found = true;
                    break;
                }
            }
            if (!found) {
                throw new IOException(NAME + ": surface '" + e.surface() + "' not in hits");
            }
        }
    }

}
