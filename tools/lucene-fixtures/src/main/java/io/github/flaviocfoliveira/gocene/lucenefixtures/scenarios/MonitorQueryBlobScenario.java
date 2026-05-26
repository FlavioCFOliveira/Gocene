package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import io.github.flaviocfoliveira.gocene.lucenefixtures.CorpusScenario;
import io.github.flaviocfoliveira.gocene.lucenefixtures.Determinism;
import org.apache.lucene.analysis.standard.StandardAnalyzer;
import org.apache.lucene.codecs.CodecUtil;
import org.apache.lucene.index.Term;
import org.apache.lucene.monitor.MonitorQuery;
import org.apache.lucene.monitor.MonitorQuerySerializer;
import org.apache.lucene.queryparser.classic.ParseException;
import org.apache.lucene.queryparser.classic.QueryParser;
import org.apache.lucene.search.BooleanClause;
import org.apache.lucene.search.BooleanQuery;
import org.apache.lucene.search.PhraseQuery;
import org.apache.lucene.search.Query;
import org.apache.lucene.search.TermQuery;
import org.apache.lucene.store.ChecksumIndexInput;
import org.apache.lucene.store.FSDirectory;
import org.apache.lucene.store.IOContext;
import org.apache.lucene.store.IndexOutput;
import org.apache.lucene.util.BytesRef;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Locale;

/**
 * Sprint 114 T18 (rmp 4626): {@code monitor-query-blob}. Addresses the
 * monitor audit row (verbatim): "No round-trip test against Lucene-serialised
 * MonitorQuery blobs." Emits a single {@code monitor-queries.bin} file
 * framed by CodecUtil, containing the {@link MonitorQuerySerializer}
 * default-serialiser output for a small fixed batch of three
 * {@link MonitorQuery} objects (TermQuery, BooleanQuery, PhraseQuery).
 *
 * <p>File layout (manually framed because MonitorQuerySerializer is byte-
 * oriented; CodecUtil framing makes the artefact self-describing):
 * <pre>
 *   IndexHeader( codec="GoceneMonitorQueryBlob", v0, id=16B(seed), suffix="" )
 *   vInt    queryCount = 3
 *   for each query:
 *     vInt   blobLength
 *     bytes  blob  // MonitorQuerySerializer.fromParser(QueryParser).serialize(mq)
 *   Footer  ( CodecUtil )
 * </pre>
 *
 * <p>Metadata maps are kept EMPTY on every {@link MonitorQuery} so that the
 * underlying HashMap iteration order in Lucene's default serialiser cannot
 * leak into the byte stream — the only source of non-determinism that
 * scenario's wire format would otherwise expose.
 */
public final class MonitorQueryBlobScenario implements CorpusScenario {

    public static final String NAME = "monitor-query-blob";
    public static final String CODEC = "GoceneMonitorQueryBlob";
    public static final int VERSION = 0;
    public static final String FILE_NAME = "monitor-queries.bin";
    public static final String FIELD = "body";

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "MonitorQuerySerializer wire format: 3 MonitorQuery blobs (term/bool/phrase) framed by CodecUtil.";
    }

    @Override
    public void generate(Path target, long seed) throws IOException {
        Determinism.seed(seed);
        Files.createDirectories(target);
        List<MonitorQuery> queries = buildBatch(seed);
        MonitorQuerySerializer ser = newSerializer();
        try (FSDirectory dir = FSDirectory.open(target);
             IndexOutput out = dir.createOutput(FILE_NAME, IOContext.DEFAULT)) {
            CodecUtil.writeIndexHeader(out, CODEC, VERSION, Determinism.idBytes(seed), "");
            out.writeVInt(queries.size());
            for (MonitorQuery mq : queries) {
                BytesRef blob = ser.serialize(mq);
                out.writeVInt(blob.length);
                out.writeBytes(blob.bytes, blob.offset, blob.length);
            }
            CodecUtil.writeFooter(out);
        }
    }

    @Override
    public void verify(Path source, long seed) throws IOException {
        Determinism.seed(seed);
        List<MonitorQuery> expected = buildBatch(seed);
        MonitorQuerySerializer ser = newSerializer();
        try (FSDirectory dir = FSDirectory.open(source);
             ChecksumIndexInput in = dir.openChecksumInput(FILE_NAME)) {
            CodecUtil.checkIndexHeader(in, CODEC, VERSION, VERSION, Determinism.idBytes(seed), "");
            int got = in.readVInt();
            if (got != expected.size()) {
                throw new IOException(NAME + ": queryCount mismatch, expected "
                        + expected.size() + ", got " + got);
            }
            for (int i = 0; i < expected.size(); i++) {
                int len = in.readVInt();
                byte[] blob = new byte[len];
                in.readBytes(blob, 0, len);
                MonitorQuery decoded = ser.deserialize(new BytesRef(blob));
                MonitorQuery exp = expected.get(i);
                if (!decoded.getId().equals(exp.getId())) {
                    throw new IOException(String.format(Locale.ROOT,
                            "%s: blob[%d] id mismatch, expected '%s' got '%s'",
                            NAME, i, exp.getId(), decoded.getId()));
                }
                if (!decoded.getQueryString().equals(exp.getQueryString())) {
                    throw new IOException(NAME + ": blob[" + i + "] queryString mismatch, expected '"
                            + exp.getQueryString() + "' got '" + decoded.getQueryString() + "'");
                }
                if (!decoded.getMetadata().equals(exp.getMetadata())) {
                    throw new IOException(NAME + ": blob[" + i + "] metadata mismatch, expected "
                            + exp.getMetadata() + " got " + decoded.getMetadata());
                }
            }
            CodecUtil.checkFooter(in);
        }
    }

    /** Builds the three canonical MonitorQuery objects. The query strings
     *  are intentionally seed-independent so the blob layout (and therefore
     *  every byte the harness emits) is stable across the two canary seeds,
     *  while the per-seed segment id (Determinism.idBytes) keeps the header
     *  bytes distinct between seeds. */
    public static List<MonitorQuery> buildBatch(long seed) {
        List<MonitorQuery> out = new ArrayList<>(3);
        // 1. TermQuery: body:lucene
        TermQuery tq = new TermQuery(new Term(FIELD, "lucene"));
        out.add(new MonitorQuery("term-1", tq, "body:lucene", Collections.emptyMap()));
        // 2. BooleanQuery: +body:gocene +body:port
        BooleanQuery.Builder bq = new BooleanQuery.Builder();
        bq.add(new TermQuery(new Term(FIELD, "gocene")), BooleanClause.Occur.MUST);
        bq.add(new TermQuery(new Term(FIELD, "port")), BooleanClause.Occur.MUST);
        out.add(new MonitorQuery("bool-1", bq.build(),
                "+body:gocene +body:port", Collections.emptyMap()));
        // 3. PhraseQuery: body:"binary compatibility"
        PhraseQuery pq = new PhraseQuery.Builder()
                .add(new Term(FIELD, "binary"))
                .add(new Term(FIELD, "compatibility"))
                .build();
        out.add(new MonitorQuery("phrase-1", pq,
                "body:\"binary compatibility\"", Collections.emptyMap()));
        return out;
    }

    /** The default Lucene serializer driven by a StandardAnalyzer-backed
     *  classic QueryParser over the {@code body} field. */
    public static MonitorQuerySerializer newSerializer() {
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
}
