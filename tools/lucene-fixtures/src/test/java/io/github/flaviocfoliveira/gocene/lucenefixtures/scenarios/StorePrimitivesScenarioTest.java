package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.nio.file.Path;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.util.Arrays;

/**
 * Sprint 114 T6 byte-determinism gate for {@link StorePrimitivesScenario}.
 *
 * <p>Exercises the two task-mandated seeds (0xC0FFEE and 0xDECAF) and confirms
 * that two consecutive generations produce byte-identical output, plus that
 * generate→verify round-trips cleanly for both seeds.
 */
class StorePrimitivesScenarioTest {

    private static final long SEED_C0FFEE = 0xC0FFEEL;     // 12648430
    private static final long SEED_DECAF = 0xDECAFL;       // 912559

    private final StorePrimitivesScenario scenario = new StorePrimitivesScenario();

    @Test
    void roundTripSucceedsForCoffeeSeed(@TempDir Path tmp) throws IOException {
        Path target = tmp.resolve("a");
        scenario.generate(target, SEED_C0FFEE);
        scenario.verify(target, SEED_C0FFEE);
    }

    @Test
    void roundTripSucceedsForDecafSeed(@TempDir Path tmp) throws IOException {
        Path target = tmp.resolve("b");
        scenario.generate(target, SEED_DECAF);
        scenario.verify(target, SEED_DECAF);
    }

    @Test
    void writeIsByteDeterministicCoffee(@TempDir Path tmp) throws IOException {
        Path a = tmp.resolve("c1");
        Path b = tmp.resolve("c2");
        scenario.generate(a, SEED_C0FFEE);
        scenario.generate(b, SEED_C0FFEE);
        byte[] bytesA = StorePrimitivesScenario.readAllBytes(a);
        byte[] bytesB = StorePrimitivesScenario.readAllBytes(b);
        assertArrayEquals(bytesA, bytesB,
                "two generations with seed=0xC0FFEE must produce byte-identical output");
    }

    @Test
    void writeIsByteDeterministicDecaf(@TempDir Path tmp) throws IOException {
        Path a = tmp.resolve("d1");
        Path b = tmp.resolve("d2");
        scenario.generate(a, SEED_DECAF);
        scenario.generate(b, SEED_DECAF);
        byte[] bytesA = StorePrimitivesScenario.readAllBytes(a);
        byte[] bytesB = StorePrimitivesScenario.readAllBytes(b);
        assertArrayEquals(bytesA, bytesB,
                "two generations with seed=0xDECAF must produce byte-identical output");
    }

    @Test
    void differentSeedsProduceDifferentBytes(@TempDir Path tmp) throws IOException {
        Path a = tmp.resolve("e1");
        Path b = tmp.resolve("e2");
        scenario.generate(a, SEED_C0FFEE);
        scenario.generate(b, SEED_DECAF);
        byte[] bytesA = StorePrimitivesScenario.readAllBytes(a);
        byte[] bytesB = StorePrimitivesScenario.readAllBytes(b);
        assertFalse(Arrays.equals(bytesA, bytesB),
                "different seeds must produce different bytes");
    }

    @Test
    void verifyFailsWhenSeedMismatch(@TempDir Path tmp) throws IOException {
        Path target = tmp.resolve("f");
        scenario.generate(target, SEED_C0FFEE);
        IOException e = assertThrows(IOException.class, () -> scenario.verify(target, SEED_DECAF));
        assertNotEquals("", e.getMessage());
    }
}
