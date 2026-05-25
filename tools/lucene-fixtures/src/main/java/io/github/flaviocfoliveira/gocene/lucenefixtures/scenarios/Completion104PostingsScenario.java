package io.github.flaviocfoliveira.gocene.lucenefixtures.scenarios;

import org.apache.lucene.codecs.Codec;
import org.apache.lucene.codecs.PostingsFormat;
import org.apache.lucene.codecs.lucene104.Lucene104Codec;
import org.apache.lucene.document.Document;
import org.apache.lucene.document.Field;
import org.apache.lucene.document.StringField;
import org.apache.lucene.index.IndexReader;
import org.apache.lucene.search.suggest.document.Completion104PostingsFormat;
import org.apache.lucene.search.suggest.document.PrefixCompletionQuery;
import org.apache.lucene.search.suggest.document.SuggestField;
import org.apache.lucene.search.suggest.document.SuggestIndexSearcher;
import org.apache.lucene.search.suggest.document.TopSuggestDocs;
import org.apache.lucene.analysis.standard.StandardAnalyzer;

import java.io.IOException;

/**
 * Sprint 114 T13 (rmp 4621): {@code completion104-postings}. Addresses
 * suggest audit row (verbatim): "No isolated, combined, or fixture coverage
 * of completion postings format." Builds a Lucene IndexWriter whose
 * {@link org.apache.lucene.codecs.Codec} routes the {@value #SUGGEST_FIELD}
 * field through {@link Completion104PostingsFormat}; on commit the segment
 * emits a {@code _0_Completion104_0.lkp} (completion dictionary) and a
 * {@code _0_Completion104_0.cmp} (completion index) pair.
 *
 * <p>Verify reopens the index, runs a {@link PrefixCompletionQuery} for the
 * common surface-form prefix, and asserts every seeded suggestion surfaces.
 */
public final class Completion104PostingsScenario extends IndexCorpusScenario {

    public static final String NAME = "completion104-postings";
    public static final String SUGGEST_FIELD = "suggest";
    public static final int NUM_DOCS = 10;

    @Override public String name() { return NAME; }
    @Override public String description() {
        return "Completion104PostingsFormat: SuggestField-indexed docs emit .lkp/.cmp pair.";
    }

    @Override protected int numDocs() { return NUM_DOCS; }

    @Override
    protected Codec codec() {
        // Wraps the default codec so the suggest field is routed to
        // Completion104PostingsFormat while everything else stays on
        // Lucene104PostingsFormat (mirrors TestSuggestField.iwcWithSuggestField).
        // Use Lucene104Codec.getPostingsFormatForField() to delegate non-suggest
        // fields to the canonical Lucene104PostingsFormat instance.
        return new Lucene104Codec() {
            private final PostingsFormat completion = new Completion104PostingsFormat();

            @Override
            public PostingsFormat getPostingsFormatForField(String field) {
                if (SUGGEST_FIELD.equals(field)) {
                    return completion;
                }
                return super.getPostingsFormatForField(field);
            }
        };
    }

    @Override
    protected Document buildDoc(int i, long seed) {
        Document doc = new Document();
        doc.add(new StringField("id", "sf-" + i, Field.Store.YES));
        // The same seeded entry set is exercised across all four scenarios:
        // each doc owns the i-th seeded suggestion.
        CompletionFstScenario.Entry e = CompletionFstScenario.seededEntries(seed).get(i % NUM_DOCS);
        doc.add(new SuggestField(SUGGEST_FIELD, e.surface(), e.weight()));
        return doc;
    }

    @Override
    protected void verifyReader(IndexReader reader, long seed) throws IOException {
        super.verifyReader(reader, seed);
        // Run a PrefixCompletionQuery for the common "term" prefix and check
        // we see every seeded surface form.
        SuggestIndexSearcher searcher = new SuggestIndexSearcher(reader);
        try (StandardAnalyzer analyzer = new StandardAnalyzer()) {
            PrefixCompletionQuery q = new PrefixCompletionQuery(analyzer,
                    new org.apache.lucene.index.Term(SUGGEST_FIELD, "term"));
            TopSuggestDocs hits = searcher.suggest(q, NUM_DOCS * 2, false);
            int found = 0;
            for (var sd : hits.scoreLookupDocs()) {
                String key = sd.key.toString();
                for (CompletionFstScenario.Entry e : CompletionFstScenario.seededEntries(seed)) {
                    if (e.surface().equals(key)) {
                        found++;
                        break;
                    }
                }
            }
            if (found != NUM_DOCS) {
                throw new IOException(NAME + ": expected " + NUM_DOCS
                        + " seeded suggestions to surface, got " + found);
            }
        }
    }
}
