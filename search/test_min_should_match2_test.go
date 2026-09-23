// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestMinShouldMatch2.java
// (Apache Lucene 10.5.0): tests the scorers for minShouldMatch. The @Nightly
// testAdvanceVaryingNumberOfTerms lives in test_min_should_match2_monster_test.go
// behind the gocene_monsters build tag. The static @BeforeClass state is rebuilt
// per test (msm2BeforeClass).

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

var (
	msm2AlwaysTerms = []string{"a"}
	msm2CommonTerms = []string{"b", "c", "d"}
	msm2MediumTerms = []string{"e", "f", "g"}
	msm2RareTerms   = []string{
		"h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
	}
)

// msm2Mode renders the enum Mode.
type msm2Mode int

const (
	msm2Scorer msm2Mode = iota
	msm2BulkScorer
	msm2DocValues
)

// msm2Class renders the static fields of TestMinShouldMatch2.
type msm2Class struct {
	reader   index.LeafReader
	ctx      *index.LeafReaderContext
	searcher *search.IndexSearcher
}

// msm2BeforeClass renders beforeClass(); afterClass is registered with t.Cleanup.
func msm2BeforeClass(t *testing.T) *msm2Class {
	t.Helper()
	c := &msm2Class{}
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	numDocs := atLeast(300)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		msm2AddSome(t, doc, msm2AlwaysTerms)

		if random().Intn(100) < 90 {
			msm2AddSome(t, doc, msm2CommonTerms)
		}
		if random().Intn(100) < 50 {
			msm2AddSome(t, doc, msm2MediumTerms)
		}
		if random().Intn(100) < 10 {
			msm2AddSome(t, doc, msm2RareTerms)
		}
		mustAddDocument(t, iw, doc)
	}
	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, iw)
	r := mustOpenDirectoryReader(t, dir)
	t.Cleanup(func() { mustClose(t, r, dir) })
	c.reader = mustLeaves(t, r)[0].LeafReader() // getOnlyLeafReader(r)
	if n := len(mustLeaves(t, r)); n != 1 {
		t.Fatalf("reader has %d segments instead of exactly one", n)
	}
	c.searcher = search.NewIndexSearcher(c.reader)
	c.searcher.SetSimilarity(search.NewClassicSimilarity())
	c.ctx = mustLeaves(t, c.searcher.GetIndexReader())[0] // reader.getContext()
	return c
}

// msm2AddSome renders the private static addSome(Document, String[]).
func msm2AddSome(t *testing.T, doc *document.Document, values []string) {
	t.Helper()
	list := values // Arrays.asList(values): a view shuffled in place
	random().Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	howMany := nextInt(1, len(list))
	for i := 0; i < howMany; i++ {
		doc.Add(mustStringField(t, "field", list[i], false))
		dv, err := document.NewSortedSetDocValuesField("dv", [][]byte{[]byte(list[i])})
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(dv)
	}
}

// scorer renders the private scorer(String[], int, Mode).
func (c *msm2Class) scorer(t *testing.T, values []string, minShouldMatch int, mode msm2Mode) search.Scorer {
	t.Helper()
	bq := search.NewBooleanQueryBuilder()
	for _, value := range values {
		bq.Add(search.NewTermQuery(index.NewTerm("field", value)), search.SHOULD)
	}
	bq.SetMinimumNumberShouldMatch(minShouldMatch)

	weight := mustCreateWeight(t, c.searcher, mustRewrite(t, c.searcher, bq.Build()), search.COMPLETE, 1).(*search.BooleanWeight)

	switch mode {
	case msm2DocValues:
		return newSlowMinShouldMatchScorer(t, weight, c.reader, c.searcher)
	case msm2Scorer:
		s, err := weight.Scorer(c.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if s == nil {
			return nil
		}
		return s
	case msm2BulkScorer:
		ss, err := weight.ScorerSupplier(c.ctx)
		if err != nil {
			t.Fatal(err)
		}
		bulkScorer, err := ss.BulkScorer()
		if err != nil {
			t.Fatal(err)
		}
		if bulkScorer == nil {
			if s, _ := weight.Scorer(c.ctx); s != nil {
				panic(util.NewAssertionError("BooleanScorer should be applicable for this query"))
			}
			return nil
		}
		return testsearch.NewBulkScorerWrapperScorer(bulkScorer, nextInt(1, 100))
	default:
		panic(util.NewAssertionError(nil))
	}
}

// assertNext renders the private assertNext(Scorer, Scorer).
func msm2AssertNext(t *testing.T, expected, actual search.Scorer) {
	t.Helper()
	if actual == nil {
		doc, err := expected.Iterator().NextDoc()
		if err != nil {
			t.Fatal(err)
		}
		assertIntEquals(t, search.NO_MORE_DOCS, doc)
		return
	}
	expectedIt := expected.Iterator()
	actualIt := actual.Iterator()
	for {
		doc, err := expectedIt.NextDoc()
		if err != nil {
			t.Fatal(err)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		got, err := actualIt.NextDoc()
		if err != nil {
			t.Fatal(err)
		}
		assertIntEquals(t, doc, got)
		msm2AssertScores(t, expected, actual)
	}
	got, err := actualIt.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	assertIntEquals(t, search.NO_MORE_DOCS, got)
}

func msm2AssertScores(t *testing.T, expected, actual search.Scorer) {
	t.Helper()
	expectedScore, err := expected.Score()
	if err != nil {
		t.Fatal(err)
	}
	actualScore, err := actual.Score()
	if err != nil {
		t.Fatal(err)
	}
	if expectedScore != actualScore {
		t.Fatalf("score: expected %v, got %v", expectedScore, actualScore)
	}
}

// assertAdvance renders the private assertAdvance(Scorer, Scorer, int).
func msm2AssertAdvance(t *testing.T, expected, actual search.Scorer, amount int) {
	t.Helper()
	if actual == nil {
		doc, err := expected.Iterator().NextDoc()
		if err != nil {
			t.Fatal(err)
		}
		assertIntEquals(t, search.NO_MORE_DOCS, doc)
		return
	}
	expectedIt := expected.Iterator()
	actualIt := actual.Iterator()
	prevDoc := 0
	for {
		doc, err := expectedIt.Advance(prevDoc + amount)
		if err != nil {
			t.Fatal(err)
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		got, err := actualIt.Advance(prevDoc + amount)
		if err != nil {
			t.Fatal(err)
		}
		assertIntEquals(t, doc, got)
		msm2AssertScores(t, expected, actual)
		prevDoc = doc
	}
	got, err := actualIt.Advance(prevDoc + amount)
	if err != nil {
		t.Fatal(err)
	}
	assertIntEquals(t, search.NO_MORE_DOCS, got)
}

// simple test for next(): minShouldMatch=2 on 3 terms (one common, one medium, one rare)
func TestMinShouldMatch2NextCMR2(t *testing.T) {
	c := msm2BeforeClass(t)
	for common := 0; common < len(msm2CommonTerms); common++ {
		for medium := 0; medium < len(msm2MediumTerms); medium++ {
			for rare := 0; rare < len(msm2RareTerms); rare++ {
				terms := []string{msm2CommonTerms[common], msm2MediumTerms[medium], msm2RareTerms[rare]}
				expected := c.scorer(t, terms, 2, msm2DocValues)
				actual := c.scorer(t, terms, 2, msm2Scorer)
				msm2AssertNext(t, expected, actual)

				expected = c.scorer(t, terms, 2, msm2DocValues)
				actual = c.scorer(t, terms, 2, msm2BulkScorer)
				msm2AssertNext(t, expected, actual)
			}
		}
	}
}

// simple test for advance(): minShouldMatch=2 on 3 terms (one common, one medium, one rare)
func TestMinShouldMatch2AdvanceCMR2(t *testing.T) {
	c := msm2BeforeClass(t)
	for amount := 25; amount < 200; amount += 25 {
		for common := 0; common < len(msm2CommonTerms); common++ {
			for medium := 0; medium < len(msm2MediumTerms); medium++ {
				for rare := 0; rare < len(msm2RareTerms); rare++ {
					terms := []string{msm2CommonTerms[common], msm2MediumTerms[medium], msm2RareTerms[rare]}
					expected := c.scorer(t, terms, 2, msm2DocValues)
					actual := c.scorer(t, terms, 2, msm2Scorer)
					msm2AssertAdvance(t, expected, actual, amount)

					expected = c.scorer(t, terms, 2, msm2DocValues)
					actual = c.scorer(t, terms, 2, msm2BulkScorer)
					msm2AssertAdvance(t, expected, actual, amount)
				}
			}
		}
	}
}

func msm2AllTermsList() []string {
	var termsList []string
	termsList = append(termsList, msm2CommonTerms...)
	termsList = append(termsList, msm2MediumTerms...)
	termsList = append(termsList, msm2RareTerms...)
	return termsList
}

// test next with giant bq of all terms with varying minShouldMatch
func TestMinShouldMatch2NextAllTerms(t *testing.T) {
	c := msm2BeforeClass(t)
	terms := msm2AllTermsList()

	for minNrShouldMatch := 1; minNrShouldMatch < len(terms); minNrShouldMatch++ {
		expected := c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
		actual := c.scorer(t, terms, minNrShouldMatch, msm2Scorer)
		msm2AssertNext(t, expected, actual)

		expected = c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
		actual = c.scorer(t, terms, minNrShouldMatch, msm2BulkScorer)
		msm2AssertNext(t, expected, actual)
	}
}

// test advance with giant bq of all terms with varying minShouldMatch
func TestMinShouldMatch2AdvanceAllTerms(t *testing.T) {
	c := msm2BeforeClass(t)
	terms := msm2AllTermsList()

	for amount := 25; amount < 200; amount += 25 {
		for minNrShouldMatch := 1; minNrShouldMatch < len(terms); minNrShouldMatch++ {
			expected := c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
			actual := c.scorer(t, terms, minNrShouldMatch, msm2Scorer)
			msm2AssertAdvance(t, expected, actual, amount)

			expected = c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
			actual = c.scorer(t, terms, minNrShouldMatch, msm2BulkScorer)
			msm2AssertAdvance(t, expected, actual, amount)
		}
	}
}

// test next with varying numbers of terms with varying minShouldMatch
func TestMinShouldMatch2NextVaryingNumberOfTerms(t *testing.T) {
	c := msm2BeforeClass(t)
	termsList := msm2AllTermsList()
	random().Shuffle(len(termsList), func(i, j int) { termsList[i], termsList[j] = termsList[j], termsList[i] })
	for numTerms := 2; numTerms <= len(termsList); numTerms++ {
		terms := append([]string(nil), termsList[:numTerms]...)
		for minNrShouldMatch := 1; minNrShouldMatch < len(terms); minNrShouldMatch++ {
			expected := c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
			actual := c.scorer(t, terms, minNrShouldMatch, msm2Scorer)
			msm2AssertNext(t, expected, actual)

			expected = c.scorer(t, terms, minNrShouldMatch, msm2DocValues)
			actual = c.scorer(t, terms, minNrShouldMatch, msm2BulkScorer)
			msm2AssertNext(t, expected, actual)
		}
	}
}

// msm2SortedSetDocValuesLookup names the SortedSetDocValues members the slow
// scorer needs beyond Gocene's spi.SortedSetDocValues.
type msm2SortedSetDocValuesLookup interface {
	GetValueCount() int
	LookupTerm(key *util.BytesRef) (int, error)
}

// sortedSetDocValuesLookupBlocker names the missing SortedSetDocValues members.
const sortedSetDocValuesLookupBlocker = "requires org.apache.lucene.index.SortedSetDocValues#getValueCount() and " +
	"#lookupTerm(BytesRef) on the codec's SortedSetDocValues (not ported)"

// slowMinShouldMatchScorer renders the static class SlowMinShouldMatchScorer:
// a slow min-should match scorer that uses a docvalues field. later, we can
// make debugging easier as it can record the set of ords it currently matched
// and e.g. print out their values and so on for the document.
type slowMinShouldMatchScorer struct {
	search.BaseScorer
	currentDoc     int // current docid
	currentMatched int // current number of terms matched

	dv     index.SortedSetDocValues
	maxDoc int

	ords             map[int64]struct{}
	sims             []search.SimScorer
	norms            index.NumericDocValues
	minNrShouldMatch int

	score float64
	it    *slowMinShouldMatchIterator
}

func newSlowMinShouldMatchScorer(t *testing.T, weight *search.BooleanWeight, reader index.LeafReader, searcher *search.IndexSearcher) *slowMinShouldMatchScorer {
	t.Helper()
	s := &slowMinShouldMatchScorer{currentDoc: -1, currentMatched: -1, ords: map[int64]struct{}{}, score: float64(float32(math.NaN()))}
	s.it = &slowMinShouldMatchIterator{s: s}
	dv, err := reader.GetSortedSetDocValues("dv")
	if err != nil {
		t.Fatal(err)
	}
	s.dv = dv
	s.maxDoc = reader.MaxDoc()
	bq := weight.GetQuery().(*search.BooleanQuery)
	s.minNrShouldMatch = bq.GetMinimumNumberShouldMatch()
	lookup, ok := dv.(msm2SortedSetDocValuesLookup)
	if !ok {
		t.Fatal(sortedSetDocValuesLookupBlocker)
	}
	s.sims = make([]search.SimScorer, lookup.GetValueCount())
	for _, clause := range bq.Clauses() {
		if util.AssertsEnabled() && (clause.IsProhibited() || clause.IsRequired()) {
			panic(util.NewAssertionError(nil))
		}
		term := clause.Query().(*search.TermQuery).GetTerm()
		ord, err := lookup.LookupTerm(term.BytesValue())
		if err != nil {
			t.Fatal(err)
		}
		if ord >= 0 {
			_, dup := s.ords[int64(ord)]
			s.ords[int64(ord)] = struct{}{}
			if util.AssertsEnabled() && dup {
				panic(util.NewAssertionError(nil)) // no dups
			}
			ts, err := index.BuildTermStates(searcher, term, true)
			if err != nil {
				t.Fatal(err)
			}
			collectionStats, err := searcher.CollectionStatistics("field")
			if err != nil {
				t.Fatal(err)
			}
			termStats := searcher.TermStatistics(term, ts.DocFreq(), ts.TotalTermFreq())
			s.sims[ord] = search.BooleanWeightSimilarity(weight).Scorer104(1, collectionStats, &termStats)
		}
	}
	norms, err := reader.GetNormValues("field")
	if err != nil {
		t.Fatal(err)
	}
	s.norms = norms
	return s
}

func (s *slowMinShouldMatchScorer) Score() (float32, error) {
	if util.AssertsEnabled() && s.score == 0 {
		panic(util.NewAssertionError(s.currentMatched))
	}
	return float32(s.score), nil
}

func (s *slowMinShouldMatchScorer) GetMaxScore(upTo int) (float32, error) {
	return float32(math.Inf(1)), nil
}

func (s *slowMinShouldMatchScorer) DocID() int { return s.currentDoc }

func (s *slowMinShouldMatchScorer) Iterator() search.DocIdSetIterator { return s.it }

func (s *slowMinShouldMatchScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// slowMinShouldMatchIterator renders the anonymous DocIdSetIterator of iterator().
type slowMinShouldMatchIterator struct {
	s *slowMinShouldMatchScorer
}

func (it *slowMinShouldMatchIterator) NextDoc() (int, error) {
	s := it.s
	if util.AssertsEnabled() && s.currentDoc == search.NO_MORE_DOCS {
		panic(util.NewAssertionError(nil))
	}
	for s.currentDoc = s.currentDoc + 1; s.currentDoc < s.maxDoc; s.currentDoc++ {
		s.currentMatched = 0
		s.score = 0
		if s.currentDoc > s.dv.DocID() {
			if _, err := s.dv.Advance(s.currentDoc); err != nil {
				return 0, err
			}
		}
		if s.currentDoc != s.dv.DocID() {
			continue
		}
		norm := int64(1)
		if s.norms != nil {
			ok, err := s.norms.AdvanceExact(s.currentDoc)
			if err != nil {
				return 0, err
			}
			if ok {
				norm, err = s.norms.LongValue()
				if err != nil {
					return 0, err
				}
			}
		}
		for i := 0; i < s.dv.DocValueCount(); i++ {
			ord, err := s.dv.NextOrd()
			if err != nil {
				return 0, err
			}
			if _, ok := s.ords[int64(ord)]; ok {
				s.currentMatched++
				s.score += float64(s.sims[ord].Score104(1, norm))
			}
		}
		if s.currentMatched >= s.minNrShouldMatch {
			return s.currentDoc, nil
		}
	}
	s.currentDoc = search.NO_MORE_DOCS
	return s.currentDoc, nil
}

func (it *slowMinShouldMatchIterator) Advance(target int) (int, error) {
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc >= target {
			return doc, nil
		}
	}
}

func (it *slowMinShouldMatchIterator) Cost() int64 { return int64(it.s.maxDoc) }

func (it *slowMinShouldMatchIterator) DocID() int { return it.s.currentDoc }

func (it *slowMinShouldMatchIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

func (it *slowMinShouldMatchIterator) DocIDRunEnd() (int, error) { return util.DefaultDocIDRunEnd(it) }
