// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestMultiTermConstantScore.java
// (Apache Lucene 10.5.0). The class extends TestBaseRangeFilter
// (test_base_range_filter_test.go); testPad is inherited and runs there.
// The @BeforeClass states of both classes are rebuilt per test (mtcsBeforeClass).

package search_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// mtcsScoreCompThresh renders SCORE_COMP_THRESH: threshold for comparing floats.
const mtcsScoreCompThresh = 1e-6

// mtcsConstantScoreRewrites renders CONSTANT_SCORE_REWRITES.
var mtcsConstantScoreRewrites = []search.RewriteMethod{search.ConstantScoreRewrite, search.ConstantScoreBlendedRewrite}

// mtcsClass renders the static fields of TestMultiTermConstantScore together
// with the inherited TestBaseRangeFilter state.
type mtcsClass struct {
	*brfClass
	reader *index.DirectoryReader
}

// mtcsBeforeClass renders beforeClass() (after the superclass one).
func mtcsBeforeClass(t *testing.T) *mtcsClass {
	t.Helper()
	c := &mtcsClass{brfClass: brfBeforeClass(t)}
	data := []*string{
		mtcsStr("A 1 2 3 4 5 6"),
		mtcsStr("Z       4 5 6"),
		nil,
		mtcsStr("B   2   4 5 6"),
		mtcsStr("Y     3   5 6"),
		nil,
		mtcsStr("C     3     6"),
		mtcsStr("X       4 5 6"),
	}

	small := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, false, 0, nil, true))
	iwc.SetMergePolicy(newLogMergePolicy())
	writer := newRandomIndexWriterWithConfig(t, small, iwc)

	customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType.SetTokenized(false)
	for i := 0; i < len(data); i++ {
		doc := document.NewDocument()
		doc.Add(newField(t, "id", strconv.Itoa(i), customType)) // Field.Keyword("id",String.valueOf(i)));
		doc.Add(newField(t, "all", "all", customType))          // Field.Keyword("all","all"));
		if nil != data[i] {
			doc.Add(newTextField(t, "data", *data[i], true)) // Field.Text("data",data[i]));
		}
		mustAddDocument(t, writer, doc)
	}

	c.reader = mustGetReader(t, writer)
	mustClose(t, writer)
	t.Cleanup(func() {
		mustClose(t, c.reader, small)
	})
	return c
}

func mtcsStr(s string) *string { return &s }

// csrq is a macro for readability; nil l or h renders a null bound.
func csrq(t *testing.T, f string, l, h *string, il, ih bool, method search.RewriteMethod) search.Query {
	var lower, upper *util.BytesRef
	if l != nil {
		lower = util.NewBytesRef([]byte(*l))
	}
	if h != nil {
		upper = util.NewBytesRef([]byte(*h))
	}
	query := search.NewTermRangeQueryWithRewriteMethod(f, lower, upper, il, ih, method)
	if testing.Verbose() {
		t.Logf("TEST: query=%v method=%v", query, method)
	}
	return query
}

// cspq is a macro for readability.
func cspq(prefix *index.Term, method search.RewriteMethod) search.Query {
	return search.NewPrefixQueryWithRewriteMethod(prefix, method)
}

// cswcq is a macro for readability.
func cswcq(wild *index.Term, method search.RewriteMethod) search.Query {
	return search.NewWildcardQueryWithRewrite(wild, automaton.DefaultDeterminizeWorkLimit, method)
}

func mtcsAssertEquals(t *testing.T, m string, e, a int) {
	t.Helper()
	if e != a {
		t.Fatalf("%s: expected %d, got %d", m, e, a)
	}
}

func mtcsAssertScore(t *testing.T, m string, e, a float32) {
	t.Helper()
	if math.Abs(float64(e-a)) > mtcsScoreCompThresh {
		t.Fatalf("%s: expected %v, got %v", m, e, a)
	}
}

func TestMultiTermConstantScoreBasics(t *testing.T) {
	mtcsBeforeClass(t)
	for _, rw := range mtcsConstantScoreRewrites {
		queryUtilsCheck(t, csrq(t, "data", mtcsStr("1"), mtcsStr("6"), brfT, brfT, rw))
		queryUtilsCheck(t, csrq(t, "data", mtcsStr("A"), mtcsStr("Z"), brfT, brfT, rw))
		queryUtilsCheckUnequal(t, csrq(t, "data", mtcsStr("1"), mtcsStr("6"), brfT, brfT, rw), csrq(t, "data", mtcsStr("A"), mtcsStr("Z"), brfT, brfT, rw))

		queryUtilsCheck(t, cspq(index.NewTerm("data", "p*u?"), rw))
		queryUtilsCheckUnequal(t, cspq(index.NewTerm("data", "pre*"), rw), cspq(index.NewTerm("data", "pres*"), rw))

		queryUtilsCheck(t, cswcq(index.NewTerm("data", "p"), rw))
		queryUtilsCheckUnequal(t, cswcq(index.NewTerm("data", "pre*n?t"), rw), cswcq(index.NewTerm("data", "pr*t?j"), rw))
	}
}

func TestMultiTermConstantScoreEqualScores(t *testing.T) {
	c := mtcsBeforeClass(t)
	// NOTE: uses index build in *this* setUp

	s := newSearcher(t, c.reader)

	// some hits match more terms then others, score should be the same

	result := mustSearch(t, s, csrq(t, "data", mtcsStr("1"), mtcsStr("6"), brfT, brfT, search.ConstantScoreBlendedRewrite), 1000).ScoreDocs
	numHits := len(result)
	mtcsAssertEquals(t, "wrong number of results", 6, numHits)
	score := result[0].Score
	for i := 1; i < numHits; i++ {
		mtcsAssertScore(t, "score for "+strconv.Itoa(i)+" was not the same", score, result[i].Score)
	}

	result = mustSearch(t, s, csrq(t, "data", mtcsStr("1"), mtcsStr("6"), brfT, brfT, search.ConstantScoreBooleanRewrite), 1000).ScoreDocs
	numHits = len(result)
	mtcsAssertEquals(t, "wrong number of results", 6, numHits)
	for i := 0; i < numHits; i++ {
		mtcsAssertScore(t, "score for "+strconv.Itoa(i)+" was not the same", score, result[i].Score)
	}

	result = mustSearch(t, s, csrq(t, "data", mtcsStr("1"), mtcsStr("6"), brfT, brfT, search.ConstantScoreRewrite), 1000).ScoreDocs
	numHits = len(result)
	mtcsAssertEquals(t, "wrong number of results", 6, numHits)
	for i := 0; i < numHits; i++ {
		mtcsAssertScore(t, "score for "+strconv.Itoa(i)+" was not the same", score, result[i].Score)
	}
}

// Test for LUCENE-5245: Empty MTQ rewrites should have a consistent norm, so
// always need to return a CSQ!
func TestMultiTermConstantScoreEqualScoresWhenNoHits(t *testing.T) {
	c := mtcsBeforeClass(t)
	// NOTE: uses index build in *this* setUp

	s := newSearcher(t, c.reader)

	dummyTerm := search.NewTermQuery(index.NewTerm("data", "1"))

	bq := search.NewBooleanQueryBuilder()
	bq.Add(dummyTerm, search.SHOULD)                                                                                   // hits one doc
	bq.Add(csrq(t, "data", mtcsStr("#"), mtcsStr("#"), brfT, brfT, search.ConstantScoreBlendedRewrite), search.SHOULD) // hits no docs
	result := mustSearch(t, s, bq.Build(), 1000).ScoreDocs
	numHits := len(result)
	mtcsAssertEquals(t, "wrong number of results", 1, numHits)
	score := result[0].Score
	for i := 1; i < numHits; i++ {
		mtcsAssertScore(t, "score for "+strconv.Itoa(i)+" was not the same", score, result[i].Score)
	}

	bq = search.NewBooleanQueryBuilder()
	bq.Add(dummyTerm, search.SHOULD)                                                                                   // hits one doc
	bq.Add(csrq(t, "data", mtcsStr("#"), mtcsStr("#"), brfT, brfT, search.ConstantScoreBooleanRewrite), search.SHOULD) // hits no docs
	result = mustSearch(t, s, bq.Build(), 1000).ScoreDocs
	numHits = len(result)
	mtcsAssertEquals(t, "wrong number of results", 1, numHits)
	for i := 0; i < numHits; i++ {
		mtcsAssertScore(t, "score for "+strconv.Itoa(i)+" was not the same", score, result[i].Score)
	}

	bq = search.NewBooleanQueryBuilder()
	bq.Add(dummyTerm, search.SHOULD)                                                                            // hits one doc
	bq.Add(csrq(t, "data", mtcsStr("#"), mtcsStr("#"), brfT, brfT, search.ConstantScoreRewrite), search.SHOULD) // hits no docs
	result = mustSearch(t, s, bq.Build(), 1000).ScoreDocs
	numHits = len(result)
	mtcsAssertEquals(t, "wrong number of results", 1, numHits)
	for i := 0; i < numHits; i++ {
		mtcsAssertScore(t, "score for "+strconv.Itoa(i)+" was not the same", score, result[i].Score)
	}
}

func TestMultiTermConstantScoreBooleanOrderUnAffected(t *testing.T) {
	c := mtcsBeforeClass(t)
	// NOTE: uses index build in *this* setUp

	s := newSearcher(t, c.reader)

	for _, rw := range mtcsConstantScoreRewrites {

		// first do a regular TermRangeQuery which uses term expansion so
		// docs with more terms in range get higher scores

		rq := search.NewStringRangeWithRewriteMethod("data", "1", "4", brfT, brfT, rw)

		expected := mustSearch(t, s, rq, 1000).ScoreDocs
		numHits := len(expected)

		// now do a boolean where which also contains a
		// ConstantScoreRangeQuery and make sure the order is the same

		q := search.NewBooleanQueryBuilder()
		q.Add(rq, search.MUST)                                                          // T, F);
		q.Add(csrq(t, "data", mtcsStr("1"), mtcsStr("6"), brfT, brfT, rw), search.MUST) // T, F);

		actual := mustSearch(t, s, q.Build(), 1000).ScoreDocs

		mtcsAssertEquals(t, "wrong number of hits", numHits, len(actual))
		for i := 0; i < numHits; i++ {
			mtcsAssertEquals(t, "mismatch in docid for hit#"+strconv.Itoa(i), expected[i].Doc, actual[i].Doc)
		}
	}
}

func TestMultiTermConstantScoreRangeQueryID(t *testing.T) {
	c := mtcsBeforeClass(t)
	// NOTE: uses index build in *super* setUp

	reader := c.signedIndexReader
	s := newSearcher(t, reader)

	if testing.Verbose() {
		t.Logf("TEST: reader=%v", reader)
	}

	maxID := c.maxID
	medID := (maxID - brfMinID) / 2

	minIP := mtcsStr(padRangeFilter(brfMinID))
	maxIP := mtcsStr(padRangeFilter(int32(maxID)))
	medIP := mtcsStr(padRangeFilter(int32(medID)))

	numDocs := reader.NumDocs()

	mtcsAssertEquals(t, "num of docs", numDocs, 1+maxID-brfMinID)

	T, F := brfT, brfF
	count := func(q search.Query) int { return len(mustSearch(t, s, q, numDocs).ScoreDocs) }
	for _, rw := range mtcsConstantScoreRewrites {

		// test id, bounded on both ends

		mtcsAssertEquals(t, "find all", numDocs, count(csrq(t, "id", minIP, maxIP, T, T, rw)))
		mtcsAssertEquals(t, "all but last", numDocs-1, count(csrq(t, "id", minIP, maxIP, T, F, rw)))
		mtcsAssertEquals(t, "all but first", numDocs-1, count(csrq(t, "id", minIP, maxIP, F, T, rw)))
		mtcsAssertEquals(t, "all but ends", numDocs-2, count(csrq(t, "id", minIP, maxIP, F, F, rw)))
		mtcsAssertEquals(t, "med and up", 1+maxID-medID, count(csrq(t, "id", medIP, maxIP, T, T, rw)))
		mtcsAssertEquals(t, "up to med", 1+medID-brfMinID, count(csrq(t, "id", minIP, medIP, T, T, rw)))

		// unbounded id

		mtcsAssertEquals(t, "min and up", numDocs, count(csrq(t, "id", minIP, nil, T, F, rw)))
		mtcsAssertEquals(t, "max and down", numDocs, count(csrq(t, "id", nil, maxIP, F, T, rw)))
		mtcsAssertEquals(t, "not min, but up", numDocs-1, count(csrq(t, "id", minIP, nil, F, F, rw)))
		mtcsAssertEquals(t, "not max, but down", numDocs-1, count(csrq(t, "id", nil, maxIP, F, F, rw)))
		mtcsAssertEquals(t, "med and up, not max", maxID-medID, count(csrq(t, "id", medIP, maxIP, T, F, rw)))
		mtcsAssertEquals(t, "not min, up to med", medID-brfMinID, count(csrq(t, "id", minIP, medIP, F, T, rw)))

		// very small sets

		mtcsAssertEquals(t, "min,min,F,F", 0, count(csrq(t, "id", minIP, minIP, F, F, rw)))
		mtcsAssertEquals(t, "med,med,F,F", 0, count(csrq(t, "id", medIP, medIP, F, F, rw)))
		mtcsAssertEquals(t, "max,max,F,F", 0, count(csrq(t, "id", maxIP, maxIP, F, F, rw)))
		mtcsAssertEquals(t, "min,min,T,T", 1, count(csrq(t, "id", minIP, minIP, T, T, rw)))
		mtcsAssertEquals(t, "nul,min,F,T", 1, count(csrq(t, "id", nil, minIP, F, T, rw)))
		mtcsAssertEquals(t, "max,max,T,T", 1, count(csrq(t, "id", maxIP, maxIP, T, T, rw)))
		mtcsAssertEquals(t, "max,nul,T,T", 1, count(csrq(t, "id", maxIP, nil, T, F, rw)))
		mtcsAssertEquals(t, "med,med,T,T", 1, count(csrq(t, "id", medIP, medIP, T, T, rw)))
	}
}

func TestMultiTermConstantScoreRangeQueryRand(t *testing.T) {
	c := mtcsBeforeClass(t)
	// NOTE: uses index build in *super* setUp

	reader := c.signedIndexReader
	s := newSearcher(t, reader)

	minRP := mtcsStr(padRangeFilter(c.signedIndexDir.minR))
	maxRP := mtcsStr(padRangeFilter(c.signedIndexDir.maxR))

	numDocs := reader.NumDocs()

	mtcsAssertEquals(t, "num of docs", numDocs, 1+c.maxID-brfMinID)

	T, F := brfT, brfF
	count := func(q search.Query) int { return len(mustSearch(t, s, q, numDocs).ScoreDocs) }
	for _, rw := range mtcsConstantScoreRewrites {

		// test extremes, bounded on both ends

		mtcsAssertEquals(t, "find all", numDocs, count(csrq(t, "rand", minRP, maxRP, T, T, rw)))
		mtcsAssertEquals(t, "all but biggest", numDocs-1, count(csrq(t, "rand", minRP, maxRP, T, F, rw)))
		mtcsAssertEquals(t, "all but smallest", numDocs-1, count(csrq(t, "rand", minRP, maxRP, F, T, rw)))
		mtcsAssertEquals(t, "all but extremes", numDocs-2, count(csrq(t, "rand", minRP, maxRP, F, F, rw)))

		// unbounded

		mtcsAssertEquals(t, "smallest and up", numDocs, count(csrq(t, "rand", minRP, nil, T, F, rw)))
		mtcsAssertEquals(t, "biggest and down", numDocs, count(csrq(t, "rand", nil, maxRP, F, T, rw)))
		mtcsAssertEquals(t, "not smallest, but up", numDocs-1, count(csrq(t, "rand", minRP, nil, F, F, rw)))
		mtcsAssertEquals(t, "not biggest, but down", numDocs-1, count(csrq(t, "rand", nil, maxRP, F, F, rw)))

		// very small sets

		mtcsAssertEquals(t, "min,min,F,F", 0, count(csrq(t, "rand", minRP, minRP, F, F, rw)))
		mtcsAssertEquals(t, "max,max,F,F", 0, count(csrq(t, "rand", maxRP, maxRP, F, F, rw)))
		mtcsAssertEquals(t, "min,min,T,T", 1, count(csrq(t, "rand", minRP, minRP, T, T, rw)))
		mtcsAssertEquals(t, "nul,min,F,T", 1, count(csrq(t, "rand", nil, minRP, F, T, rw)))
		mtcsAssertEquals(t, "max,max,T,T", 1, count(csrq(t, "rand", maxRP, maxRP, T, T, rw)))
		mtcsAssertEquals(t, "max,nul,T,T", 1, count(csrq(t, "rand", maxRP, nil, T, F, rw)))
	}
}
