package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.Codec;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.Field;
import org.apache.lucene.facet.FacetsConfig;
import org.apache.lucene.facet.taxonomy.FloatAssociationFacetField;
import org.apache.lucene.facet.taxonomy.IntAssociationFacetField;
import org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyReader;
import org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter;
import org.apache.lucene.index.BinaryDocValues;
import org.apache.lucene.index.DocValues;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.LeafReaderContext;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.search.DocIdSetIterator;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.util.BitUtil;
import org.apache.lucene.util.BytesRef;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * Sprint 114 T12 (rmp 4620): {@code facet-association-payload}. Addresses
 * the facets audit row (verbatim): "No byte-level fixture for association
 * payloads". Persists Int/Float AssociationFacetFields into
 * {@code $facets.int}/{@code $facets.float} BinaryDocValues fields with
 * compound files disabled; verifies the [ord(4B BE), value(4B BE)] pairs
 * byte-for-byte against the seeded expectation.
 */
public final class FacetAssociationPayloadScenario implements CorpusScenario {

    public static final String NAME = "facet-association-payload";
    public static final int NUM_DOCS = 8;
    public static final String DIM_INT = "int";
    public static final String DIM_FLOAT = "float";
    /** Subdirectory holding the taxonomy sidecar (separate from main index). */
    public static final String TAXO_SUBDIR = "taxo";
    private static final String FIELD_INT_BIN = "$facets.int";
    private static final String FIELD_FLOAT_BIN = "$facets.float";

    @Override public String name() { return NAME; }

    @Override public String description() {
        return "Float/IntAssociationFacetField persisted as $facets.* BinaryDocValues "
                + "with compound files disabled.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        Path taxoDir = target.resolve(TAXO_SUBDIR);
        Files.createDirectories(taxoDir);
        FacetsConfig config = newConfig();
        Codec codec = new Lucene104Codec();
        try (FSDirectory mainDir = FSDirectory.open(target);
             FSDirectory taxFsDir = FSDirectory.open(taxoDir);
             DirectoryTaxonomyWriter taxoWriter = new DirectoryTaxonomyWriter(taxFsDir);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(codec)
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(mainDir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    Document d = new Document();
                    d.add(new StringField("id", "fa-" + i, Field.Store.YES));
                    d.add(new IntAssociationFacetField(seededInt(seed, i), DIM_INT, "v" + i));
                    d.add(new FloatAssociationFacetField(seededFloat(seed, i), DIM_FLOAT, "v" + i));
                    writer.addDocument(config.build(taxoWriter, d));
                }
                writer.commit();
            }
            taxoWriter.commit();
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path taxoDir = source.resolve(TAXO_SUBDIR);
        if (!Files.isDirectory(taxoDir)) {
            throw new IOException(NAME + ": missing taxonomy sidecar " + taxoDir);
        }
        try (FSDirectory mainDir = FSDirectory.open(source);
             FSDirectory taxFsDir = FSDirectory.open(taxoDir);
             IndexReader reader = org.apache.lucene.index.DirectoryReader.open(mainDir);
             DirectoryTaxonomyReader taxoReader = new DirectoryTaxonomyReader(taxFsDir)) {
            if (reader.numDocs() != NUM_DOCS) {
                throw new IOException(NAME + ": numDocs mismatch, got "
                        + reader.numDocs() + " expected " + NUM_DOCS);
            }
            verifyStream(reader, taxoReader, seed, FIELD_INT_BIN, DIM_INT, false);
            verifyStream(reader, taxoReader, seed, FIELD_FLOAT_BIN, DIM_FLOAT, true);
        }
    }

    /** Decodes one BinaryDocValues field; {@code asFloat} switches the value lane. */
    private static void verifyStream(IndexReader reader, DirectoryTaxonomyReader taxoReader,
            long seed, String field, String dim, boolean asFloat) throws IOException {
        for (LeafReaderContext ctx : reader.leaves()) {
            BinaryDocValues dv = DocValues.getBinary(ctx.reader(), field);
            int doc;
            while ((doc = dv.nextDoc()) != DocIdSetIterator.NO_MORE_DOCS) {
                BytesRef ref = dv.binaryValue();
                if (ref.length != 8) {
                    throw new IOException(NAME + ": " + dim + " payload length=" + ref.length
                            + " (want 8 for one [ord,value] pair) doc=" + doc);
                }
                int ord = (int) BitUtil.VH_BE_INT.get(ref.bytes, ref.offset);
                String[] comps = taxoReader.getPath(ord).components;
                if (comps.length != 2 || !dim.equals(comps[0])) {
                    throw new IOException(NAME + ": unexpected " + dim + " dim at ord=" + ord);
                }
                if (asFloat) {
                    float got = (float) BitUtil.VH_BE_FLOAT.get(ref.bytes, ref.offset + 4);
                    float want = seededFloat(seed, doc);
                    if (Float.floatToIntBits(got) != Float.floatToIntBits(want)) {
                        throw new IOException(NAME + ": " + dim + " doc=" + doc + " got=" + got + " want=" + want);
                    }
                } else {
                    int got = (int) BitUtil.VH_BE_INT.get(ref.bytes, ref.offset + 4);
                    int want = seededInt(seed, doc);
                    if (got != want) {
                        throw new IOException(NAME + ": " + dim + " doc=" + doc + " got=" + got + " want=" + want);
                    }
                }
            }
        }
    }

    /** Deterministic int association for {@code (seed, doc)}. */
    public static int seededInt(long seed, int doc) {
        return (int) ((seed * 0x9E3779B97F4A7C15L) ^ ((long) doc * 31L + 17L));
    }

    /** Deterministic float in [1.0, 2.0) — IEEE-754 mantissa from seed, fixed exponent. */
    public static float seededFloat(long seed, int doc) {
        int bits = (int) ((seed * 0xBF58476D1CE4E5B9L) ^ ((long) doc * 41L + 23L));
        return Float.intBitsToFloat((127 << 23) | (bits & 0x7FFFFF));
    }

    private static FacetsConfig newConfig() {
        FacetsConfig cfg = new FacetsConfig();
        cfg.setIndexFieldName(DIM_INT, FIELD_INT_BIN);
        cfg.setIndexFieldName(DIM_FLOAT, FIELD_FLOAT_BIN);
        return cfg;
    }
}
