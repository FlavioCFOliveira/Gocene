package io.github.flaviocfoliveira.gocene.lucenefixtures;

import io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios.SmokeScenario;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * Sprint 114 T2 smoke test: proves the harness produces and verifies a byte-deterministic
 * fixture under {@link SmokeScenario}.
 */
class SmokeScenarioTest {

    private final SmokeScenario scenario = new SmokeScenario();

    @Test
    void roundTripSucceedsForSeedZero(@TempDir Path tmp) throws IOException {
        Path target = tmp.resolve("a");
        scenario.generate(target, 0L);
        scenario.verify(target, 0L);
    }

    @Test
    void roundTripSucceedsForRandomSeed(@TempDir Path tmp) throws IOException {
        Path target = tmp.resolve("b");
        scenario.generate(target, 0xC0FFEEL);
        scenario.verify(target, 0xC0FFEEL);
    }

    @Test
    void verifyFailsWhenSeedMismatch(@TempDir Path tmp) throws IOException {
        Path target = tmp.resolve("c");
        scenario.generate(target, 0L);
        IOException e = assertThrows(IOException.class, () -> scenario.verify(target, 1L));
        // either id mismatch (CodecUtil) or payload mismatch
        assertNotEquals("", e.getMessage());
    }

    @Test
    void writeIsByteDeterministic(@TempDir Path tmp) throws IOException {
        Path a = tmp.resolve("d1");
        Path b = tmp.resolve("d2");
        scenario.generate(a, 42L);
        scenario.generate(b, 42L);
        byte[] bytesA = SmokeScenario.readAllBytes(a);
        byte[] bytesB = SmokeScenario.readAllBytes(b);
        assertArrayEquals(bytesA, bytesB,
                "two writes with identical seed must produce byte-identical output");
        // Sanity: expected size = indexHeader(9+len("GoceneSmoke")=20) + 16 id + 1 suffix-len
        //                       + 4 (count) + 4*8 (payload) + 16 (footer) = 89
        assertEquals(89, bytesA.length);
    }

    @Test
    void differentSeedsProduceDifferentBytes(@TempDir Path tmp) throws IOException {
        Path a = tmp.resolve("e1");
        Path b = tmp.resolve("e2");
        scenario.generate(a, 0L);
        scenario.generate(b, 1L);
        byte[] bytesA = SmokeScenario.readAllBytes(a);
        byte[] bytesB = SmokeScenario.readAllBytes(b);
        // sizes are identical but contents must differ
        assertEquals(bytesA.length, bytesB.length);
        assertNotEquals(true, Arrays.equals(bytesA, bytesB),
                "different seeds must produce different bytes");
    }

    @Test
    void mainCliRoundTrip(@TempDir Path tmp) throws IOException {
        Path target = tmp.resolve("cli");
        Files.createDirectories(target);
        int genRc = Main.run(new String[]{"gen", "smoke", "7", target.toString()},
                System.out, System.err);
        assertEquals(0, genRc);
        int verifyRc = Main.run(new String[]{"verify", "smoke", "7", target.toString()},
                System.out, System.err);
        assertEquals(0, verifyRc);
    }

    @Test
    void mainCliUnknownScenarioReturnsTwo(@TempDir Path tmp) {
        int rc = Main.run(new String[]{"gen", "does-not-exist", "0", tmp.toString()},
                System.out, System.err);
        assertEquals(2, rc);
    }
}
