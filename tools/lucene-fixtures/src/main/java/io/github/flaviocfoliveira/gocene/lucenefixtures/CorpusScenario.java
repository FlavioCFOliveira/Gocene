package io.github.flaviocfoliveira.gocene.lucenefixtures;

import java.io.IOException;
import java.nio.file.Path;

/**
 * A binary-compatibility test scenario.
 *
 * <p>Implementations are registered in {@link Scenarios} and exposed through the
 * {@code gen} and {@code verify} CLIs. Each scenario MUST be deterministic for a
 * given seed: identical seeds MUST produce byte-identical artefacts.
 */
public interface CorpusScenario {

    /** Stable, kebab-case identifier used by the CLIs and Go-side helpers. */
    String name();

    /** Free-form human-readable description (English). */
    String description();

    /**
     * Produces the scenario's artefact deterministically at {@code target}.
     *
     * @param target output path (file or directory, scenario-specific)
     * @param seed   deterministic seed for the generator
     */
    void generate(Path target, long seed) throws IOException;

    /**
     * Verifies that {@code source} is a valid representation of this scenario
     * for the given seed. Implementations MUST fail fast with a clear message.
     *
     * @param source path produced by Gocene (or by a previous Lucene run)
     * @param seed   the seed the artefact was produced with
     */
    void verify(Path source, long seed) throws IOException;
}
