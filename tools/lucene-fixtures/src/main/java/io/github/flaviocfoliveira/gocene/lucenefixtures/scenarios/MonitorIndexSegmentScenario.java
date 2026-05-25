package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.core.KeywordAnalyzer;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.index.IndexWriterConfig;
import org.apache.lucene.index.NoMergePolicy;
import org.apache.lucene.index.SerialMergeScheduler;
import org.apache.lucene.index.Term;
import org.apache.lucene.monitor.Monitor;
import org.apache.lucene.monitor.MonitorConfiguration;
import org.apache.lucene.monitor.MonitorQuery;
import org.apache.lucene.monitor.MonitorQuerySerializer;
import org.apache.lucene.queryparser.classic.ParseException;
import org.apache.lucene.queryparser.classic.QueryParser;
import org.apache.lucene.search.TermQuery;
import org.apache.lucene.store.FSDirectory;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/**
 * Sprint 114 T18 (rmp 4626): {@code monitor-index-segment}. Addresses the
 * monitor audit row (verbatim): "No fixture from Lucene Monitor
 * persistence." Boots a {@link Monitor} backed by an {@link FSDirectory},
 * registers a fixed batch of five {@link MonitorQuery} objects, commits
 * them to the on-disk query index, then reopens the Monitor and asserts
 * the same query ids round-trip.
 *
 * <p>Determinism is enforced by a custom {@link MonitorConfiguration}
 * subclass that overrides {@code getIndexWriterConfig} to install
 * {@link NoMergePolicy}, {@link SerialMergeScheduler} and the
 * {@link Lucene104Codec} no-arg constructor — mirroring every other
 * {@code IndexCorpusScenario}-style fixture in the Sprint 114 harness.
 */
public final class MonitorIndexSegmentScenario implements CorpusScenario {

    public static final String NAME = "monitor-index-segment";
    public static final String FIELD = "body";
    public static final int QUERY_COUNT = 5;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Monitor query index: 5 registered MonitorQuery objects committed to an on-disk FSDirectory.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        List<MonitorQuery> queries = buildBatch();
        try (Monitor monitor = openMonitor(target)) {
            // register(Iterable) sorts internally only by what each individual
            // Presearcher writes; the registration order is stable.
            monitor.register(queries);
        }
        // Sanity-check: the Monitor must now report the queries on a clean
        // reopen (this is the same shape verify() asserts).
        assertReopenLoads(target, queries);
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        assertReopenLoads(source, buildBatch());
    }

    /** Opens a Monitor pointed at {@code target} with a fully-deterministic
     *  IndexWriter config. Production Monitor consumers usually accept the
     *  default; the fixture harness pins the merge policy / scheduler /
     *  codec instead so successive runs produce byte-identical segments. */
    private static Monitor openMonitor(Path target) throws IOException {
        MonitorConfiguration cfg = new MonitorConfiguration() {
            @Override
            protected IndexWriterConfig getIndexWriterConfig() {
                IndexWriterConfig iwc = new IndexWriterConfig(new KeywordAnalyzer());
                iwc.setCodec(new Lucene104Codec());
                iwc.setMergePolicy(NoMergePolicy.INSTANCE);
                iwc.setMergeScheduler(new SerialMergeScheduler());
                iwc.setUseCompoundFile(false);
                iwc.setCommitOnClose(true);
                return iwc;
            }
        };
        cfg.setDirectoryProvider(() -> FSDirectory.open(target), newSerializer());
        return new Monitor(new KeywordAnalyzer(), cfg);
    }

    /** Default Lucene serializer driven by a StandardAnalyzer-backed classic
     *  QueryParser over the {@code body} field. The Monitor needs this on
     *  reopen so {@link Monitor#getQuery} can rehydrate registered queries
     *  from the on-disk segment. */
    private static MonitorQuerySerializer newSerializer() {
        StandardAnalyzer analyzer = new StandardAnalyzer();
        QueryParser qp = new QueryParser(FIELD, analyzer);
        return MonitorQuerySerializer.fromParser(qs -> {
            try {
                return qp.parse(qs);
            } catch (ParseException e) {
                throw new RuntimeException(e);
            }
        });
    }

    /** Reopens the Monitor over {@code dir} and asserts every expected
     *  query id resolves to a non-null MonitorQuery with the same string
     *  representation. Pure read; no writes. */
    private static void assertReopenLoads(Path dir, List<MonitorQuery> expected) throws IOException {
        try (Monitor monitor = openMonitor(dir)) {
            for (MonitorQuery exp : expected) {
                MonitorQuery got = monitor.getQuery(exp.getId());
                if (got == null) {
                    throw new IOException(NAME + ": missing registered query id="
                            + exp.getId());
                }
                if (!got.getId().equals(exp.getId())) {
                    throw new IOException(NAME + ": id drift expected '" + exp.getId()
                            + "' got '" + got.getId() + "'");
                }
                // The Monitor's null-serializer mode stores the parsed Query
                // directly, so getQueryString may be null on reopen; equality
                // on the parsed Query.toString() is the stable assertion.
                String expS = exp.getQuery() == null ? "" : exp.getQuery().toString();
                String gotS = got.getQuery() == null ? "" : got.getQuery().toString();
                if (!expS.equals(gotS)) {
                    throw new IOException(NAME + ": query drift id=" + exp.getId()
                            + " expected '" + expS + "' got '" + gotS + "'");
                }
            }
        }
    }

    /** Five deterministic MonitorQuery objects — id-stable across seeds so
     *  the on-disk segment is the same byte-for-byte regardless of seed
     *  (the segment id stamped in the header is what differs between
     *  seeds, via Determinism.seed). */
    public static List<MonitorQuery> buildBatch() {
        List<MonitorQuery> out = new ArrayList<>(QUERY_COUNT);
        String[] terms = {"alpha", "beta", "gamma", "delta", "epsilon"};
        for (int i = 0; i < QUERY_COUNT; i++) {
            String id = String.format("q-%02d", i);
            String qs = FIELD + ":" + terms[i];
            TermQuery tq = new TermQuery(new Term(FIELD, terms[i]));
            out.add(new MonitorQuery(id, tq, qs, Collections.emptyMap()));
        }
        return out;
    }
}
