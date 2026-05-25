package io.github.flaviocfoliveira.gocene.lucenefixtures;

import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SmokeScenario;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Registry of binary-compatibility scenarios.
 *
 * <p>Sprint 114 T2 ships a single smoke scenario. T3 and the per-package T5..Tn
 * tasks register their own scenarios here.
 */
public final class Scenarios {

    private static final Map<String, CorpusScenario> REGISTRY = new LinkedHashMap<>();

    static {
        register(new SmokeScenario());
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
        return Map.copyOf(REGISTRY);
    }
}
