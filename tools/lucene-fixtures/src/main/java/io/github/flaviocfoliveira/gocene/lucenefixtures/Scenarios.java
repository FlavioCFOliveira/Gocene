package io.github.flaviocfoliveira.gocene.lucenefixtures;

import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CompoundFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CompressingStoredFieldsScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.DocValuesFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FieldInfosFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FstBlobScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.IndexCorruptionScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.IndexDeletionsAndDvUpdatesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.KnnVectorsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.LiveDocsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.NormsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PerFieldDispatchScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PointsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PostingsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.ScalarQuantizedKnnScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SegmentInfoFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SmokeScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SoftDeletesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.StorePrimitivesScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.StoredFieldsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.TermVectorsFormatScenario;

import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Registry of binary-compatibility scenarios.
 *
 * <p>Sprint 114 T2 shipped the smoke scenario. T3 adds the foundational format
 * scenarios listed below. Further per-package tasks (T5..Tn) plug additional
 * scenarios in via {@link #register(CorpusScenario)}.
 */
public final class Scenarios {

    private static final Map<String, CorpusScenario> REGISTRY = new LinkedHashMap<>();

    static {
        register(new SmokeScenario());
        register(new StorePrimitivesScenario());
        register(new PostingsFormatScenario());
        register(new DocValuesFormatScenario());
        register(new StoredFieldsFormatScenario());
        register(new TermVectorsFormatScenario());
        register(new NormsFormatScenario());
        register(new PointsFormatScenario());
        register(new KnnVectorsFormatScenario());
        register(new CompoundFormatScenario());
        register(new FieldInfosFormatScenario());
        register(new SegmentInfoFormatScenario());
        register(new LiveDocsFormatScenario());
        register(new FstBlobScenario());
        // Sprint 114 T7 (rmp 4615): three new codec-specific scenarios.
        register(new PerFieldDispatchScenario());
        register(new CompressingStoredFieldsScenario());
        register(new ScalarQuantizedKnnScenario());
        // Sprint 114 T8 (rmp 4616): two new index-package scenarios.
        // Appended at the end of the registration order so existing
        // baseline.tsv rows keep their position.
        register(new IndexDeletionsAndDvUpdatesScenario());
        register(new IndexCorruptionScenario());
        // T8 helper: soft-deletes coverage for the soft-deletes audit row.
        register(new SoftDeletesScenario());
    }

    private Scenarios() {}

    public static void register(CorpusScenario scenario) {
        if (REGISTRY.containsKey(scenario.name())) {
            throw new IllegalStateException("scenario already registered: " + scenario.name());
        }
        REGISTRY.put(scenario.name(), scenario);
    }

    public static CorpusScenario require(String name) {
        CorpusScenario s = REGISTRY.get(name);
        if (s == null) {
            throw new IllegalArgumentException("unknown scenario: " + name
                    + " (known: " + REGISTRY.keySet() + ")");
        }
        return s;
    }

    public static Map<String, CorpusScenario> all() {
        // Preserve insertion order so the CLI list / manifest / Makefile loop are stable.
        return Collections.unmodifiableMap(new LinkedHashMap<>(REGISTRY));
    }
}
