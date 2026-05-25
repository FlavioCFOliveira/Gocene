package io.github.flaviocfoliveira.gocene.lucenefixtures;

import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.CompoundFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.DocValuesFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FieldInfosFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.FstBlobScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.KnnVectorsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.LiveDocsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.NormsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PointsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.PostingsFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SegmentInfoFormatScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SmokeScenario;
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
