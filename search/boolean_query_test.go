// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestBooleanQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// bqOccurValues renders Occur.values().
var bqOccurValues = []search.Occur{search.MUST, search.FILTER, search.SHOULD, search.MUST_NOT}

// randomSimpleStringR renders TestUtil.randomSimpleString(Random): 0..10 chars
// in 'a'..'z'.
func randomSimpleStringR(r *rand.Rand) string {
	end := r.Intn(11)
	if end == 0 {
		// allow 0 length
		return ""
	}
	buffer := make([]byte, end)
	for i := 0; i < end; i++ {
		buffer[i] = byte('a' + r.Intn('z'-'a'+1))
	}
	return string(buffer)
}

// bqTermQuery renders new TermQuery(new Term(field, text)).
func bqTermQuery(field, text string) *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(field, text))
}

func TestBooleanQueryEquality(t *testing.T) {
	bq1 := search.NewBooleanQueryBuilder()
	bq1.Add(bqTermQuery("field", "value1"), search.SHOULD)
	bq1.Add(bqTermQuery("field", "value2"), search.SHOULD)
	nested1 := search.NewBooleanQueryBuilder()
	nested1.Add(bqTermQuery("field", "nestedvalue1"), search.SHOULD)
	nested1.Add(bqTermQuery("field", "nestedvalue2"), search.SHOULD)
	bq1.Add(nested1.Build(), search.SHOULD)

	bq2 := search.NewBooleanQueryBuilder()
	bq2.Add(bqTermQuery("field", "value1"), search.SHOULD)
	bq2.Add(bqTermQuery("field", "value2"), search.SHOULD)
	nested2 := search.NewBooleanQueryBuilder()
	nested2.Add(bqTermQuery("field", "nestedvalue1"), search.SHOULD)
	nested2.Add(bqTermQuery("field", "nestedvalue2"), search.SHOULD)
	bq2.Add(nested2.Build(), search.SHOULD)

	if !bq1.Build().Equals(bq2.Build()) {
		t.Fatalf("expected %v to equal %v", bq1.Build(), bq2.Build())
	}
}

func TestBooleanQueryEqualityDoesNotDependOnOrder(t *testing.T) {
	queries := []*search.TermQuery{bqTermQuery("foo", "bar"), bqTermQuery("foo", "baz")}
	for iter := 0; iter < 10; iter++ {
		var clauses []*search.BooleanClause
		numClauses := random().Intn(20)
		for i := 0; i < numClauses; i++ {
			var query search.Query = queries[random().Intn(len(queries))]
			if random().Intn(2) == 0 {
				query = search.NewBoostQuery(query, random().Float32())
			}
			occur := bqOccurValues[random().Intn(len(bqOccurValues))]
			clauses = append(clauses, search.NewBooleanClause(query, occur))
		}

		minShouldMatch := random().Intn(5)
		bq1Builder := search.NewBooleanQueryBuilder()
		bq1Builder.SetMinimumNumberShouldMatch(minShouldMatch)
		for _, clause := range clauses {
			bq1Builder.AddClause(clause)
		}
		bq1 := bq1Builder.Build()

		random().Shuffle(len(clauses), func(i, j int) { clauses[i], clauses[j] = clauses[j], clauses[i] })
		bq2Builder := search.NewBooleanQueryBuilder()
		bq2Builder.SetMinimumNumberShouldMatch(minShouldMatch)
		for _, clause := range clauses {
			bq2Builder.AddClause(clause)
		}
		bq2 := bq2Builder.Build()

		queryUtilsCheckEqual(t, bq1, bq2)
	}
}

func TestBooleanQueryEqualityOnDuplicateShouldClauses(t *testing.T) {
	bq1 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(random().Intn(2)).
		Add(bqTermQuery("foo", "bar"), search.SHOULD).
		Build()
	bq2 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(bq1.GetMinimumNumberShouldMatch()).
		Add(bqTermQuery("foo", "bar"), search.SHOULD).
		Add(bqTermQuery("foo", "bar"), search.SHOULD).
		Build()
	queryUtilsCheckUnequal(t, bq1, bq2)
}

func TestBooleanQueryEqualityOnDuplicateMustClauses(t *testing.T) {
	bq1 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(random().Intn(2)).
		Add(bqTermQuery("foo", "bar"), search.MUST).
		Build()
	bq2 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(bq1.GetMinimumNumberShouldMatch()).
		Add(bqTermQuery("foo", "bar"), search.MUST).
		Add(bqTermQuery("foo", "bar"), search.MUST).
		Build()
	queryUtilsCheckUnequal(t, bq1, bq2)
}

func TestBooleanQueryEqualityOnDuplicateFilterClauses(t *testing.T) {
	bq1 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(random().Intn(2)).
		Add(bqTermQuery("foo", "bar"), search.FILTER).
		Build()
	bq2 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(bq1.GetMinimumNumberShouldMatch()).
		Add(bqTermQuery("foo", "bar"), search.FILTER).
		Add(bqTermQuery("foo", "bar"), search.FILTER).
		Build()
	queryUtilsCheckEqual(t, bq1, bq2)
}

func TestBooleanQueryEqualityOnDuplicateMustNotClauses(t *testing.T) {
	bq1 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(random().Intn(2)).
		Add(search.Instance, search.MUST).
		Add(bqTermQuery("foo", "bar"), search.FILTER).
		Build()
	bq2 := search.NewBooleanQueryBuilder().
		SetMinimumNumberShouldMatch(bq1.GetMinimumNumberShouldMatch()).
		Add(search.Instance, search.MUST).
		Add(bqTermQuery("foo", "bar"), search.FILTER).
		Add(bqTermQuery("foo", "bar"), search.FILTER).
		Build()
	queryUtilsCheckEqual(t, bq1, bq2)
}

func TestBooleanQueryHashCodeIsStable(t *testing.T) {
	bq := search.NewBooleanQueryBuilder().
		Add(bqTermQuery("foo", randomSimpleStringR(random())), search.SHOULD).
		Add(bqTermQuery("foo", randomSimpleStringR(random())), search.SHOULD).
		Build()
	hashCode := bq.HashCode()
	if got := bq.HashCode(); got != hashCode {
		t.Fatalf("hashCode = %d, want %d", got, hashCode)
	}
}

func TestBooleanQueryTooManyClauses(t *testing.T) {
	// Bad code (such as in a Query.rewrite() impl) should be prevented from creating a BooleanQuery
	// that directly exceeds the maxClauseCount (prior to needing IndexSearcher.rewrite() to do a
	// full walk of the final result)
	bq := search.NewBooleanQueryBuilder()
	for i := 0; i < search.GetMaxClauseCount(); i++ {
		bq.Add(bqTermQuery("foo", "bar-"+strconv.Itoa(i)), search.SHOULD)
	}
	func() {
		defer func() {
			r := recover()
			if _, ok := r.(*search.TooManyClauses); !ok {
				t.Fatalf("expected IndexSearcher.TooManyClauses, got %v", r)
			}
		}()
		bq.Add(bqTermQuery("foo", "bar-MAX"), search.SHOULD)
	}()
}

// LUCENE-1630
func TestBooleanQueryNullOrSubScorer(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "a b c d", false))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)
	s := newSearcher(t, r)
	// this test relies upon coord being the default implementation,
	// otherwise scores are different!
	s.SetSimilarity(search.NewClassicSimilarity())

	q := search.NewBooleanQueryBuilder()
	q.Add(bqTermQuery("field", "a"), search.SHOULD)

	// PhraseQuery w/ no terms added returns a null scorer
	pq := search.NewPhraseQuery(0, "field")
	q.Add(pq, search.SHOULD)
	if got := mustSearch(t, s, q.Build(), 10).TotalHits.Value; got != 1 {
		t.Fatalf("totalHits = %d, want 1", got)
	}

	// A required clause which returns null scorer should return null scorer to
	// IndexSearcher.
	q = search.NewBooleanQueryBuilder()
	pq = search.NewPhraseQuery(0, "field")
	q.Add(bqTermQuery("field", "a"), search.SHOULD)
	q.Add(pq, search.MUST)
	if got := mustSearch(t, s, q.Build(), 10).TotalHits.Value; got != 0 {
		t.Fatalf("totalHits = %d, want 0", got)
	}

	dmq := search.NewDisjunctionMaxQuery([]search.Query{bqTermQuery("field", "a"), pq}, 1.0)
	if got := mustSearch(t, s, dmq, 10).TotalHits.Value; got != 1 {
		t.Fatalf("totalHits = %d, want 1", got)
	}

	mustClose(t, r, w, dir)
}

func TestBooleanQueryDeMorgan(t *testing.T) {
	dir1 := newDirectory()
	iw1 := newRandomIndexWriter(t, dir1)
	doc1 := document.NewDocument()
	doc1.Add(newTextField(t, "field", "foo bar", false))
	mustAddDocument(t, iw1, doc1)
	reader1 := mustGetReader(t, iw1)
	mustClose(t, iw1)

	dir2 := newDirectory()
	iw2 := newRandomIndexWriter(t, dir2)
	doc2 := document.NewDocument()
	doc2.Add(newTextField(t, "field", "foo baz", false))
	mustAddDocument(t, iw2, doc2)
	reader2 := mustGetReader(t, iw2)
	mustClose(t, iw2)

	query := search.NewBooleanQueryBuilder() // Query: +foo -ba*
	query.Add(bqTermQuery("field", "foo"), search.MUST)
	wildcardQuery := search.NewWildcardQueryWithRewrite(
		index.NewTerm("field", "ba*"),
		automaton.DefaultDeterminizeWorkLimit,
		search.ScoringBooleanRewrite)
	query.Add(wildcardQuery, search.MUST_NOT)

	multireader := newMultiReader(t, reader1, reader2)
	searcher := newSearcher(t, multireader)
	if got := mustSearch(t, searcher, query.Build(), 10).TotalHits.Value; got != 0 {
		t.Fatalf("totalHits = %d, want 0", got)
	}

	es := newCachedThreadPool()
	searcher = search.NewIndexSearcherWithExecutor(multireader, es)
	if testing.Verbose() {
		t.Logf("rewritten form: %v", mustRewrite(t, searcher, query.Build()))
	}
	if got := mustSearch(t, searcher, query.Build(), 10).TotalHits.Value; got != 0 {
		t.Fatalf("totalHits = %d, want 0", got)
	}
	es.shutdownAndAwaitTermination()

	mustClose(t, multireader, reader1, reader2, dir1, dir2)
}

func TestBooleanQueryBS2DisjunctionNextVsAdvance(t *testing.T) {
	d := newDirectory()
	w := newRandomIndexWriter(t, d)
	numDocs := atLeast(300)
	for docUpto := 0; docUpto < numDocs; docUpto++ {
		contents := "a"
		if random().Intn(20) <= 16 {
			contents += " b"
		}
		if random().Intn(20) <= 8 {
			contents += " c"
		}
		if random().Intn(20) <= 4 {
			contents += " d"
		}
		if random().Intn(20) <= 2 {
			contents += " e"
		}
		if random().Intn(20) <= 1 {
			contents += " f"
		}
		doc := document.NewDocument()
		f, err := document.NewTextField("field", contents, false)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f)
		mustAddDocument(t, w, doc)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := mustGetReader(t, w)
	s := newSearcher(t, r)
	mustClose(t, w)

	for iter := 0; iter < 10; iter++ { // 10 * RANDOM_MULTIPLIER
		if testing.Verbose() {
			t.Logf("iter=%d", iter)
		}
		terms := []string{"a", "b", "c", "d", "e", "f"}
		numTerms := nextInt(1, len(terms))
		for len(terms) > numTerms {
			i := random().Intn(len(terms))
			terms = append(terms[:i], terms[i+1:]...)
		}

		if testing.Verbose() {
			t.Logf("  terms=%v", terms)
		}

		q := search.NewBooleanQueryBuilder()
		for _, term := range terms {
			q.AddClause(search.NewBooleanClause(bqTermQuery("field", term), search.SHOULD))
		}

		weight := mustCreateWeight(t, s, mustRewrite(t, s, q.Build()), search.COMPLETE, 1)

		scorer := mustScorer(t, weight, s.GetLeafContexts()[0])

		// First pass: just use .nextDoc() to gather all hits
		var hits []*search.ScoreDoc
		for mustNextDoc(t, scorer.Iterator()) != search.NO_MORE_DOCS {
			hits = append(hits, search.NewScoreDoc(scorer.DocID(), mustScore(t, scorer), -1))
		}

		if testing.Verbose() {
			t.Logf("  %d hits", len(hits))
		}

		// Now, randomly next/advance through the list and
		// verify exact match:
		for iter2 := 0; iter2 < 10; iter2++ {

			weight = mustCreateWeight(t, s, mustRewrite(t, s, q.Build()), search.COMPLETE, 1)
			scorer = mustScorer(t, weight, s.GetLeafContexts()[0])

			if testing.Verbose() {
				t.Logf("  iter2=%d", iter2)
			}

			upto := -1
			for upto < len(hits) {
				var nextUpto, nextDoc int
				left := len(hits) - upto
				if left == 1 || random().Intn(2) == 0 {
					// next
					nextUpto = 1 + upto
					nextDoc = mustNextDoc(t, scorer.Iterator())
				} else {
					// advance
					inc := nextInt(1, left-1)
					nextUpto = inc + upto
					var err error
					nextDoc, err = scorer.Iterator().Advance(hits[nextUpto].Doc)
					if err != nil {
						t.Fatalf("advance: %v", err)
					}
				}

				if nextUpto == len(hits) {
					if nextDoc != search.NO_MORE_DOCS {
						t.Fatalf("nextDoc = %d, want NO_MORE_DOCS", nextDoc)
					}
				} else {
					hit := hits[nextUpto]
					if hit.Doc != nextDoc {
						t.Fatalf("nextDoc = %d, want %d", nextDoc, hit.Doc)
					}
					// Test for precise float equality:
					if actual := mustScore(t, scorer); hit.Score != actual {
						t.Fatalf("doc %d has wrong score: expected=%v actual=%v", hit.Doc, hit.Score, actual)
					}
				}
				upto = nextUpto
			}
		}
	}

	mustClose(t, r, d)
}

func TestBooleanQueryMinShouldMatchLeniency(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "a b c d", false))
	mustAddDocument(t, w, doc)
	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcher(t, r)
	bq := search.NewBooleanQueryBuilder()
	bq.Add(bqTermQuery("field", "a"), search.SHOULD)
	bq.Add(bqTermQuery("field", "b"), search.SHOULD)

	// No doc can match: BQ has only 2 clauses and we are asking for minShouldMatch=4
	bq.SetMinimumNumberShouldMatch(4)
	if got := mustSearch(t, s, bq.Build(), 1).TotalHits.Value; got != 0 {
		t.Fatalf("totalHits = %d, want 0", got)
	}
	mustClose(t, r, w, dir)
}

// bqGetMatches renders the private static getMatches(IndexSearcher, Query):
// searcher.search(query, FixedBitSetCollector.createManager(maxDoc)).
func bqGetMatches(t *testing.T, searcher *search.IndexSearcher, query search.Query) *util.FixedBitSet {
	t.Helper()
	matches, err := search.SearchWithCollectorManager(searcher, query, testsearch.FixedBitSetCollectorCreateManager(searcher.GetIndexReader().MaxDoc()))
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return matches
}

func TestBooleanQueryFILTERClauseBehavesLikeMUST(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	f := newTextField(t, "field", "a b c d", false)
	doc.Add(f)
	mustAddDocument(t, w, doc)
	f.SetStringValue("b d")
	mustAddDocument(t, w, doc)
	f.SetStringValue("d")
	mustAddDocument(t, w, doc)
	mustCommit(t, w)

	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)

	for _, requiredTerms := range [][]string{
		{"a", "d"},
		{"a", "b", "d"},
		{"d"},
		{"e"},
		{},
	} {
		bq1 := search.NewBooleanQueryBuilder()
		bq2 := search.NewBooleanQueryBuilder()
		for _, term := range requiredTerms {
			q := bqTermQuery("field", term)
			bq1.Add(q, search.MUST)
			bq2.Add(q, search.FILTER)
		}

		matches1 := bqGetMatches(t, searcher, bq1.Build())
		matches2 := bqGetMatches(t, searcher, bq2.Build())
		if !matches1.Equals(matches2) {
			t.Fatalf("matches differ for %v", requiredTerms)
		}
	}

	mustClose(t, reader, w, dir)
}

// bqScoreCheckingCollector renders the anonymous SimpleCollector of
// assertSameScoresWithoutFilters.
type bqScoreCheckingCollector struct {
	search.BaseSimpleCollector
	t        *testing.T
	searcher *search.IndexSearcher
	bq2      search.Query
	matched  *bool
	docBase  int
	scorer   search.Scorable
}

func (c *bqScoreCheckingCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	c.docBase = context.DocBase
	return nil
}

func (c *bqScoreCheckingCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *bqScoreCheckingCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *bqScoreCheckingCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *bqScoreCheckingCollector) Collect(doc int) error {
	actualScore, err := c.scorer.Score()
	if err != nil {
		return err
	}
	explanation, err := c.searcher.Explain(c.bq2, c.docBase+doc)
	if err != nil {
		return err
	}
	expectedScore := explanation.GetValue()
	if math.Abs(float64(expectedScore-actualScore)) > 10e-5 {
		c.t.Errorf("score = %v, want %v", actualScore, expectedScore)
	}
	*c.matched = true
	return nil
}

func (c *bqScoreCheckingCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *bqScoreCheckingCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func (c *bqScoreCheckingCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

func (c *bqScoreCheckingCollector) Finish() error { return nil }

// bqScoreCheckingCollectorManager renders the anonymous
// CollectorManager<SimpleCollector, Void> of assertSameScoresWithoutFilters.
type bqScoreCheckingCollectorManager struct {
	t        *testing.T
	searcher *search.IndexSearcher
	bq2      search.Query
	matched  *bool
}

func (m *bqScoreCheckingCollectorManager) NewCollector() (*bqScoreCheckingCollector, error) {
	c := &bqScoreCheckingCollector{t: m.t, searcher: m.searcher, bq2: m.bq2, matched: m.matched}
	c.Outer = c
	return c, nil
}

func (m *bqScoreCheckingCollectorManager) Reduce(collectors []*bqScoreCheckingCollector) (struct{}, error) {
	return struct{}{}, nil
}

// assertSameScoresWithoutFilters renders the private
// assertSameScoresWithoutFilters(IndexSearcher, BooleanQuery).
func assertSameScoresWithoutFilters(t *testing.T, searcher *search.IndexSearcher, bq *search.BooleanQuery) {
	t.Helper()
	bq2Builder := search.NewBooleanQueryBuilder()
	for _, c := range bq.Clauses() {
		if c.Occur() != search.FILTER {
			bq2Builder.AddClause(c)
		}
	}
	bq2Builder.SetMinimumNumberShouldMatch(bq.GetMinimumNumberShouldMatch())
	bq2 := bq2Builder.Build()

	matched := false
	manager := &bqScoreCheckingCollectorManager{t: t, searcher: searcher, bq2: bq2, matched: &matched}
	if _, err := search.SearchWithCollectorManager[*bqScoreCheckingCollector, struct{}](searcher, bq, manager); err != nil {
		t.Fatalf("search: %v", err)
	}

	if !matched {
		t.Fatal("expected a match")
	}
}

func TestBooleanQueryFilterClauseDoesNotImpactScore(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	f := newTextField(t, "field", "a b c d", false)
	doc.Add(f)
	mustAddDocument(t, w, doc)
	f.SetStringValue("b d")
	mustAddDocument(t, w, doc)
	f.SetStringValue("a d")
	mustAddDocument(t, w, doc)
	mustCommit(t, w)

	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)

	qBuilder := search.NewBooleanQueryBuilder()
	qBuilder.Add(bqTermQuery("field", "a"), search.FILTER)

	// With a single clause, we will rewrite to the underlying
	// query. Make sure that it returns null scores
	assertSameScoresWithoutFilters(t, searcher, qBuilder.Build())

	// Now with two clauses, we will get a conjunction scorer
	// Make sure it returns null scores
	qBuilder.Add(bqTermQuery("field", "b"), search.FILTER)
	q := qBuilder.Build()
	assertSameScoresWithoutFilters(t, searcher, q)

	// Now with a scoring clause, we need to make sure that
	// the boolean scores are the same as those from the term
	// query
	qBuilder.Add(bqTermQuery("field", "c"), search.SHOULD)
	q = qBuilder.Build()
	assertSameScoresWithoutFilters(t, searcher, q)

	// FILTER and empty SHOULD
	qBuilder = search.NewBooleanQueryBuilder()
	qBuilder.Add(bqTermQuery("field", "a"), search.FILTER)
	qBuilder.Add(bqTermQuery("field", "e"), search.SHOULD)
	q = qBuilder.Build()
	assertSameScoresWithoutFilters(t, searcher, q)

	// mix of FILTER and MUST
	qBuilder = search.NewBooleanQueryBuilder()
	qBuilder.Add(bqTermQuery("field", "a"), search.FILTER)
	qBuilder.Add(bqTermQuery("field", "d"), search.MUST)
	q = qBuilder.Build()
	assertSameScoresWithoutFilters(t, searcher, q)

	// FILTER + minShouldMatch
	qBuilder = search.NewBooleanQueryBuilder()
	qBuilder.Add(bqTermQuery("field", "b"), search.FILTER)
	qBuilder.Add(bqTermQuery("field", "a"), search.SHOULD)
	qBuilder.Add(bqTermQuery("field", "d"), search.SHOULD)
	qBuilder.SetMinimumNumberShouldMatch(1)
	q = qBuilder.Build()
	assertSameScoresWithoutFilters(t, searcher, q)

	mustClose(t, reader, w, dir)
}

// bqApproximationFixture renders the shared set-up of the five
// *PropagatesApproximations tests: one document "a b c", committed, and a
// plain IndexSearcher without query cache (to still have approximations).
func bqApproximationFixture(t *testing.T) (*search.IndexSearcher, *index.DirectoryReader, func()) {
	t.Helper()
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	f := newTextField(t, "field", "a b c", false)
	doc.Add(f)
	mustAddDocument(t, w, doc)
	mustCommit(t, w)

	reader := mustGetReader(t, w)
	// not LuceneTestCase.newSearcher to not have the asserting wrappers
	// and do instanceof checks
	searcher := search.NewIndexSearcher(reader)
	searcher.SetQueryCache(nil) // to still have approximations
	return searcher, reader, func() { mustClose(t, reader, w, dir) }
}

// bqApproximationScorer renders
// weight = searcher.createWeight(searcher.rewrite(q), COMPLETE, 1) followed by
// weight.scorer(leaves.get(0)).
func bqApproximationScorer(t *testing.T, searcher *search.IndexSearcher, reader *index.DirectoryReader, q search.Query) search.Scorer {
	t.Helper()
	weight := mustCreateWeight(t, searcher, mustRewrite(t, searcher, q), search.COMPLETE, 1)
	return mustScorer(t, weight, mustLeaves(t, reader)[0])
}

func TestBooleanQueryConjunctionPropagatesApproximations(t *testing.T) {
	searcher, reader, closeAll := bqApproximationFixture(t)

	pq := search.NewPhraseQuery(0, "field", "a", "b")

	q := search.NewBooleanQueryBuilder()
	q.Add(pq, search.MUST)
	q.Add(bqTermQuery("field", "c"), search.FILTER)

	scorer := bqApproximationScorer(t, searcher, reader, q.Build())
	if _, ok := scorer.(*search.ConjunctionScorer); !ok {
		t.Fatalf("scorer = %T, want *ConjunctionScorer", scorer)
	}
	if scorer.TwoPhaseIterator() == nil {
		t.Fatal("twoPhaseIterator is nil")
	}

	closeAll()
}

func TestBooleanQueryDisjunctionPropagatesApproximations(t *testing.T) {
	searcher, reader, closeAll := bqApproximationFixture(t)

	pq := search.NewPhraseQuery(0, "field", "a", "b")

	q := search.NewBooleanQueryBuilder()
	q.Add(pq, search.SHOULD)
	q.Add(bqTermQuery("field", "c"), search.SHOULD)

	scorer := bqApproximationScorer(t, searcher, reader, q.Build())
	if !search.IsDisjunctionScorer(scorer) {
		t.Fatalf("scorer = %T, want a DisjunctionScorer", scorer)
	}
	if scorer.TwoPhaseIterator() == nil {
		t.Fatal("twoPhaseIterator is nil")
	}

	closeAll()
}

func TestBooleanQueryBoostedScorerPropagatesApproximations(t *testing.T) {
	searcher, reader, closeAll := bqApproximationFixture(t)

	pq := search.NewPhraseQuery(0, "field", "a", "b")

	q := search.NewBooleanQueryBuilder()
	q.Add(pq, search.SHOULD)
	q.Add(bqTermQuery("field", "d"), search.SHOULD)

	scorer := bqApproximationScorer(t, searcher, reader, q.Build())
	if !search.IsPhraseScorer(scorer) {
		t.Fatalf("scorer = %T, want a PhraseScorer", scorer)
	}
	if scorer.TwoPhaseIterator() == nil {
		t.Fatal("twoPhaseIterator is nil")
	}

	closeAll()
}

func TestBooleanQueryExclusionPropagatesApproximations(t *testing.T) {
	searcher, reader, closeAll := bqApproximationFixture(t)

	pq := search.NewPhraseQuery(0, "field", "a", "b")

	q := search.NewBooleanQueryBuilder()
	q.Add(pq, search.SHOULD)
	q.Add(bqTermQuery("field", "c"), search.MUST_NOT)

	scorer := bqApproximationScorer(t, searcher, reader, q.Build())
	if _, ok := scorer.(*search.ReqExclScorer); !ok {
		t.Fatalf("scorer = %T, want *ReqExclScorer", scorer)
	}
	if scorer.TwoPhaseIterator() == nil {
		t.Fatal("twoPhaseIterator is nil")
	}

	closeAll()
}

func TestBooleanQueryReqOptPropagatesApproximations(t *testing.T) {
	searcher, reader, closeAll := bqApproximationFixture(t)

	pq := search.NewPhraseQuery(0, "field", "a", "b")

	q := search.NewBooleanQueryBuilder()
	q.Add(pq, search.MUST)
	q.Add(bqTermQuery("field", "c"), search.SHOULD)

	scorer := bqApproximationScorer(t, searcher, reader, q.Build())
	if _, ok := scorer.(*search.ReqOptSumScorer); !ok {
		t.Fatalf("scorer = %T, want *ReqOptSumScorer", scorer)
	}
	if scorer.TwoPhaseIterator() == nil {
		t.Fatal("twoPhaseIterator is nil")
	}

	closeAll()
}

// LUCENE-9620 Add Weight#count(LeafReaderContext)
func TestBooleanQueryQueryMatchesCount(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)

	randomNumDocs := nextInt(10, 100)
	numMatchingDocs := 0

	for i := 0; i < randomNumDocs; i++ {
		doc := document.NewDocument()
		var f *document.TextField
		if random().Intn(2) == 0 {
			f = newTextField(t, "field", "a b c "+strconv.Itoa(int(random().Int31())), false)
			numMatchingDocs++
		} else {
			f = newTextField(t, "field", strconv.Itoa(int(random().Int31())), false)
		}
		doc.Add(f)
		mustAddDocument(t, w, doc)
	}
	mustCommit(t, w)

	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)

	q := search.NewBooleanQueryBuilder()
	q.Add(search.NewPhraseQuery(0, "field", "a", "b"), search.SHOULD)
	q.Add(bqTermQuery("field", "c"), search.SHOULD)

	builtQuery := q.Build()

	if got := mustCount(t, searcher, builtQuery); got != numMatchingDocs {
		t.Fatalf("count = %d, want %d", got, numMatchingDocs)
	}

	mustClose(t, reader, w, dir)
}

// bqTwoDocLongIndex renders the index shared by the *MatchesCount tests: two
// documents (long=3, string=abc) and (long=10, string=xyz), opened with
// DirectoryReader.open(writer). extra, when non-nil, adds the long3dim point
// of testDisjunctionMatchesCount.
func bqTwoDocLongIndex(t *testing.T, withLong3dim bool) (*index.DirectoryReader, func()) {
	t.Helper()
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
	doc := document.NewDocument()
	longPoint := document.NewLongPoint("long", 3)
	doc.Add(longPoint)
	var longPoint3dim *document.LongPoint
	if withLong3dim {
		longPoint3dim = document.NewLongPoint("long3dim", 3, 4, 5)
		doc.Add(longPoint3dim)
	}
	stringField, err := document.NewStringField("string", "abc", false)
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(stringField)
	mustAddDocument(t, writer, doc)
	longPoint.SetLongValue(10)
	if withLong3dim {
		longPoint3dim.SetLongValues(10, 11, 12)
	}
	stringField.SetStringValue("xyz")
	mustAddDocument(t, writer, doc)
	reader := mustOpenDirectoryReaderFromWriter(t, writer)
	mustClose(t, writer)
	return reader, func() { mustClose(t, reader, dir) }
}

// bqWeightCount renders searcher.createWeight(query, COMPLETE, 1f).count(leaf).
func bqWeightCount(t *testing.T, searcher *search.IndexSearcher, query search.Query, leaf *index.LeafReaderContext) int {
	t.Helper()
	weight := mustCreateWeight(t, searcher, query, search.COMPLETE, 1)
	count, err := weight.Count(leaf)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

func bqAssertCount(t *testing.T, searcher *search.IndexSearcher, query search.Query, leaf *index.LeafReaderContext, want int) {
	t.Helper()
	if got := bqWeightCount(t, searcher, query, leaf); got != want {
		t.Fatalf("count(%v) = %d, want %d", query, got, want)
	}
}

func TestBooleanQueryConjunctionMatchesCount(t *testing.T) {
	reader, closeAll := bqTwoDocLongIndex(t, false)
	searcher := search.NewIndexSearcher(reader)
	leaf := mustLeaves(t, reader)[0]

	query := search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.MUST).
		Add(longPointNewExactQuery(t, "long", 3), search.FILTER).
		Build()
	// Both queries match a single doc, BooleanWeight can't figure out the count of the conjunction
	bqAssertCount(t, searcher, query, leaf, -1)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "missing"), search.MUST).
		Add(longPointNewExactQuery(t, "long", 3), search.FILTER).
		Build()
	// One query has a count of 0, the conjunction has a count of 0 too
	bqAssertCount(t, searcher, query, leaf, 0)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.MUST).
		Add(longPointNewExactQuery(t, "long", 5), search.FILTER).
		Build()
	// One query has a count of 0, the conjunction has a count of 0 too
	bqAssertCount(t, searcher, query, leaf, 0)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.MUST).
		Add(longPointNewRangeQuery(t, "long", 0, 10), search.FILTER).
		Build()
	// One query matches all docs, the count of the conjunction is the count of the other query
	bqAssertCount(t, searcher, query, leaf, 1)

	query = search.NewBooleanQueryBuilder().
		Add(search.Instance, search.MUST).
		Add(longPointNewRangeQuery(t, "long", 1, 5), search.FILTER).
		Build()
	// One query matches all docs, the count of the conjunction is the count of the other query
	bqAssertCount(t, searcher, query, leaf, 1)

	closeAll()
}

// bqLongMultiDimRange renders LongPoint.newRangeQuery(String, long[], long[]),
// one of the unported static query factories of the document point classes.
func bqLongMultiDimRange(t *testing.T, field string, lower, upper []int64) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func TestBooleanQueryDisjunctionMatchesCount(t *testing.T) {
	reader, closeAll := bqTwoDocLongIndex(t, true)
	searcher := search.NewIndexSearcher(reader)
	leaf := mustLeaves(t, reader)[0]

	query := search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.SHOULD).
		Add(longPointNewExactQuery(t, "long", 3), search.SHOULD).
		Build()
	// Both queries match a single doc, BooleanWeight can't figure out the count of the disjunction
	bqAssertCount(t, searcher, query, leaf, -1)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "missing"), search.SHOULD).
		Add(longPointNewExactQuery(t, "long", 3), search.SHOULD).
		Build()
	// One query has a count of 0, the disjunction count is the other count
	bqAssertCount(t, searcher, query, leaf, 1)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.SHOULD).
		Add(longPointNewExactQuery(t, "long", 5), search.SHOULD).
		Build()
	// One query has a count of 0, the disjunction count is the other count
	bqAssertCount(t, searcher, query, leaf, 1)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.SHOULD).
		Add(longPointNewRangeQuery(t, "long", 0, 10), search.SHOULD).
		Build()
	// One query matches all docs, the count of the disjunction is the number of docs
	bqAssertCount(t, searcher, query, leaf, 2)

	query = search.NewBooleanQueryBuilder().
		Add(search.Instance, search.SHOULD).
		Add(longPointNewRangeQuery(t, "long", 1, 5), search.SHOULD).
		Build()
	// One query matches all docs, the count of the disjunction is the number of docs
	bqAssertCount(t, searcher, query, leaf, 2)

	lower := []int64{4, 5, 6}
	upper := []int64{9, 10, 11}
	unknownCountQuery := bqLongMultiDimRange(t, "long3dim", lower, upper)
	if util.AssertsEnabled() {
		if len(mustLeaves(t, reader)) != 1 {
			panic(util.NewAssertionError(""))
		}
		if bqWeightCount(t, searcher, unknownCountQuery, leaf) != -1 {
			panic(util.NewAssertionError(""))
		}
	}

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "xyz"), search.MUST).
		Add(unknownCountQuery, search.MUST_NOT).
		Add(search.Instance, search.MUST_NOT).
		Build()
	// count of the first MUST_NOT clause is unknown, but the second MUST_NOT clause matches all
	// docs
	bqAssertCount(t, searcher, query, leaf, 0)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "xyz"), search.MUST).
		Add(unknownCountQuery, search.MUST_NOT).
		Add(bqTermQuery("string", "abc"), search.MUST_NOT).
		Build()
	// count of the first MUST_NOT clause is unknown, though the second MUST_NOT clause matche one
	// doc, we can't figure out the number of
	// docs
	bqAssertCount(t, searcher, query, leaf, -1)

	// test pure disjunction
	query = search.NewBooleanQueryBuilder().
		Add(unknownCountQuery, search.SHOULD).
		Add(search.Instance, search.SHOULD).
		Build()
	// count of the first SHOULD clause is unknown, but the second SHOULD clause matches all docs
	bqAssertCount(t, searcher, query, leaf, 2)

	query = search.NewBooleanQueryBuilder().
		Add(unknownCountQuery, search.SHOULD).
		Add(bqTermQuery("string", "abc"), search.SHOULD).
		Build()
	// count of the first SHOULD clause is unknown, though the second SHOULD clause matche one doc,
	// we can't figure out the number of
	// docs
	bqAssertCount(t, searcher, query, leaf, -1)

	closeAll()
}

func TestBooleanQueryTwoClauseTermDisjunctionCountOptimization(t *testing.T) {
	largerTermCount := nextInt(11, 100)
	smallerTermCount := nextInt(1, (largerTermCount-1)/10)

	docContent := make([][]string, 0, largerTermCount+smallerTermCount)

	for i := 0; i < largerTermCount; i++ {
		docContent = append(docContent, []string{"large"})
	}

	for i := 0; i < smallerTermCount; i++ {
		docContent = append(docContent, []string{"small", "also small"})
	}

	dir := newDirectory()
	defer mustClose(t, dir)
	{
		iwc := newIndexWriterConfig()
		iwc.SetMergePolicy(newLogMergePolicy())
		w := mustNewIndexWriter(t, dir, iwc)
		for _, values := range docContent {
			doc := document.NewDocument()
			for _, value := range values {
				f, err := document.NewStringField("foo", value, false)
				if err != nil {
					t.Fatal(err)
				}
				doc.Add(f)
			}
			mustAddDocument(t, w, doc)
		}
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
		mustClose(t, w)
	}

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	countInvocations := []int{0}
	countingIndexSearcher := search.NewIndexSearcher(reader)
	countingIndexSearcher.SetCountOverride(func(super func(search.Query) (int, error), query search.Query) (int, error) {
		countInvocations[0]++
		return super(query)
	})

	{
		// Test no matches in either term
		countInvocations[0] = 0
		query := search.NewBooleanQueryBuilder().
			Add(bqTermQuery("foo", "no match"), search.SHOULD).
			Add(bqTermQuery("foo", "also no match"), search.SHOULD).
			Build()

		assertIntEquals(t, 0, mustCount(t, countingIndexSearcher, query))
		assertIntEquals(t, 3, countInvocations[0])
	}
	{
		// Test match no match in first term
		countInvocations[0] = 0
		query := search.NewBooleanQueryBuilder().
			Add(bqTermQuery("foo", "no match"), search.SHOULD).
			Add(bqTermQuery("foo", "small"), search.SHOULD).
			Build()

		assertIntEquals(t, smallerTermCount, mustCount(t, countingIndexSearcher, query))
		assertIntEquals(t, 3, countInvocations[0])
	}
	{
		// Test match no match in second term
		countInvocations[0] = 0
		query := search.NewBooleanQueryBuilder().
			Add(bqTermQuery("foo", "small"), search.SHOULD).
			Add(bqTermQuery("foo", "no match"), search.SHOULD).
			Build()

		assertIntEquals(t, smallerTermCount, mustCount(t, countingIndexSearcher, query))
		assertIntEquals(t, 3, countInvocations[0])
	}
	{
		// Test match in both terms that hits optimization threshold with small term first
		countInvocations[0] = 0

		query := search.NewBooleanQueryBuilder().
			Add(bqTermQuery("foo", "small"), search.SHOULD).
			Add(bqTermQuery("foo", "large"), search.SHOULD).
			Build()

		count := mustCount(t, countingIndexSearcher, query)

		assertIntEquals(t, largerTermCount+smallerTermCount, count)
		assertIntEquals(t, 4, countInvocations[0])

		if !query.IsTwoClausePureDisjunctionWithTerms() {
			t.Fatal("expected a two clause pure disjunction with terms")
		}
		queries, err := query.RewriteTwoClauseDisjunctionWithTermsForCount(countingIndexSearcher)
		if err != nil {
			t.Fatal(err)
		}
		assertIntEquals(t, len(queries), 3)
		assertIntEquals(t, smallerTermCount, mustCount(t, countingIndexSearcher, queries[0]))
		assertIntEquals(t, largerTermCount, mustCount(t, countingIndexSearcher, queries[1]))
	}
	{
		// Test match in both terms that hits optimization threshold with large term first
		countInvocations[0] = 0

		query := search.NewBooleanQueryBuilder().
			Add(bqTermQuery("foo", "large"), search.SHOULD).
			Add(bqTermQuery("foo", "small"), search.SHOULD).
			Build()

		count := mustCount(t, countingIndexSearcher, query)

		assertIntEquals(t, largerTermCount+smallerTermCount, count)
		assertIntEquals(t, 4, countInvocations[0])

		if !query.IsTwoClausePureDisjunctionWithTerms() {
			t.Fatal("expected a two clause pure disjunction with terms")
		}
		queries, err := query.RewriteTwoClauseDisjunctionWithTermsForCount(countingIndexSearcher)
		if err != nil {
			t.Fatal(err)
		}
		assertIntEquals(t, len(queries), 3)
		assertIntEquals(t, largerTermCount, mustCount(t, countingIndexSearcher, queries[0]))
		assertIntEquals(t, smallerTermCount, mustCount(t, countingIndexSearcher, queries[1]))
	}
	{
		// Test match in both terms that doesn't hit optimization threshold
		countInvocations[0] = 0
		query := search.NewBooleanQueryBuilder().
			Add(bqTermQuery("foo", "small"), search.SHOULD).
			Add(bqTermQuery("foo", "also small"), search.SHOULD).
			Build()

		count := mustCount(t, countingIndexSearcher, query)

		assertIntEquals(t, smallerTermCount, count)
		assertIntEquals(t, 3, countInvocations[0])
	}
}

// test BlockMaxMaxscoreScorer
func TestBooleanQueryDisjunctionTwoClausesMatchesCountAndScore(t *testing.T) {
	docContent := [][]string{
		{"A", "B"},      // 0
		{"A"},           // 1
		{},              // 2
		{"A", "B", "C"}, // 3
		{"B"},           // 4
		{"B", "C"},      // 5
	}

	// result sorted by score
	matchDocScore := [][]int{
		{0, 2 + 1},
		{3, 2 + 1},
		{1, 2},
		{4, 1},
		{5, 1},
	}

	dir := newDirectory()
	defer mustClose(t, dir)
	{
		iwc := newIndexWriterConfig()
		iwc.SetMergePolicy(newLogMergePolicy())
		w := mustNewIndexWriter(t, dir, iwc)
		for _, values := range docContent {
			doc := document.NewDocument()
			for _, value := range values {
				f, err := document.NewStringField("foo", value, false)
				if err != nil {
					t.Fatal(err)
				}
				doc.Add(f)
			}
			mustAddDocument(t, w, doc)
		}
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
		mustClose(t, w)
	}

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)

	query := search.NewBooleanQueryBuilder().
		Add(search.NewBoostQuery(search.NewConstantScoreQuery(bqTermQuery("foo", "A")), 2), search.SHOULD).
		Add(search.NewConstantScoreQuery(bqTermQuery("foo", "B")), search.SHOULD).
		Build()

	topDocs := mustSearch(t, searcher, query, 10)

	for i, scoreDoc := range topDocs.ScoreDocs {
		assertIntEquals(t, matchDocScore[i][0], scoreDoc.Doc)
		if float32(matchDocScore[i][1]) != scoreDoc.Score {
			t.Fatalf("score = %v, want %d", scoreDoc.Score, matchDocScore[i][1])
		}
	}
}

func TestBooleanQueryDisjunctionRandomClausesMatchesCount(t *testing.T) {
	numFieldValue := nextInt(1, 10)
	numDocsPerFieldValue := make([]int, numFieldValue)
	allDocsCount := 0

	for i := range numDocsPerFieldValue {
		numDocs := nextInt(10, 50)
		numDocsPerFieldValue[i] = numDocs
		allDocsCount += numDocs
	}

	dir := newDirectory()
	defer mustClose(t, dir)
	{
		iwc := newIndexWriterConfig()
		iwc.SetMergePolicy(newLogMergePolicy())
		w := mustNewIndexWriter(t, dir, iwc)
		for i := 0; i < numFieldValue; i++ {
			for j := 0; j < numDocsPerFieldValue[i]; j++ {
				doc := document.NewDocument()
				f, err := document.NewStringField("field", strconv.Itoa(i), false)
				if err != nil {
					t.Fatal(err)
				}
				doc.Add(f)
				mustAddDocument(t, w, doc)
			}
		}

		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
		mustClose(t, w)
	}

	matchedDocsCount := 0
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	searcher := newSearcher(t, reader)

	builder := search.NewBooleanQueryBuilder()

	for i := 0; i < numFieldValue; i++ {
		if random().Intn(2) == 0 {
			matchedDocsCount += numDocsPerFieldValue[i]
			builder.Add(bqTermQuery("field", strconv.Itoa(i)), search.SHOULD)
		}
	}

	query := builder.Build()

	topDocs := mustSearch(t, searcher, query, allDocsCount)
	assertIntEquals(t, matchedDocsCount, len(topDocs.ScoreDocs))
}

func TestBooleanQueryProhibitedMatchesCount(t *testing.T) {
	reader, closeAll := bqTwoDocLongIndex(t, false)
	searcher := search.NewIndexSearcher(reader)
	leaf := mustLeaves(t, reader)[0]

	query := search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.MUST).
		Add(longPointNewExactQuery(t, "long", 3), search.MUST_NOT).
		Build()
	// Both queries match a single doc, BooleanWeight can't figure out the count of the query
	bqAssertCount(t, searcher, query, leaf, -1)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "missing"), search.MUST).
		Add(longPointNewExactQuery(t, "long", 3), search.MUST_NOT).
		Build()
	// the positive clause doesn't match any docs, so the overall query doesn't either
	bqAssertCount(t, searcher, query, leaf, 0)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.MUST).
		Add(longPointNewExactQuery(t, "long", 5), search.MUST_NOT).
		Build()
	// the negative clause doesn't match any docs, so the overall count is the count of the positive
	// clause
	bqAssertCount(t, searcher, query, leaf, 1)

	query = search.NewBooleanQueryBuilder().
		Add(bqTermQuery("string", "abc"), search.MUST).
		Add(longPointNewRangeQuery(t, "long", 0, 10), search.MUST_NOT).
		Build()
	// the negative clause matches all docs, so the query doesn't match any docs
	bqAssertCount(t, searcher, query, leaf, 0)

	query = search.NewBooleanQueryBuilder().
		Add(longPointNewRangeQuery(t, "long", 0, 10), search.MUST).
		Add(bqTermQuery("string", "abc"), search.MUST_NOT).
		Build()
	// The positive clause matches all docs, so we can subtract the number of matches of the
	// negative clause
	bqAssertCount(t, searcher, query, leaf, 1)

	closeAll()
}

func TestBooleanQueryRandomBooleanQueryMatchesCount(t *testing.T) {
	reader, closeAll := bqTwoDocLongIndex(t, false)
	defer closeAll()
	searcher := search.NewIndexSearcher(reader)
	for iter := 0; iter < 1000; iter++ {
		numClauses := nextInt(2, 5)
		builder := search.NewBooleanQueryBuilder()
		numShouldClauses := 0
		for i := 0; i < numClauses; i++ {
			var query search.Query
			switch random().Intn(6) {
			case 0:
				query = bqTermQuery("string", "abc")
			case 1:
				query = longPointNewExactQuery(t, "long", 3)
			case 2:
				query = bqTermQuery("string", "missing")
			case 3:
				query = longPointNewExactQuery(t, "long", 5)
			case 4:
				query = search.Instance
			default:
				query = longPointNewRangeQuery(t, "long", 0, 10)
			}
			occur := bqOccurValues[random().Intn(len(bqOccurValues))]
			if occur == search.SHOULD {
				numShouldClauses++
			}
			builder.Add(query, occur)
		}
		builder.SetMinimumNumberShouldMatch(nextInt(0, numShouldClauses))
		booleanQuery := builder.Build()
		expected, err := search.SearchWithCollectorManager(searcher, booleanQuery, testsearch.DummyTotalHitCountCollectorCreateManager())
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		assertIntEquals(t, expected, mustCount(t, searcher, booleanQuery))
	}
}

func TestBooleanQueryToString(t *testing.T) {
	bq := search.NewBooleanQueryBuilder()
	bq.Add(bqTermQuery("field", "a"), search.SHOULD)
	bq.Add(bqTermQuery("field", "b"), search.MUST)
	bq.Add(bqTermQuery("field", "c"), search.MUST_NOT)
	bq.Add(bqTermQuery("field", "d"), search.FILTER)
	if got := bq.Build().ToString("field"); got != "a +b -c #d" {
		t.Fatalf("toString = %q, want %q", got, "a +b -c #d")
	}
}

// bqExpectingVisitor renders the anonymous QueryVisitor of testQueryVisitor.
type bqExpectingVisitor struct {
	t          *testing.T
	a, b, c, d *index.Term
	expected   *index.Term
}

func (v *bqExpectingVisitor) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	switch occur {
	case search.SHOULD:
		v.expected = v.a
	case search.MUST:
		v.expected = v.b
	case search.FILTER:
		v.expected = v.c
	case search.MUST_NOT:
		v.expected = v.d
	default:
		panic("IllegalStateException")
	}
	return v
}

func (v *bqExpectingVisitor) ConsumeTerms(query search.Query, terms ...*index.Term) {
	if !v.expected.Equals(terms[0]) {
		v.t.Fatalf("term = %v, want %v", terms[0], v.expected)
	}
}

// ConsumeTermsMatching, VisitLeaf and AcceptField keep the QueryVisitor
// defaults.
func (v *bqExpectingVisitor) ConsumeTermsMatching(query search.Query, field string, automaton func() search.ByteRunAutomaton) {
}

func (v *bqExpectingVisitor) VisitLeaf(query search.Query) {}

func (v *bqExpectingVisitor) AcceptField(field string) bool { return true }

func TestBooleanQueryQueryVisitor(t *testing.T) {
	a := index.NewTerm("f", "a")
	b := index.NewTerm("f", "b")
	c := index.NewTerm("f", "c")
	d := index.NewTerm("f", "d")
	bqBuilder := search.NewBooleanQueryBuilder()
	bqBuilder.Add(search.NewTermQuery(a), search.SHOULD)
	bqBuilder.Add(search.NewTermQuery(b), search.MUST)
	bqBuilder.Add(search.NewTermQuery(c), search.FILTER)
	bqBuilder.Add(search.NewTermQuery(d), search.MUST_NOT)
	bq := bqBuilder.Build()

	bq.Visit(&bqExpectingVisitor{t: t, a: a, b: b, c: c, d: d})
}

func TestBooleanQueryClauseSetsImmutability(t *testing.T) {
	a := index.NewTerm("f", "a")
	b := index.NewTerm("f", "b")
	c := index.NewTerm("f", "c")
	d := index.NewTerm("f", "d")
	bqBuilder := search.NewBooleanQueryBuilder()
	bqBuilder.Add(search.NewTermQuery(a), search.SHOULD)
	bqBuilder.Add(search.NewTermQuery(a), search.SHOULD)
	bqBuilder.Add(search.NewTermQuery(b), search.MUST)
	bqBuilder.Add(search.NewTermQuery(b), search.MUST)
	bqBuilder.Add(search.NewTermQuery(c), search.FILTER)
	bqBuilder.Add(search.NewTermQuery(c), search.FILTER)
	bqBuilder.Add(search.NewTermQuery(d), search.MUST_NOT)
	bqBuilder.Add(search.NewTermQuery(d), search.MUST_NOT)
	bq := bqBuilder.Build()
	// should and must are not deduplicated
	assertIntEquals(t, 2, len(bq.GetClauses(search.SHOULD)))
	assertIntEquals(t, 2, len(bq.GetClauses(search.MUST)))
	// filter and must not are deduplicated
	assertIntEquals(t, 1, len(bq.GetClauses(search.FILTER)))
	assertIntEquals(t, 1, len(bq.GetClauses(search.MUST_NOT)))
	// check immutability: Java's unmodifiable views throw
	// UnsupportedOperationException on add; a Go slice cannot refuse an
	// append, so the rendering asserts that appending to the returned slice
	// leaves the query unchanged.
	for _, occur := range bqOccurValues {
		before := len(bq.GetClauses(occur))
		_ = append(bq.GetClauses(occur), search.MatchNoDocsQueryInstance)
		assertIntEquals(t, before, len(bq.GetClauses(occur)))
	}
	clauses := bq.Clauses()
	before := len(clauses)
	grown := append(clauses, search.NewBooleanClause(search.MatchNoDocsQueryInstance, search.SHOULD))
	assertIntEquals(t, before, len(bq.Clauses()))
	if &grown[0] == &bq.Clauses()[0] {
		t.Fatal("appending to clauses() wrote into the query")
	}
}

// assertIntEquals renders assertEquals(long, long).
func assertIntEquals(t *testing.T, expected, actual int) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected:<%d> but was:<%d>", expected, actual)
	}
}
