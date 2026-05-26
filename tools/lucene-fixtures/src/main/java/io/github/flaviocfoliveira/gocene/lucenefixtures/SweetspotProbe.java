package io.github.flaviocfoliveira.gocene.lucenefixtures;

import org.apache.lucene.index.DirectoryReader;
import org.apache.lucene.index.StoredFields;
import org.apache.lucene.index.Term;
import org.apache.lucene.misc.SweetSpotSimilarity;
import org.apache.lucene.search.BooleanClause;
import org.apache.lucene.search.BooleanQuery;
import org.apache.lucene.search.IndexSearcher;
import org.apache.lucene.search.PhraseQuery;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.ScoreDoc;
import org.apache.lucene.search.TermQuery;
import org.apache.lucene.search.TopDocs;
import org.apache.lucene.search.similarities.BM25Similarity;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Path;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.Locale;
import java.util.Map;
import java.util.Set;

/**
 * Sprint 114 T24 (rmp 4632) helper for {@code verify-sweetspot}. Opens a
 * Lucene index (the T9 {@code search-scoring-corpus} shape), runs the
 * fixed BM25 query catalogue twice — once under {@link BM25Similarity},
 * once under {@link SweetSpotSimilarity} — and asserts (1) hit-id sets
 * agree per query and (2) at least one (q,doc) score pair differs by
 * more than {@value #SCORE_DRIFT_EPS}.
 */
public final class SweetspotProbe {

    private static final String BODY_FIELD = "body";
    public static final double SCORE_DRIFT_EPS = 1e-3;
    public static final int MAX_HITS = 12;

    private SweetspotProbe() {}

    /** Run the probe and return the number of queries compared. */
    public static int run(Path source) throws IOException {
        try (FSDirectory dir = FSDirectory.open(source);
             DirectoryReader reader = DirectoryReader.open(dir)) {
            Map<String, Query> queries = buildQueries();
            StoredFields sf = reader.storedFields();
            IndexSearcher bm25 = new IndexSearcher(reader);
            bm25.setSimilarity(new BM25Similarity());
            IndexSearcher sweet = new IndexSearcher(reader);
            sweet.setSimilarity(new SweetSpotSimilarity());

            int compared = 0;
            boolean sawDrift = false;
            for (Map.Entry<String, Query> e : queries.entrySet()) {
                String qid = e.getKey();
                Query q = e.getValue();
                TopDocs aDocs = bm25.search(q, MAX_HITS);
                TopDocs bDocs = sweet.search(q, MAX_HITS);
                Set<String> aIds = collectIds(sf, aDocs);
                Set<String> bIds = collectIds(sf, bDocs);
                if (!aIds.equals(bIds)) {
                    throw new IOException(String.format(Locale.ROOT,
                            "sweetspot: hit-set drift for query %s: bm25=%s sweetspot=%s",
                            qid, aIds, bIds));
                }
                Map<String, Double> aScore = scoreById(sf, aDocs);
                Map<String, Double> bScore = scoreById(sf, bDocs);
                for (String id : aIds) {
                    double da = aScore.getOrDefault(id, 0.0);
                    double db = bScore.getOrDefault(id, 0.0);
                    if (Math.abs(da - db) > SCORE_DRIFT_EPS) sawDrift = true;
                }
                compared++;
            }
            if (!sawDrift) {
                throw new IOException("sweetspot: no score drift > " + SCORE_DRIFT_EPS
                        + " between BM25 and SweetSpotSimilarity across " + compared
                        + " queries — SweetSpot lengthNorm plateau not exercised");
            }
            return compared;
        }
    }

    /** Same query catalogue as {@code SearchScoringCorpusScenario.buildQueries()}. */
    private static Map<String, Query> buildQueries() {
        Map<String, Query> q = new LinkedHashMap<>();
        q.put("tq-alpha", new TermQuery(new Term(BODY_FIELD, "alpha")));
        q.put("tq-beta", new TermQuery(new Term(BODY_FIELD, "beta")));
        q.put("tq-gamma", new TermQuery(new Term(BODY_FIELD, "gamma")));
        q.put("tq-delta", new TermQuery(new Term(BODY_FIELD, "delta")));
        q.put("tq-epsilon", new TermQuery(new Term(BODY_FIELD, "epsilon")));
        q.put("ph-alpha-beta", new PhraseQuery(BODY_FIELD, "alpha", "beta"));
        q.put("ph-gamma-delta", new PhraseQuery(BODY_FIELD, "gamma", "delta"));
        q.put("bool-alpha-or-zeta", new BooleanQuery.Builder()
                .add(new TermQuery(new Term(BODY_FIELD, "alpha")), BooleanClause.Occur.SHOULD)
                .add(new TermQuery(new Term(BODY_FIELD, "zeta")), BooleanClause.Occur.SHOULD)
                .build());
        return q;
    }

    private static Set<String> collectIds(StoredFields sf, TopDocs td) throws IOException {
        Set<String> ids = new HashSet<>(td.scoreDocs.length * 2);
        for (ScoreDoc sd : td.scoreDocs) ids.add(sf.document(sd.doc).get("id"));
        return ids;
    }

    private static Map<String, Double> scoreById(StoredFields sf, TopDocs td) throws IOException {
        Map<String, Double> m = new LinkedHashMap<>(td.scoreDocs.length * 2);
        for (ScoreDoc sd : td.scoreDocs) m.put(sf.document(sd.doc).get("id"), (double) sd.score);
        return m;
    }
}
