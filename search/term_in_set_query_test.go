// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestTermInSetQuery.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"bytes"
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// booleanRewriteTermCountThreshold renders the package-private
// AbstractMultiTermQueryConstantScoreWrapper.BOOLEAN_REWRITE_TERM_COUNT_THRESHOLD
// (16); Gocene has no port of that class that declares it.
const booleanRewriteTermCountThreshold = 16

// Blockers for the test-framework and randomizedtesting utilities the Java
// test calls.
const (
	randomAnalysisStringBlocker          = "requires org.apache.lucene.tests.util.TestUtil.randomAnalysisString(Random, int, boolean) (not ported)"
	randomRealisticUnicodeStringBlocker  = "requires org.apache.lucene.tests.util.TestUtil.randomRealisticUnicodeString(Random) (not ported)"
	randomStringsBlocker                 = "requires com.carrotsearch.randomizedtesting.generators.RandomStrings (not ported)"
	ramUsageTesterBlocker                = "requires org.apache.lucene.tests.util.RamUsageTester (not ported)"
	sortedSetDocValuesIndexedFieldReason = sortedSetIndexedFieldBlocker
)

// randomAnalysisString renders TestUtil.randomAnalysisString(Random, int, boolean).
func randomAnalysisString(t *testing.T, maxLength int, simple bool) string {
	t.Helper()
	t.Fatal(randomAnalysisStringBlocker)
	return ""
}

// randomRealisticUnicodeString renders TestUtil.randomRealisticUnicodeString(Random).
func randomRealisticUnicodeString(t *testing.T) string {
	t.Helper()
	t.Fatal(randomRealisticUnicodeStringBlocker)
	return ""
}

func TestTermInSetQueryAllDocsInFieldTerm(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	field := "f"

	denseTerm := util.NewBytesRef([]byte(randomAnalysisString(t, 10, true)))

	randomTerms := map[string]*util.BytesRef{}
	for len(randomTerms) < booleanRewriteTermCountThreshold {
		b := util.NewBytesRef([]byte(randomAnalysisString(t, 10, true)))
		randomTerms[string(b.ValidBytes())] = b
	}
	if util.AssertsEnabled() && !(len(randomTerms) == booleanRewriteTermCountThreshold) {
		panic(util.NewAssertionError(nil))
	}
	otherTerms := make([]*util.BytesRef, 0, len(randomTerms))
	for _, term := range randomTerms {
		otherTerms = append(otherTerms, term)
	}

	// Every doc with a value for `field` will contain `denseTerm`:
	numDocs := 10 * len(otherTerms)
	for i := 0; i < numDocs; i++ {
		sparseTerm := otherTerms[i%len(otherTerms)]
		doc := newTestDocument(
			newStringFieldBytes(t, field, denseTerm.ValidBytes(), false),
			newStringFieldBytes(t, field, sparseTerm.ValidBytes(), false))
		mustAddDocument(t, iw, doc)
	}

	// Make sure there are some docs in the index that don't contain a value for the field at all:
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(mustStringField(t, "foo", "bar", false))
	}

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	queryTerms := append([]*util.BytesRef(nil), otherTerms...)
	queryTerms = append(queryTerms, denseTerm)

	query := search.NewTermInSetQuery(field, queryTerms)
	topDocs := mustSearch(t, searcher, query, numDocs)
	if topDocs.TotalHits.Value != int64(numDocs) {
		t.Fatalf("expected %d, got %d", numDocs, topDocs.TotalHits.Value)
	}

	mustClose(t, reader, dir)
}

func TestTermInSetQueryDuel(t *testing.T) {
	iters := atLeast(2)
	field := "f"
	for iter := 0; iter < iters; iter++ {
		var allTerms []*util.BytesRef
		numTerms := nextInt(1, 1<<nextInt(1, 10))
		for i := 0; i < numTerms; i++ {
			value := randomAnalysisString(t, 10, true)
			allTerms = append(allTerms, util.NewBytesRef([]byte(value)))
		}
		dir := newDirectory()
		iw := newRandomIndexWriter(t, dir)
		numDocs := atLeast(10_000)
		for i := 0; i < numDocs; i++ {
			term := allTerms[random().Intn(len(allTerms))]
			doc := newTestDocument(newStringFieldBytes(t, field, term.ValidBytes(), false))
			// Also include a doc values field with a skip-list so we can test doc-value rewrite as
			// well: doc.add(SortedSetDocValuesField.indexedField(field, term));
			t.Fatal(sortedSetDocValuesIndexedFieldReason)
			mustAddDocument(t, iw, doc)
		}
		if numTerms > 1 && random().Intn(2) == 0 {
			if _, err := iw.DeleteDocumentsQuery(search.NewTermQuery(index.NewTermFromBytesRef(field, allTerms[0]))); err != nil {
				t.Fatalf("deleteDocuments: %v", err)
			}
		}
		if _, err := iw.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		reader := mustGetReader(t, iw)
		searcher := newSearcher(t, reader)
		mustClose(t, iw)

		if reader.NumDocs() == 0 {
			// may occasionally happen if all documents got the same term
			mustClose(t, reader, dir)
			continue
		}

		for i := 0; i < 100; i++ {
			boost := random().Float32() * 10
			numQueryTerms := nextInt(1, 1<<nextInt(1, 8))
			var queryTerms []*util.BytesRef
			for j := 0; j < numQueryTerms; j++ {
				queryTerms = append(queryTerms, allTerms[random().Intn(len(allTerms))])
			}
			bq := search.NewBooleanQueryBuilder()
			for _, term := range queryTerms {
				bq.Add(search.NewTermQuery(index.NewTermFromBytesRef(field, term)), search.SHOULD)
			}
			q1 := search.NewConstantScoreQuery(bq.Build())
			q2 := search.NewTermInSetQuery(field, queryTerms)
			q3 := search.NewTermInSetQueryWithRewrite(search.DocValuesRewrite, field, queryTerms)
			assertTermInSetSameMatches(t, searcher, search.NewBoostQuery(q1, boost), search.NewBoostQuery(q2, boost), true)
			assertTermInSetSameMatches(t, searcher, search.NewBoostQuery(q1, boost), search.NewBoostQuery(q3, boost), false)
		}

		mustClose(t, reader, dir)
	}
}

func TestTermInSetQueryReturnsNullScoreSupplier(t *testing.T) {
	directory := newDirectory()
	defer mustClose(t, directory)
	func() {
		writer := mustNewIndexWriter(t, directory, index.NewIndexWriterConfig())
		defer mustClose(t, writer)
		for ch := 'a'; ch <= 'z'; ch++ {
			id, err := document.NewKeywordField("id", string(ch), true)
			if err != nil {
				t.Fatalf("new KeywordField: %v", err)
			}
			content, err := document.NewKeywordField("content", string(ch), true)
			if err != nil {
				t.Fatalf("new KeywordField: %v", err)
			}
			mustAddDocument(t, writer, newTestDocument(id, content))
		}
	}()
	reader := mustOpenDirectoryReader(t, directory)
	defer mustClose(t, reader)
	var terms []*util.BytesRef
	for ch := 'a'; ch <= 'z'; ch++ {
		terms = append(terms, util.NewBytesRef([]byte(string(ch))))
	}
	query2 := search.NewTermInSetQuery("content", terms)

	{
		// query1 doesn't match any documents
		query1 := search.NewTermInSetQuery("id", []*util.BytesRef{util.NewBytesRef([]byte("aaa")), util.NewBytesRef([]byte("bbb"))})
		queryBuilder := search.NewBooleanQueryBuilder()
		queryBuilder.Add(query1, search.FILTER)
		queryBuilder.Add(query2, search.FILTER)
		boolQuery := queryBuilder.Build()

		searcher := search.NewIndexSearcher(reader)
		ctx := mustLeaves(t, reader)[0]

		weight1, err := searcher.CreateWeight(mustRewrite(t, searcher, query1), search.COMPLETE, 1)
		if err != nil {
			t.Fatalf("createWeight: %v", err)
		}
		scorerSupplier1, err := weight1.ScorerSupplier(ctx)
		if err != nil {
			t.Fatalf("scorerSupplier: %v", err)
		}
		// as query1 doesn't match any documents, its scorerSupplier must be null
		if scorerSupplier1 != nil {
			t.Fatal("expected a null scorerSupplier for query1")
		}
		weight, err := searcher.CreateWeight(mustRewrite(t, searcher, boolQuery), search.COMPLETE, 1)
		if err != nil {
			t.Fatalf("createWeight: %v", err)
		}
		// scorerSupplier of a bool query where query1 is mandatory must be null
		scorerSupplier, err := weight.ScorerSupplier(ctx)
		if err != nil {
			t.Fatalf("scorerSupplier: %v", err)
		}
		if scorerSupplier != nil {
			t.Fatal("expected a null scorerSupplier for the boolean query")
		}
	}
	{
		// query1 matches some documents
		query1 := search.NewTermInSetQuery("id",
			[]*util.BytesRef{util.NewBytesRef([]byte("aaa")), util.NewBytesRef([]byte("bbb")), util.NewBytesRef([]byte("b"))})
		queryBuilder := search.NewBooleanQueryBuilder()
		queryBuilder.Add(query1, search.FILTER)
		queryBuilder.Add(query2, search.FILTER)
		boolQuery := queryBuilder.Build()

		searcher := search.NewIndexSearcher(reader)
		ctx := mustLeaves(t, reader)[0]

		weight1, err := searcher.CreateWeight(mustRewrite(t, searcher, query1), search.COMPLETE, 1)
		if err != nil {
			t.Fatalf("createWeight: %v", err)
		}
		scorerSupplier1, err := weight1.ScorerSupplier(ctx)
		if err != nil {
			t.Fatalf("scorerSupplier: %v", err)
		}
		// as query1 matches some documents, its scorerSupplier must not be null
		if scorerSupplier1 == nil {
			t.Fatal("expected a non-null scorerSupplier for query1")
		}
		weight, err := searcher.CreateWeight(mustRewrite(t, searcher, boolQuery), search.COMPLETE, 1)
		if err != nil {
			t.Fatalf("createWeight: %v", err)
		}
		// scorerSupplier of a bool query where query1 is mandatory must not be null
		scorerSupplier, err := weight.ScorerSupplier(ctx)
		if err != nil {
			t.Fatalf("scorerSupplier: %v", err)
		}
		if scorerSupplier == nil {
			t.Fatal("expected a non-null scorerSupplier for the boolean query")
		}
	}
}

// Make sure the doc values skipper isn't making the incorrect assumption that
// the min/max terms from a TermInSetQuery don't form a continuous range.
func TestTermInSetQuerySkipperOptimizationGapAssumption(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	iw := newRandomIndexWriter(t, dir)
	defer mustClose(t, iw)
	// Index the first 10,000 docs all with the term "b" to get some skip list blocks with the
	// range [b, b]: each doc carries SortedSetDocValuesField("field", term) and
	// SortedSetDocValuesField.indexedField("idx_field", term).
	t.Fatal(sortedSetDocValuesIndexedFieldReason)
}

// assertTermInSetSameMatches renders the private assertSameMatches(IndexSearcher, Query, Query, boolean).
func assertTermInSetSameMatches(t *testing.T, searcher *search.IndexSearcher, q1, q2 search.Query, scores bool) {
	t.Helper()
	maxDoc := searcher.GetIndexReader().MaxDoc()
	sort := search.INDEXORDER
	if scores {
		sort = search.RELEVANCE
	}
	td1, err := searcher.SearchWithSort(q1, maxDoc, sort, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	td2, err := searcher.SearchWithSort(q2, maxDoc, sort, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if td1.TotalHits.Value != td2.TotalHits.Value {
		t.Fatalf("totalHits: %d vs %d", td1.TotalHits.Value, td2.TotalHits.Value)
	}
	for i := 0; i < len(td1.ScoreDocs); i++ {
		if td1.ScoreDocs[i].Doc != td2.ScoreDocs[i].Doc {
			t.Fatalf("doc %d: %d vs %d", i, td1.ScoreDocs[i].Doc, td2.ScoreDocs[i].Doc)
		}
		if scores {
			if math.Abs(float64(td1.ScoreDocs[i].Score-td2.ScoreDocs[i].Score)) > 10e-7 {
				t.Fatalf("score %d: %v vs %v", i, td1.ScoreDocs[i].Score, td2.ScoreDocs[i].Score)
			}
		}
	}
}

func TestTermInSetQueryHashCodeAndEquals(t *testing.T) {
	num := atLeast(100)
	var terms []*util.BytesRef
	uniqueTerms := map[string]*util.BytesRef{}
	var uniqueOrder []string
	for i := 0; i < num; i++ {
		str := randomRealisticUnicodeString(t)
		terms = append(terms, util.NewBytesRef([]byte(str)))
		if _, ok := uniqueTerms[str]; !ok {
			uniqueOrder = append(uniqueOrder, str)
		}
		uniqueTerms[str] = util.NewBytesRef([]byte(str))
		uniqueList := make([]*util.BytesRef, 0, len(uniqueOrder))
		for _, s := range uniqueOrder {
			uniqueList = append(uniqueList, uniqueTerms[s])
		}
		left := search.NewTermInSetQuery("field", uniqueList)
		random().Shuffle(len(terms), func(a, b int) { terms[a], terms[b] = terms[b], terms[a] })
		right := search.NewTermInSetQuery("field", terms)
		if !right.Equals(left) {
			t.Fatalf("expected %v to equal %v", right, left)
		}
		if right.HashCode() != left.HashCode() {
			t.Fatal("equal queries must have equal hash codes")
		}
		if len(uniqueTerms) > 1 {
			asList := append([]*util.BytesRef(nil), uniqueList[1:]...)
			notEqual := search.NewTermInSetQuery("field", asList)
			if left.Equals(notEqual) {
				t.Fatal("left must not equal notEqual")
			}
			if right.Equals(notEqual) {
				t.Fatal("right must not equal notEqual")
			}
		}
	}

	tq1 := search.NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	tq2 := search.NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("orange"))})
	if tq1.HashCode() == tq2.HashCode() {
		t.Fatal("expected different hash codes")
	}

	// different fields with the same term should have differing hashcodes
	tq1 = search.NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	tq2 = search.NewTermInSetQuery("thing2", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	if tq1.HashCode() == tq2.HashCode() {
		t.Fatal("expected different hash codes")
	}
}

// javaStringHashCode renders java.lang.String.hashCode() over UTF-16 chars.
func javaStringHashCode(s string) int32 {
	var h int32
	for _, c := range utf16Units(s) {
		h = 31*h + int32(c)
	}
	return h
}

func utf16Units(s string) []uint16 {
	var out []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			out = append(out, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
		} else {
			out = append(out, uint16(r))
		}
	}
	return out
}

func TestTermInSetQuerySimpleEquals(t *testing.T) {
	// Two terms with the same hash code
	if javaStringHashCode("AaAaBB") != javaStringHashCode("BBBBBB") {
		t.Fatal("expected equal Java hash codes")
	}
	left := search.NewTermInSetQuery("id", []*util.BytesRef{util.NewBytesRef([]byte("AaAaAa")), util.NewBytesRef([]byte("AaAaBB"))})
	right := search.NewTermInSetQuery("id", []*util.BytesRef{util.NewBytesRef([]byte("AaAaAa")), util.NewBytesRef([]byte("BBBBBB"))})
	if left.Equals(right) {
		t.Fatal("left must not equal right")
	}
}

func TestTermInSetQueryToString(t *testing.T) {
	termsQuery := search.NewTermInSetQuery("field1",
		[]*util.BytesRef{util.NewBytesRef([]byte("a")), util.NewBytesRef([]byte("b")), util.NewBytesRef([]byte("c"))})
	if got := termsQuery.ToString(""); got != "field1:(a b c)" {
		t.Fatalf("expected %q, got %q", "field1:(a b c)", got)
	}
}

func TestTermInSetQueryDedup(t *testing.T) {
	query1 := search.NewTermInSetQuery("foo", []*util.BytesRef{util.NewBytesRef([]byte("bar"))})
	query2 := search.NewTermInSetQuery("foo", []*util.BytesRef{util.NewBytesRef([]byte("bar")), util.NewBytesRef([]byte("bar"))})
	queryUtilsCheckEqual(t, query1, query2)
}

func TestTermInSetQueryOrderDoesNotMatter(t *testing.T) {
	// order of terms if different
	query1 := search.NewTermInSetQuery("foo", []*util.BytesRef{util.NewBytesRef([]byte("bar")), util.NewBytesRef([]byte("baz"))})
	query2 := search.NewTermInSetQuery("foo", []*util.BytesRef{util.NewBytesRef([]byte("baz")), util.NewBytesRef([]byte("bar"))})
	queryUtilsCheckEqual(t, query1, query2)
}

func TestTermInSetQueryRamBytesUsed(t *testing.T) {
	// terms.add(newBytesRef(RandomStrings.randomUnicodeOfLength(random(), 10)));
	t.Fatal(randomStringsBlocker + "; " + ramUsageTesterBlocker)
}

func TestTermInSetQueryPullOneTermsEnum(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	doc := newTestDocument(mustStringField(t, "foo", "1", false))
	mustAddDocument(t, w, doc)
	reader := mustGetReader(t, w)
	mustClose(t, w)
	defer mustClose(t, reader, dir)
	// The private TermsCountingDirectoryReaderWrapper extends FilterDirectoryReader
	// with a SubReaderWrapper that counts Terms.iterator() calls.
	t.Fatal(filterDirectoryReaderSubReaderWrapperBlocker)
}

func TestTermInSetQueryBinaryToString(t *testing.T) {
	query := search.NewTermInSetQuery("field", []*util.BytesRef{util.NewBytesRef([]byte{0xff, 0xfe})})
	if got := query.ToString(""); got != "field:([ff fe])" {
		t.Fatalf("expected %q, got %q", "field:([ff fe])", got)
	}
}

func TestTermInSetQueryIsConsideredCostlyByQueryCache(t *testing.T) {
	query := search.NewTermInSetQuery("foo", []*util.BytesRef{util.NewBytesRef([]byte("bar")), util.NewBytesRef([]byte("baz"))})
	policy := search.NewUsageTrackingQueryCachingPolicy()
	if shouldCache, err := policy.ShouldCache(query); err != nil || shouldCache {
		t.Fatalf("expected not cached, got %v (%v)", shouldCache, err)
	}
	policy.OnUse(query)
	policy.OnUse(query)
	// cached after two uses
	if shouldCache, err := policy.ShouldCache(query); err != nil || !shouldCache {
		t.Fatalf("expected cached after two uses, got %v (%v)", shouldCache, err)
	}
}

// termInSetSingletonVisitor renders the first anonymous QueryVisitor of testVisitor.
type termInSetSingletonVisitor struct {
	t *testing.T
}

func (v *termInSetSingletonVisitor) ConsumeTerms(query search.Query, terms ...*index.Term) {
	if len(terms) != 1 {
		v.t.Fatalf("expected 1 term, got %d", len(terms))
	}
	if !index.NewTermFromBytesRef("field", util.NewBytesRef([]byte("term1"))).Equals(terms[0]) {
		v.t.Fatalf("unexpected term %v", terms[0])
	}
}

func (v *termInSetSingletonVisitor) ConsumeTermsMatching(query search.Query, field string, a func() search.ByteRunAutomaton) {
	v.t.Fatal("Singleton TermInSetQuery should not try to build ByteRunAutomaton")
}

func (v *termInSetSingletonVisitor) VisitLeaf(query search.Query) {}

func (v *termInSetSingletonVisitor) AcceptField(field string) bool { return true }

func (v *termInSetSingletonVisitor) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	if occur == search.MUST_NOT {
		return search.EmptyQueryVisitor
	}
	return v
}

// termInSetAutomatonVisitor renders the second anonymous QueryVisitor of testVisitor.
type termInSetAutomatonVisitor struct {
	t     *testing.T
	terms []*util.BytesRef
}

func (v *termInSetAutomatonVisitor) ConsumeTerms(query search.Query, terms ...*index.Term) {
	v.t.Fatal("TermInSetQuery with multiple terms should build automaton")
}

func (v *termInSetAutomatonVisitor) ConsumeTermsMatching(query search.Query, field string, automaton func() search.ByteRunAutomaton) {
	a := automaton()
	test := []byte("nonmatching")
	if a.Run(test) {
		v.t.Fatal("automaton must not accept nonmatching")
	}
	for _, term := range v.terms {
		if !a.Run(term.ValidBytes()) {
			v.t.Fatalf("automaton must accept %s", term.ValidBytes())
		}
	}
}

func (v *termInSetAutomatonVisitor) VisitLeaf(query search.Query) {}

func (v *termInSetAutomatonVisitor) AcceptField(field string) bool { return true }

func (v *termInSetAutomatonVisitor) GetSubVisitor(occur search.Occur, parent search.Query) search.QueryVisitor {
	if occur == search.MUST_NOT {
		return search.EmptyQueryVisitor
	}
	return v
}

func TestTermInSetQueryVisitor(t *testing.T) {
	// singleton reports back to consumeTerms()
	singleton := search.NewTermInSetQuery("field", []*util.BytesRef{util.NewBytesRef([]byte("term1"))})
	singleton.Visit(&termInSetSingletonVisitor{t: t})

	// multiple values built into automaton
	var terms []*util.BytesRef
	for i := 0; i < 100; i++ {
		terms = append(terms, util.NewBytesRef([]byte("term"+strconv.Itoa(i))))
	}
	q := search.NewTermInSetQuery("field", terms)
	q.Visit(&termInSetAutomatonVisitor{t: t, terms: terms})
}

func TestTermInSetQueryTermsIterator(t *testing.T) {
	empty := search.NewTermInSetQuery("field", nil)
	it := empty.GetBytesRefIterator()
	if next, err := it.Next(); err != nil || next != nil {
		t.Fatalf("expected null, got %v (%v)", next, err)
	}

	query := search.NewTermInSetQuery("field",
		[]*util.BytesRef{util.NewBytesRef([]byte("term1")), util.NewBytesRef([]byte("term2")), util.NewBytesRef([]byte("term3"))})
	it = query.GetBytesRefIterator()
	for _, want := range []string{"term1", "term2", "term3"} {
		next, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if next == nil || !bytes.Equal(next.ValidBytes(), []byte(want)) {
			t.Fatalf("expected %s, got %v", want, next)
		}
	}
	if next, err := it.Next(); err != nil || next != nil {
		t.Fatalf("expected null, got %v (%v)", next, err)
	}
}
