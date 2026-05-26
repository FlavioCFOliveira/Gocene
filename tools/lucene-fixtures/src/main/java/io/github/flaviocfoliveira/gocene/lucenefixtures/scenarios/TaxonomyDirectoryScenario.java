package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.facet.taxonomy.FacetLabel;
import org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyReader;
import org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter;
import org.apache.lucene.index.IndexWriterConfig.OpenMode;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

/**
 * Sprint 114 T12 (rmp 4620): {@code taxonomy-directory}. Addresses the
 * facets audit row (verbatim): "No fixture from Lucene-emitted taxonomy
 * directory". Builds a {@link DirectoryTaxonomyWriter} into the sidecar
 * {@code taxo/}, commits {@value #NUM_CATEGORIES} ordered category paths
 * derived from the seed, then asserts every ordinal round-trips via
 * {@link DirectoryTaxonomyReader} to its expected {@link FacetLabel}.
 */
public final class TaxonomyDirectoryScenario implements CorpusScenario {

    public static final String NAME = "taxonomy-directory";
    public static final int NUM_CATEGORIES = 12;
    /** Number of distinct dim parents auto-created by addCategory. */
    public static final int DIMS = 3;
    /** Sidecar subdirectory holding the taxonomy index. */
    public static final String TAXO_SUBDIR = "taxo";

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "DirectoryTaxonomyWriter sidecar: "
                + NUM_CATEGORIES + " ordered category paths derived from seed.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        Path taxoDir = target.resolve(TAXO_SUBDIR);
        Files.createDirectories(taxoDir);
        List<FacetLabel> labels = seededLabels(seed);
        try (FSDirectory dir = FSDirectory.open(taxoDir);
             DirectoryTaxonomyWriter writer =
                     new DirectoryTaxonomyWriter(dir, OpenMode.CREATE)) {
            for (FacetLabel cp : labels) {
                writer.addCategory(cp);
            }
            writer.commit();
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path taxoDir = source.resolve(TAXO_SUBDIR);
        if (!Files.isDirectory(taxoDir)) {
            throw new IOException(NAME + ": missing taxonomy sidecar dir " + taxoDir);
        }
        List<FacetLabel> expected = seededLabels(seed);
        try (FSDirectory dir = FSDirectory.open(taxoDir);
             DirectoryTaxonomyReader reader = new DirectoryTaxonomyReader(dir)) {
            // expectedSize = 1 (synthetic root) + DIMS (auto dim parents) + leaves.
            int expectedSize = 1 + DIMS + expected.size();
            if (reader.getSize() != expectedSize) {
                throw new IOException(NAME + ": taxonomy size mismatch, got "
                        + reader.getSize() + " expected " + expectedSize);
            }
            for (int i = 0; i < expected.size(); i++) {
                FacetLabel cp = expected.get(i);
                int ord = reader.getOrdinal(cp);
                if (ord == TaxonomyReaderInvalidOrdinal()) {
                    throw new IOException(NAME + ": category absent, path=" + cp);
                }
                FacetLabel readBack = reader.getPath(ord);
                if (readBack == null || readBack.compareTo(cp) != 0) {
                    throw new IOException(NAME + ": ord=" + ord
                            + " round-trip drift: want " + cp + " got " + readBack);
                }
            }
        }
    }

    /** Mirrors {@code TaxonomyReader.INVALID_ORDINAL} without importing the abstract base. */
    private static int TaxonomyReaderInvalidOrdinal() {
        return org.apache.lucene.facet.taxonomy.TaxonomyReader.INVALID_ORDINAL;
    }

    /** Deterministic 12-element list of {@code dim/value-i-tag} labels for {@code seed}. */
    public static List<FacetLabel> seededLabels(long seed) {
        String tag = String.format("%08x", seed & 0xFFFFFFFFL);
        String[] dims = {"Author", "Genre", "Year"};
        if (dims.length != DIMS) {
            throw new IllegalStateException("dims[] and DIMS out of sync");
        }
        List<FacetLabel> out = new ArrayList<>(NUM_CATEGORIES);
        for (int i = 0; i < NUM_CATEGORIES; i++) {
            out.add(new FacetLabel(dims[i % DIMS], "v" + i + "-" + tag));
        }
        return out;
    }
}
