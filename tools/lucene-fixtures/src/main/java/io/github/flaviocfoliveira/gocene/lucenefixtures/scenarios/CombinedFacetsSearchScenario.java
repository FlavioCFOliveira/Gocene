package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.facet.FacetField;
import org.apache.lucene.facet.FacetResult;
import org.apache.lucene.facet.Facets;
import org.apache.lucene.facet.FacetsCollector;
import org.apache.lucene.facet.FacetsCollectorManager;
import org.apache.lucene.facet.FacetsConfig;
import org.apache.lucene.facet.LabelAndValue;
import org.apache.lucene.facet.taxonomy.FastTaxonomyFacetCounts;
import org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyReader;
import org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.IndexWriterConfig.OpenMode;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.Term;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.TermQuery;
import org.apache.lucene.store.FSDirectory;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

/**
 * Sprint 114 T5 (rmp 4611), S3 {@code combined-facets-search}.
 * Faceted Lucene index + DirectoryTaxonomyWriter sidecar under {@value #TAXO_SUBDIR};
 * runs a TermQuery-gated drill-down under FacetsCollectorManager and emits
 * {@value #TSV_NAME} (dim, label, count) sorted by (dim asc, count desc, label asc).
 */
public final class CombinedFacetsSearchScenario implements CorpusScenario {

    public static final String NAME = "combined-facets-search";
    public static final String TSV_NAME = "s3-facet-counts.tsv";
    public static final String TAXO_SUBDIR = "taxo";
    public static final int NUM_DOCS = 16;
    public static final String[] COLORS = {"red", "green", "blue"};
    public static final String[] SIZES = {"s", "m", "l"};

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Facets sidecar + faceted query; emits s3-facet-counts.tsv (dim,label,count).";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        Path taxoPath = target.resolve(TAXO_SUBDIR);
        Files.createDirectories(taxoPath);
        FacetsConfig config = new FacetsConfig();
        try (FSDirectory mainDir = FSDirectory.open(target);
             FSDirectory taxoDir = FSDirectory.open(taxoPath);
             Analyzer analyzer = new StandardAnalyzer();
             DirectoryTaxonomyWriter taxoWriter =
                     new DirectoryTaxonomyWriter(taxoDir, OpenMode.CREATE)) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(mainDir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) {
                    Document d = new Document();
                    d.add(new StringField("id", "f-" + i, Field.Store.YES));
                    // A simple body so the TermQuery has something to match.
                    d.add(new TextField("body", "alpha pivot-" + i, Field.Store.NO));
                    d.add(new FacetField("color", pickColor(seed, i)));
                    d.add(new FacetField("size", pickSize(seed, i)));
                    writer.addDocument(config.build(taxoWriter, d));
                }
                taxoWriter.commit();
                writer.commit();
            }
            try (DirectoryReader reader = DirectoryReader.open(mainDir);
                 DirectoryTaxonomyReader taxoReader = new DirectoryTaxonomyReader(taxoDir)) {
                writeTsv(target.resolve(TSV_NAME), evaluate(reader, taxoReader, config));
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tsv = source.resolve(TSV_NAME);
        if (!Files.isRegularFile(tsv)) {
            throw new IOException(NAME + ": missing " + TSV_NAME);
        }
        Path taxoPath = source.resolve(TAXO_SUBDIR);
        if (!Files.isDirectory(taxoPath)) {
            throw new IOException(NAME + ": missing taxonomy sidecar " + taxoPath);
        }
        List<Row> recorded = readTsv(tsv);
        FacetsConfig config = new FacetsConfig();
        try (FSDirectory mainDir = FSDirectory.open(source);
             FSDirectory taxoDir = FSDirectory.open(taxoPath);
             DirectoryReader reader = DirectoryReader.open(mainDir);
             DirectoryTaxonomyReader taxoReader = new DirectoryTaxonomyReader(taxoDir)) {
            List<Row> recomputed = evaluate(reader, taxoReader, config);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(NAME + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                if (!recorded.get(i).equals(recomputed.get(i))) {
                    throw new IOException(NAME + ": row " + i + " drift: "
                            + recorded.get(i) + " vs " + recomputed.get(i));
                }
            }
        }
    }

    private static List<Row> evaluate(DirectoryReader reader,
                                      DirectoryTaxonomyReader taxoReader,
                                      FacetsConfig config) throws IOException {
        IndexSearcher searcher = new IndexSearcher(reader);
        // Query gates the corpus: TermQuery("body","alpha") matches every doc
        // (every doc has 'alpha' in body). Faceted aggregation runs over the
        // matched set; this gives a deterministic, non-trivial count surface
        // (every dim/value combination from pickColor/pickSize is hit).
        FacetsCollectorManager fcm = new FacetsCollectorManager();
        FacetsCollector fc = FacetsCollectorManager.search(searcher,
                new TermQuery(new Term("body", "alpha")), 0, fcm).facetsCollector();
        Facets facets = new FastTaxonomyFacetCounts(taxoReader, config, fc);
        List<Row> rows = new ArrayList<>();
        for (String dim : new String[]{"color", "size"}) {
            FacetResult result = facets.getTopChildren(10, dim);
            if (result == null) continue;
            for (LabelAndValue lv : result.labelValues) {
                rows.add(new Row(dim, lv.label, lv.value.intValue()));
            }
        }
        rows.sort((a, b) -> {
            int c = a.dim().compareTo(b.dim());
            if (c != 0) return c;
            c = Integer.compare(b.count(), a.count()); // descending count
            if (c != 0) return c;
            return a.label().compareTo(b.label());
        });
        return rows;
    }

    private static void writeTsv(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# dim\tlabel\tcount\n");
        for (Row r : rows) {
            sb.append(r.dim()).append('\t').append(r.label()).append('\t').append(r.count()).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    private static List<Row> readTsv(Path file) throws IOException {
        List<Row> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 3) throw new IOException("malformed row: " + line);
                rows.add(new Row(cols[0], cols[1], Integer.parseInt(cols[2])));
            }
        }
        return rows;
    }

    private static String pickColor(long seed, int i) {
        long mix = (seed * 0x9E3779B97F4A7C15L) ^ (long) i;
        return COLORS[(int) Math.floorMod(mix, COLORS.length)];
    }

    private static String pickSize(long seed, int i) {
        long mix = (seed * 0xBF58476D1CE4E5B9L) ^ ((long) i * 31L);
        return SIZES[(int) Math.floorMod(mix, SIZES.length)];
    }

    /** Single TSV row (dim, label, count). */
    public record Row(String dim, String label, int count) {
        @Override public String toString() { return dim + "/" + label + "=" + count; }
    }
}
