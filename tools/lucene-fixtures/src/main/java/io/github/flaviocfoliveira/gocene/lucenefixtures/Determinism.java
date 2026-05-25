package io.github.flaviocfoliveira.gocene.lucenefixtures;

import org.apache.lucene.util.StringHelper;

import java.lang.reflect.Field;
import java.math.BigInteger;
import java.nio.ByteBuffer;

/**
 * Centralises all determinism shims required to make Apache Lucene 10.4.0
 * produce byte-identical artefacts for a given seed.
 *
 * <p>Two effects are forced:
 * <ol>
 *   <li>The {@code tests.seed} system property is set BEFORE any Lucene class
 *       that observes it (notably {@link StringHelper}) is initialised. This
 *       seeds the xorshift128 PRNG used by {@link StringHelper#randomId()}
 *       — which Lucene calls to stamp segment IDs and codec headers — to a
 *       deterministic value derived from {@code seed}.</li>
 *   <li>If {@link StringHelper} has already loaded (for example when the
 *       harness is exercised in-process by JUnit), {@code nextId} is reset
 *       via reflection so that every {@code generate()} call starts from the
 *       same point regardless of test ordering.</li>
 * </ol>
 *
 * <p>The 128-bit reset value is derived from the seed as the unsigned
 * concatenation of {@code (seed, ~seed)} big-endian, which gives every seed
 * a distinct, non-zero starting state without colliding with adjacent seeds.
 */
public final class Determinism {

    private Determinism() {}

    /**
     * Forces deterministic state in all Lucene PRNGs we observed during
     * Sprint 114. Safe to call multiple times.
     */
    public static void seed(long seed) {
        // (1) ensure tests.seed is set in case StringHelper has not loaded yet.
        // StringHelper reads the LAST 8 hex chars of the property and parses
        // them as an unsigned hex long. Use the low 32 bits of the seed and
        // its complement so the property remains seed-specific.
        int low = (int) seed;
        int high = (int) ~seed;
        long packed = ((long) high << 32) | (low & 0xFFFFFFFFL);
        System.setProperty("tests.seed", String.format("%016X", packed));

        // (2) if StringHelper has already loaded, override nextId via reflection.
        try {
            Field f = StringHelper.class.getDeclaredField("nextId");
            f.setAccessible(true);
            f.set(null, new BigInteger(1, idBytes(seed)));
        } catch (ReflectiveOperationException | RuntimeException e) {
            // If reflection is blocked, the JVM must have been launched with
            // --add-opens java.base/java.lang=ALL-UNNAMED or the property must
            // have been set before StringHelper loaded. We still proceed; the
            // unit tests will catch any non-determinism.
            System.err.println("Determinism.seed: could not reset StringHelper.nextId: " + e);
        }
    }

    /** Deterministic 16-byte id derived from {@code seed}. */
    public static byte[] idBytes(long seed) {
        ByteBuffer buf = ByteBuffer.allocate(16);
        buf.putLong(seed);
        buf.putLong(~seed);
        return buf.array();
    }
}
