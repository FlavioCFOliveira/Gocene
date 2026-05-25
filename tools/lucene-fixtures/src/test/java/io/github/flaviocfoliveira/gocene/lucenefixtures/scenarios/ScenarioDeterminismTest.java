package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Manifest;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Scenarios;
import org.junit.jupiter.api.io.TempDir;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

import java.io.IOException;
import java.nio.file.Path;
import java.util.List;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * Sprint 114 T3 byte-determinism gate.
 *
 * <p>For every registered scenario and for two distinct seeds, this test
 * regenerates the artefact twice into separate directories and asserts that
 * the normalised digest (see {@link Manifest#snapshot(Path)}) is identical
 * across runs. Any source of non-determinism — wall-clock, unseeded PRNG,
 * uninitialised memory leaking into the output — will fail here.
 */
class ScenarioDeterminismTest {

    private static final List<Long> SEEDS = List.of(0L, 0xC0FFEEL);

    static Stream<Arguments> scenariosWithSeeds() {
        return Scenarios.all().values().stream()
                .flatMap(s -> SEEDS.stream().map(seed -> Arguments.of(s, seed)));
    }

    @ParameterizedTest(name = "{0} @ seed={1}")
    @MethodSource("scenariosWithSeeds")
    void scenarioIsByteDeterministic(CorpusScenario scenario, long seed, @TempDir Path tmp) throws IOException {
        Path a = tmp.resolve("a");
        Path b = tmp.resolve("b");
        Determinism.seed(seed);
        scenario.generate(a, seed);
        Determinism.seed(seed);
        scenario.generate(b, seed);
        Manifest.Snapshot snapA = Manifest.snapshot(a);
        Manifest.Snapshot snapB = Manifest.snapshot(b);
        assertEquals(snapA.sha256(), snapB.sha256(),
                "two generations with seed=" + seed + " produced different digests for "
                        + scenario.name() + " (fileCount a=" + snapA.fileCount()
                        + " b=" + snapB.fileCount() + ")");
        assertEquals(snapA.fileCount(), snapB.fileCount(),
                "two generations produced a different file count for " + scenario.name());
        scenario.verify(a, seed);
        scenario.verify(b, seed);
    }
}
