package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.Analyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.FieldType;
import org.apache.lucene.document.StoredField;
import org.apache.lucene.document.TextField;
import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.IndexOptions;
import org.apache.lucene.index.IndexWriter;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.index.Term;
import org.apache.lucene.search.PhraseQuery;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.vectorhighlight.FastVectorHighlighter;
import org.apache.lucene.search.vectorhighlight.FieldPhraseList;
import org.apache.lucene.search.vectorhighlight.FieldPhraseList.WeightedPhraseInfo;
import org.apache.lucene.search.vectorhighlight.FieldQuery;
import org.apache.lucene.search.vectorhighlight.FieldTermStack;
import org.apache.lucene.store.FSDirectory;

import java.io.BufferedReader;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Sprint 114 T14 (rmp 4622): {@code fast-vector-highlight-phrases}.
 * Audit row (verbatim, FastVectorHighlighter phrase list): "No Lucene
 * fixture for vector-highlight inputs". Indexes docs with
 * positions+offsets+term-vectors, runs FVH (via FieldTermStack +
 * FieldPhraseList for per-phrase offsets) over phrase queries, emits
 * {@code fvh-phrases.tsv}.
 */
public final class FastVectorHighlightPhrasesScenario implements CorpusScenario {

    public static final int NUM_DOCS = 14;
    public static final String TSV_NAME = "fvh-phrases.tsv";

    private static final String FIELD_BODY = "body";
    private static final String FIELD_ID = "id";
    private static final List<String> QUERY_IDS = List.of(
            "ph-alpha-beta", "ph-gamma-delta", "ph-epsilon-zeta");

    @Override public String name() { return "fast-vector-highlight-phrases"; }

    @Override public String description() {
        return "FastVectorHighlighter phrase corpus: " + NUM_DOCS + " docs + 3 phrases";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        try (FSDirectory dir = FSDirectory.open(target);
             Analyzer analyzer = new StandardAnalyzer()) {
            IndexWriterConfig iwc = new IndexWriterConfig(analyzer)
                    .setCodec(new Lucene104Codec())
                    .setUseCompoundFile(false)
                    .setMergePolicy(NoMergePolicy.INSTANCE)
                    .setMergeScheduler(new SerialMergeScheduler())
                    .setCommitOnClose(true);
            try (IndexWriter writer = new IndexWriter(dir, iwc)) {
                for (int i = 0; i < NUM_DOCS; i++) writer.addDocument(buildDoc(i, seed));
            }
            try (DirectoryReader reader = DirectoryReader.open(dir)) {
                writeTsv(target.resolve(TSV_NAME), evaluate(reader));
            }
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        Path tsv = source.resolve(TSV_NAME);
        if (!Files.exists(tsv)) throw new IOException(name() + ": missing " + TSV_NAME);
        List<Row> recorded = readTsv(tsv);
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            List<Row> recomputed = evaluate(reader);
            if (recorded.size() != recomputed.size()) {
                throw new IOException(name() + ": row count drift recorded="
                        + recorded.size() + " recomputed=" + recomputed.size());
            }
            for (int i = 0; i < recorded.size(); i++) {
                if (!recorded.get(i).equals(recomputed.get(i))) {
                    throw new IOException(name() + ": row " + i + " drift: "
                            + recorded.get(i) + " vs " + recomputed.get(i));
                }
            }
        }
    }

    private static FieldType bodyFieldType() {
        FieldType ft = new FieldType(TextField.TYPE_STORED);
        ft.setIndexOptions(IndexOptions.DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS);
        ft.setStoreTermVectors(true);
        ft.setStoreTermVectorPositions(true);
        ft.setStoreTermVectorOffsets(true);
        ft.freeze();
        return ft;
    }

    private static Document buildDoc(int i, long seed) {
        Document doc = new Document();
        String id = "doc-" + i;
        doc.add(new StoredField(FIELD_ID, id));
        long mix = (seed * 0xBF58476D1CE4E5B9L) ^ (long) i;
        StringBuilder body = new StringBuilder();
        body.append("prefix-").append(i).append(' ');
        if ((mix & 1L) != 0L) body.append("alpha beta ");
        body.append("gamma delta ");
        if ((mix & 2L) != 0L) body.append("alpha beta extra ");
        if ((i % 3) == 0) body.append("epsilon zeta ");
        body.append("tail ").append(id);
        doc.add(new Field(FIELD_BODY, body.toString().trim(), bodyFieldType()));
        return doc;
    }

    private static Map<String, Query> buildQueries() {
        Map<String, Query> q = new LinkedHashMap<>();
        q.put("ph-alpha-beta", new PhraseQuery(FIELD_BODY, "alpha", "beta"));
        q.put("ph-gamma-delta", new PhraseQuery(FIELD_BODY, "gamma", "delta"));
        q.put("ph-epsilon-zeta", new PhraseQuery(FIELD_BODY, "epsilon", "zeta"));
        if (!q.keySet().equals(new java.util.LinkedHashSet<>(QUERY_IDS))) {
            throw new IllegalStateException("QUERY_IDS / buildQueries drift");
        }
        return q;
    }

    private static List<Row> evaluate(DirectoryReader reader) throws IOException {
        FastVectorHighlighter fvh = new FastVectorHighlighter();
        StoredFields sf = reader.storedFields();
        List<Row> rows = new ArrayList<>();
        for (Map.Entry<String, Query> e : buildQueries().entrySet()) {
            String qid = e.getKey();
            FieldQuery fq = fvh.getFieldQuery(e.getValue(), reader);
            for (int docId = 0; docId < reader.maxDoc(); docId++) {
                FieldTermStack stack = new FieldTermStack(reader, docId, FIELD_BODY, fq);
                List<WeightedPhraseInfo> infos = new FieldPhraseList(stack, fq).getPhraseList();
                if (infos == null || infos.isEmpty()) continue;
                String id = sf.document(docId).get(FIELD_ID);
                int phraseIndex = 0;
                for (WeightedPhraseInfo wpi : infos) {
                    rows.add(new Row(qid, id, phraseIndex++, wpi.getText(),
                            wpi.getStartOffset(), wpi.getEndOffset()));
                }
            }
        }
        rows.sort((a, b) -> {
            int c = a.queryId().compareTo(b.queryId());
            if (c != 0) return c;
            c = a.docId().compareTo(b.docId());
            return c != 0 ? c : Integer.compare(a.phraseIndex(), b.phraseIndex());
        });
        return rows;
    }

    private static void writeTsv(Path file, List<Row> rows) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("# query_id\tdoc_id\tphrase_index\tphrase_text\tstart_offset\tend_offset\n");
        for (Row r : rows) {
            sb.append(r.queryId()).append('\t').append(r.docId()).append('\t')
                    .append(r.phraseIndex()).append('\t').append(TsvEscape.escape(r.phraseText()))
                    .append('\t').append(r.startOffset()).append('\t').append(r.endOffset()).append('\n');
        }
        Files.writeString(file, sb.toString(), StandardCharsets.UTF_8);
    }

    public static List<Row> readTsv(Path file) throws IOException {
        List<Row> rows = new ArrayList<>();
        try (BufferedReader br = Files.newBufferedReader(file, StandardCharsets.UTF_8)) {
            String line;
            while ((line = br.readLine()) != null) {
                if (line.isEmpty() || line.startsWith("#")) continue;
                String[] cols = line.split("\t", -1);
                if (cols.length != 6) throw new IOException("malformed row: " + line);
                rows.add(new Row(cols[0], cols[1], Integer.parseInt(cols[2]),
                        TsvEscape.unescape(cols[3]),
                        Integer.parseInt(cols[4]), Integer.parseInt(cols[5])));
            }
        }
        return rows;
    }

    /** Single TSV row. */
    public record Row(String queryId, String docId, int phraseIndex, String phraseText,
                      int startOffset, int endOffset) {
        @Override public String toString() {
            return queryId + "#" + phraseIndex + "/" + docId + ":\"" + phraseText
                    + "\"[" + startOffset + "," + endOffset + ")";
        }
    }
}
