// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestBlockJoin.java
// (Apache Lucene 10.5.0). The @Nightly testRandom and the helpers only it uses
// live in block_join_random_monster_test.go (build tag gocene_monsters).

// intPointNewRangeQueryBlocker names IntPoint.newRangeQuery / newSetQuery:
// they return org.apache.lucene.search.PointRangeQuery / PointInSetQuery, and
// Gocene's document package cannot import search (search imports document),
// so the IntPoint query factories are not ported.
const intPointNewRangeQueryBlocker = "requires org.apache.lucene.document.IntPoint.newRangeQuery(String, int, int) (not ported)"

// intPointNewRangeQuery renders IntPoint.newRangeQuery(String, int, int).
func intPointNewRangeQuery(t testing.TB, field string, lowerValue, upperValue int) search.Query {
	t.Helper()
	t.Fatal(intPointNewRangeQueryBlocker)
	return nil
}

// makeResume renders the private makeResume(String, String): one resume...
func makeResume(t testing.TB, name, country string) *document.Document {
	t.Helper()
	return newTestDocument(
		newStringField(t, "docType", "resume", false),
		newStringField(t, "name", name, true),
		newStringField(t, "country", country, false))
}

// makeJob renders the private makeJob(String, int): ... has multiple jobs
func makeJob(t testing.TB, skill string, year int) *document.Document {
	t.Helper()
	return newTestDocument(
		newStringField(t, "skill", skill, true),
		document.NewIntPoint("year", int32(year)),
		mustStoredIntField(t, "year", year))
}

// makeQualification renders the private makeQualification(String, int): ...
// has multiple qualifications
func makeQualification(t testing.TB, qualification string, year int) *document.Document {
	t.Helper()
	return newTestDocument(
		newStringField(t, "qualification", qualification, true),
		document.NewIntPoint("year", int32(year)),
		mustStoredIntField(t, "year", year))
}

// makeVector renders the private makeVector(String, String, float[]).
func makeVector(t testing.TB, vectorField, childsParent string, value []float32) *document.Document {
	t.Helper()
	vf, err := document.NewKnnFloatVectorFieldEuclidean(vectorField, value)
	if err != nil {
		t.Fatalf("new KnnFloatVectorField: %v", err)
	}
	return newTestDocument(vf, newStringField(t, "my_parent_id", childsParent, true))
}

// makeParent renders the private makeParent(String).
func makeParent(t testing.TB, parentID string) *document.Document {
	t.Helper()
	return newTestDocument(
		newStringField(t, "docType", "_parent", false),
		newStringField(t, "parent_id", parentID, true))
}

// mustStoredIntField renders new StoredField(String, int).
func mustStoredIntField(t testing.TB, name string, value int) *document.StoredField {
	t.Helper()
	f, err := document.NewStoredFieldFromInt(name, value)
	if err != nil {
		t.Fatalf("new StoredField: %v", err)
	}
	return f
}

// mustStoredStringField renders new StoredField(String, String).
func mustStoredStringField(t testing.TB, name, value string) *document.StoredField {
	t.Helper()
	f, err := document.NewStoredField(name, value)
	if err != nil {
		t.Fatalf("new StoredField: %v", err)
	}
	return f
}

// mustAddDocuments renders addDocuments(List<Document>).
func mustAddDocuments(t testing.TB, w interface {
	AddDocuments(docs []*document.Document) (int64, error)
}, docs ...*document.Document) {
	t.Helper()
	if _, err := w.AddDocuments(docs); err != nil {
		t.Fatalf("addDocuments: %v", err)
	}
}

// storedGet renders searcher.storedFields().document(doc).get(field): the
// string value of the first field of that name, or "" (Java's null) when absent.
func storedGet(t testing.TB, p storedFieldsProvider, doc int, field string) string {
	t.Helper()
	f := storedDocument(t, mustStoredFields(t, p), doc).Get(field)
	if f == nil {
		return ""
	}
	return f.StringValue()
}

// asSet renders LuceneTestCase.asSet(T...).
func asSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}

func assertSetEquals(t testing.TB, expected, actual map[string]struct{}) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("sets differ: expected %v, got %v", expected, actual)
	}
	for k := range expected {
		if _, ok := actual[k]; !ok {
			t.Fatalf("sets differ: expected %v, got %v", expected, actual)
		}
	}
}

func assertInt64Equals(t testing.TB, expected, actual int64) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %d, got %d", expected, actual)
	}
}

func assertStringEquals(t testing.TB, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}

func mustCheckJoinIndex(t testing.TB, r index.IndexReaderInterface, parentsFilter BitSetProducer) {
	t.Helper()
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("CheckJoinIndex.check: %v", err)
	}
}

// javaSkillJavaYear2006To2011 renders the child query shared by several tests:
// +skill:java +year:[2006 TO 2011].
func javaSkillJavaYear2006To2011(t testing.TB) *search.BooleanQueryBuilder {
	t.Helper()
	childQuery := search.NewBooleanQueryBuilder()
	childQuery.Add(search.NewTermQuery(index.NewTerm("skill", "java")), search.MUST)
	childQuery.Add(intPointNewRangeQuery(t, "year", 2006, 2011), search.MUST)
	return childQuery
}

func TestBlockJoinEmptyChildFilter(t *testing.T) {
	dir := newDirectory()
	config := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	config.SetMergePolicy(index.NewNoMergePolicy())
	// we don't want to merge - since we rely on certain segment setup
	w := mustNewIndexWriter(t, dir, config)

	mustAddDocuments(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))

	mustAddDocuments(t, w, makeJob(t, "ruby", 2005), makeJob(t, "java", 2006), makeResume(t, "Frank", "United States"))
	mustCommit(t, w)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	mustClose(t, w)
	s := newSearcher(t, r)
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	mustCheckJoinIndex(t, r, parentsFilter)

	childQuery := javaSkillJavaYear2006To2011(t)

	childJoinQuery := NewToParentBlockJoinQuery(childQuery.Build(), parentsFilter, Avg)

	fullQuery := search.NewBooleanQueryBuilder()
	fullQuery.Add(childJoinQuery, search.MUST)
	fullQuery.Add(search.Instance, search.MUST)
	topDocs := mustSearch(t, s, fullQuery.Build(), 2)
	assertInt64Equals(t, 2, topDocs.TotalHits.Value)
	assertSetEquals(t, asSet("Lisa", "Frank"), asSet(
		storedGet(t, s, topDocs.ScoreDocs[0].Doc, "name"),
		storedGet(t, s, topDocs.ScoreDocs[1].Doc, "name")))

	childrenQuery := NewParentChildrenBlockJoinQuery(parentsFilter, childQuery.Build(), topDocs.ScoreDocs[0].Doc)
	matchingChildren := mustSearch(t, s, childrenQuery, 1)
	assertInt64Equals(t, 1, matchingChildren.TotalHits.Value)
	assertStringEquals(t, "java", storedGet(t, s, matchingChildren.ScoreDocs[0].Doc, "skill"))

	childrenQuery = NewParentChildrenBlockJoinQuery(parentsFilter, childQuery.Build(), topDocs.ScoreDocs[1].Doc)
	matchingChildren = mustSearch(t, s, childrenQuery, 1)
	assertInt64Equals(t, 1, matchingChildren.TotalHits.Value)
	assertStringEquals(t, "java", storedGet(t, s, matchingChildren.ScoreDocs[0].Doc, "skill"))

	mustClose(t, r, dir)
}

// You must use ToParentBlockJoinSearcher if you want to do BQ SHOULD queries:
func TestBlockJoinBQShouldJoinedChild(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	mustAddDocuments(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))

	mustAddDocuments(t, w, makeJob(t, "ruby", 2005), makeJob(t, "java", 2006), makeResume(t, "Frank", "United States"))

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	// IndexSearcher s = new IndexSearcher(r);

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	mustCheckJoinIndex(t, r, parentsFilter)

	// Define child document criteria (finds an example of relevant work experience)
	childQuery := javaSkillJavaYear2006To2011(t)

	// Define parent document criteria (find a resident in the UK)
	parentQuery := search.NewTermQuery(index.NewTerm("country", "United Kingdom"))

	// Wrap the child document query to 'join' any matches
	// up to corresponding parent:
	childJoinQuery := NewToParentBlockJoinQuery(childQuery.Build(), parentsFilter, Avg)

	// Combine the parent and nested child queries into a single query for a candidate
	fullQuery := search.NewBooleanQueryBuilder()
	fullQuery.Add(parentQuery, search.SHOULD)
	fullQuery.Add(childJoinQuery, search.SHOULD)

	topDocs := mustSearch(t, s, fullQuery.Build(), 2)
	assertInt64Equals(t, 2, topDocs.TotalHits.Value)
	assertSetEquals(t, asSet("Lisa", "Frank"), asSet(
		storedGet(t, s, topDocs.ScoreDocs[0].Doc, "name"),
		storedGet(t, s, topDocs.ScoreDocs[1].Doc, "name")))

	childrenQuery := NewParentChildrenBlockJoinQuery(parentsFilter, childQuery.Build(), topDocs.ScoreDocs[0].Doc)
	matchingChildren := mustSearch(t, s, childrenQuery, 1)
	assertInt64Equals(t, 1, matchingChildren.TotalHits.Value)
	assertStringEquals(t, "java", storedGet(t, s, matchingChildren.ScoreDocs[0].Doc, "skill"))

	childrenQuery = NewParentChildrenBlockJoinQuery(parentsFilter, childQuery.Build(), topDocs.ScoreDocs[1].Doc)
	matchingChildren = mustSearch(t, s, childrenQuery, 1)
	assertInt64Equals(t, 1, matchingChildren.TotalHits.Value)
	assertStringEquals(t, "java", storedGet(t, s, matchingChildren.ScoreDocs[0].Doc, "skill"))

	mustClose(t, r, dir)
}

func TestBlockJoinSimpleKnn(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	mustAddDocuments(t, w,
		makeVector(t, "vector", "parent1", []float32{1, 2, 3}),
		makeVector(t, "vector", "parent1", []float32{3, 3, 3}),
		makeParent(t, "parent1"))

	mustAddDocuments(t, w,
		makeVector(t, "vector", "parent2", []float32{0, 0, 1}),
		makeVector(t, "vector", "parent2", []float32{1, 1, 1}),
		makeParent(t, "parent2"))

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	// Create a filter that defines "parent" documents in the index
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent")))
	mustCheckJoinIndex(t, r, parentsFilter)

	childKnnJoin := NewDiversifyingChildrenFloatKnnVectorQuery("vector", []float32{4, 4, 4}, 3, nil, parentsFilter)

	topDocs := mustSearch(t, s, childKnnJoin, 5)
	assertInt64Equals(t, 2, topDocs.TotalHits.Value)
	assertStringEquals(t, "parent1", storedGet(t, s, topDocs.ScoreDocs[0].Doc, "my_parent_id"))
	want := util.EuclideanSim.CompareFloat([]float32{4, 4, 4}, []float32{3, 3, 3})
	if d := float64(topDocs.ScoreDocs[0].Score - want); d > 1e-7 || d < -1e-7 {
		t.Fatalf("score: expected %v, got %v", want, topDocs.ScoreDocs[0].Score)
	}
	assertStringEquals(t, "parent2", storedGet(t, s, topDocs.ScoreDocs[1].Doc, "my_parent_id"))
	mustClose(t, r, dir)
}

func TestBlockJoinSimple(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	mustAddDocuments(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))

	mustAddDocuments(t, w, makeJob(t, "ruby", 2005), makeJob(t, "java", 2006), makeResume(t, "Frank", "United States"))

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	mustCheckJoinIndex(t, r, parentsFilter)

	// Define child document criteria (finds an example of relevant work experience)
	childQuery := javaSkillJavaYear2006To2011(t)

	// Define parent document criteria (find a resident in the UK)
	parentQuery := search.NewTermQuery(index.NewTerm("country", "United Kingdom"))

	// Wrap the child document query to 'join' any matches
	// up to corresponding parent:
	childJoinQuery := NewToParentBlockJoinQuery(childQuery.Build(), parentsFilter, Avg)

	// Combine the parent and nested child queries into a single query for a candidate
	fullQuery := search.NewBooleanQueryBuilder()
	fullQuery.Add(parentQuery, search.MUST)
	fullQuery.Add(childJoinQuery, search.MUST)

	testsearch.CheckHitCollector(t, fullQuery.Build(), "country", s, []int{2})

	topDocs := mustSearch(t, s, fullQuery.Build(), 1)
	// assertEquals(1, results.totalHitCount);
	assertInt64Equals(t, 1, topDocs.TotalHits.Value)
	assertStringEquals(t, "Lisa", storedGet(t, s, topDocs.ScoreDocs[0].Doc, "name"))

	childrenQuery := NewParentChildrenBlockJoinQuery(parentsFilter, childQuery.Build(), topDocs.ScoreDocs[0].Doc)
	matchingChildren := mustSearch(t, s, childrenQuery, 1)
	assertInt64Equals(t, 1, matchingChildren.TotalHits.Value)
	assertStringEquals(t, "java", storedGet(t, s, matchingChildren.ScoreDocs[0].Doc, "skill"))

	// Now join "up" (map parent hits to child docs) instead...:
	parentJoinQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter)
	fullChildQuery := search.NewBooleanQueryBuilder()
	fullChildQuery.Add(parentJoinQuery, search.MUST)
	fullChildQuery.Add(childQuery.Build(), search.MUST)

	hits := mustSearch(t, s, fullChildQuery.Build(), 10)
	assertInt64Equals(t, 1, hits.TotalHits.Value)
	childDoc := storedDocument(t, mustStoredFields(t, s), hits.ScoreDocs[0].Doc)
	assertStringEquals(t, "java", childDoc.Get("skill").StringValue())
	if year := childDoc.Get("year").NumericValue(); fmt.Sprint(year) != "2007" {
		t.Fatalf("year: expected 2007, got %v", year)
	}
	assertStringEquals(t, "Lisa", getParentDoc(t, r, parentsFilter, hits.ScoreDocs[0].Doc).Get("name").StringValue())

	// Test with filter on child docs:
	fullChildQuery.Add(search.NewTermQuery(index.NewTerm("skill", "foosball")), search.FILTER)
	assertIntEquals(t, 0, mustCount(t, s, fullChildQuery.Build()))

	mustClose(t, r, dir)
}

func assertIntEquals(t testing.TB, expected, actual int) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %d, got %d", expected, actual)
	}
}

func skill(s string) search.Query {
	return search.NewTermQuery(index.NewTerm("skill", s))
}

func TestBlockJoinSimpleFilter(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	docs := []*document.Document{makeJob(t, "java", 2007), makeJob(t, "python", 2010)}
	shuffleDocs(docs)
	docs = append(docs, makeResume(t, "Lisa", "United Kingdom"))

	docs2 := []*document.Document{makeJob(t, "ruby", 2005), makeJob(t, "java", 2006)}
	shuffleDocs(docs2)
	docs2 = append(docs2, makeResume(t, "Frank", "United States"))

	addSkillless(t, w)
	turn := random().Intn(2) == 0
	if turn {
		mustAddDocuments(t, w, docs...)
	} else {
		mustAddDocuments(t, w, docs2...)
	}

	addSkillless(t, w)

	if !turn {
		mustAddDocuments(t, w, docs...)
	} else {
		mustAddDocuments(t, w, docs2...)
	}

	addSkillless(t, w)

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	mustCheckJoinIndex(t, r, parentsFilter)

	// Define child document criteria (finds an example of relevant work experience)
	childQuery := javaSkillJavaYear2006To2011(t)

	// Define parent document criteria (find a resident in the UK)
	parentQuery := search.NewTermQuery(index.NewTerm("country", "United Kingdom"))

	// Wrap the child document query to 'join' any matches
	// up to corresponding parent:
	childJoinQuery := NewToParentBlockJoinQuery(childQuery.Build(), parentsFilter, Avg)

	if c := mustCount(t, s, childJoinQuery); c != 2 {
		t.Fatalf("no filter - both passed: expected 2, got %d", c)
	}

	query := search.NewBooleanQueryBuilder().
		Add(childJoinQuery, search.MUST).
		Add(search.NewTermQuery(index.NewTerm("docType", "resume")), search.FILTER).
		Build()
	if c := mustCount(t, s, query); c != 2 {
		t.Fatalf("dummy filter passes everyone: expected 2, got %d", c)
	}
	query = search.NewBooleanQueryBuilder().
		Add(childJoinQuery, search.MUST).
		Add(search.NewTermQuery(index.NewTerm("docType", "resume")), search.FILTER).
		Build()
	if c := mustCount(t, s, query); c != 2 {
		t.Fatalf("dummy filter passes everyone: expected 2, got %d", c)
	}

	// not found test
	query = search.NewBooleanQueryBuilder().
		Add(childJoinQuery, search.MUST).
		Add(search.NewTermQuery(index.NewTerm("country", "Oz")), search.FILTER).
		Build()
	if c := mustCount(t, s, query); c != 0 {
		t.Fatalf("noone live there: expected 0, got %d", c)
	}

	// apply the UK filter by the searcher
	query = search.NewBooleanQueryBuilder().
		Add(childJoinQuery, search.MUST).
		Add(parentQuery, search.FILTER).
		Build()
	ukOnly := mustSearch(t, s, query, 1)
	if ukOnly.TotalHits.Value != 1 {
		t.Fatalf("has filter - single passed: expected 1, got %d", ukOnly.TotalHits.Value)
	}
	assertStringEquals(t, "Lisa", storedGet(t, r, ukOnly.ScoreDocs[0].Doc, "name"))

	query = search.NewBooleanQueryBuilder().
		Add(childJoinQuery, search.MUST).
		Add(search.NewTermQuery(index.NewTerm("country", "United States")), search.FILTER).
		Build()
	// looking for US candidates
	usThen := mustSearch(t, s, query, 1)
	if usThen.TotalHits.Value != 1 {
		t.Fatalf("has filter - single passed: expected 1, got %d", usThen.TotalHits.Value)
	}
	assertStringEquals(t, "Frank", storedGet(t, r, usThen.ScoreDocs[0].Doc, "name"))

	us := search.NewTermQuery(index.NewTerm("country", "United States"))
	if c := mustCount(t, s, NewToChildBlockJoinQuery(us, parentsFilter)); c != 2 {
		t.Fatalf("@ US we have java and ruby: expected 2, got %d", c)
	}

	query = search.NewBooleanQueryBuilder().
		Add(NewToChildBlockJoinQuery(us, parentsFilter), search.MUST).
		Add(skill("java"), search.FILTER).
		Build()
	if c := mustCount(t, s, query); c != 1 {
		t.Fatalf("java skills in US: expected 1, got %d", c)
	}

	rubyPython := search.NewBooleanQueryBuilder()
	rubyPython.Add(search.NewTermQuery(index.NewTerm("skill", "ruby")), search.SHOULD)
	rubyPython.Add(search.NewTermQuery(index.NewTerm("skill", "python")), search.SHOULD)
	query = search.NewBooleanQueryBuilder().
		Add(NewToChildBlockJoinQuery(us, parentsFilter), search.MUST).
		Add(rubyPython.Build(), search.FILTER).
		Build()
	if c := mustCount(t, s, query); c != 1 {
		t.Fatalf("ruby skills in US: expected 1, got %d", c)
	}

	mustClose(t, r, dir)
}

// shuffleDocs renders Collections.shuffle(List<Document>, random()).
func shuffleDocs(list []*document.Document) {
	r := random()
	for i := len(list); i > 1; i-- {
		j := r.Intn(i)
		list[i-1], list[j] = list[j], list[i-1]
	}
}

func addSkillless(t testing.TB, w documentAdder) {
	t.Helper()
	if random().Intn(2) == 0 {
		country := "United States"
		if random().Intn(2) == 0 {
			country = "United Kingdom"
		}
		mustAddDocument(t, w, makeResume(t, "Skillless", country))
	}
}

// getParentDoc renders the private getParentDoc(IndexReader, BitSetProducer, int).
func getParentDoc(t testing.TB, reader index.IndexReaderInterface, parents BitSetProducer, childDocID int) *document.Document {
	t.Helper()
	leaves := mustLeaves(t, reader)
	subIndex := index.ReaderUtilSubIndexLeaves(childDocID, leaves)
	leaf := leaves[subIndex]
	bits, err := parents.GetBitSet(leaf)
	if err != nil {
		t.Fatal(err)
	}
	return storedDocument(t, mustStoredFields(t, leaf.LeafReader()), bits.NextSetBitBounded(childDocID-leaf.DocBase))
}

func TestBlockJoinBoostBug(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcher(t, r)

	q := NewToParentBlockJoinQuery(search.MatchNoDocsQueryInstance, NewQueryBitSetProducer(search.Instance), Avg)
	queryUtilsCheckSearcher(t, q, s)
	mustSearch(t, s, q, 10)
	bqB := search.NewBooleanQueryBuilder()
	bqB.Add(q, search.MUST)
	bq := bqB.Build()
	mustSearch(t, s, search.NewBoostQuery(bq, 2), 10)
	mustClose(t, r, dir)
}

func TestBlockJoinMultiChildTypes(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	mustAddDocuments(t, w,
		makeJob(t, "java", 2007),
		makeJob(t, "python", 2010),
		makeQualification(t, "maths", 1999),
		makeResume(t, "Lisa", "United Kingdom"))

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	mustCheckJoinIndex(t, s.GetIndexReader(), parentsFilter)

	// Define child document criteria (finds an example of relevant work experience)
	childJobQuery := javaSkillJavaYear2006To2011(t)

	childQualificationQuery := search.NewBooleanQueryBuilder()
	childQualificationQuery.Add(search.NewTermQuery(index.NewTerm("qualification", "maths")), search.MUST)
	childQualificationQuery.Add(intPointNewRangeQuery(t, "year", 1980, 2000), search.MUST)

	// Define parent document criteria (find a resident in the UK)
	parentQuery := search.NewTermQuery(index.NewTerm("country", "United Kingdom"))

	// Wrap the child document query to 'join' any matches
	// up to corresponding parent:
	childJobJoinQuery := NewToParentBlockJoinQuery(childJobQuery.Build(), parentsFilter, Avg)
	childQualificationJoinQuery := NewToParentBlockJoinQuery(childQualificationQuery.Build(), parentsFilter, Avg)

	// Combine the parent and nested child queries into a single query for a candidate
	fullQuery := search.NewBooleanQueryBuilder()
	fullQuery.Add(parentQuery, search.MUST)
	fullQuery.Add(childJobJoinQuery, search.MUST)
	fullQuery.Add(childQualificationJoinQuery, search.MUST)

	topDocs := mustSearch(t, s, fullQuery.Build(), 10)
	assertInt64Equals(t, 1, topDocs.TotalHits.Value)
	assertStringEquals(t, "Lisa", storedGet(t, s, topDocs.ScoreDocs[0].Doc, "name"))

	childrenQuery := NewParentChildrenBlockJoinQuery(parentsFilter, childJobQuery.Build(), topDocs.ScoreDocs[0].Doc)
	matchingChildren := mustSearch(t, s, childrenQuery, 1)
	assertInt64Equals(t, 1, matchingChildren.TotalHits.Value)
	assertStringEquals(t, "java", storedGet(t, s, matchingChildren.ScoreDocs[0].Doc, "skill"))

	childrenQuery = NewParentChildrenBlockJoinQuery(parentsFilter, childQualificationQuery.Build(), topDocs.ScoreDocs[0].Doc)
	matchingChildren = mustSearch(t, s, childrenQuery, 1)
	assertInt64Equals(t, 1, matchingChildren.TotalHits.Value)
	assertStringEquals(t, "maths", storedGet(t, s, matchingChildren.ScoreDocs[0].Doc, "qualification"))

	mustClose(t, r, dir)
}

func TestBlockJoinAdvanceSingleParentSingleChild(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	childDoc := newTestDocument(newStringField(t, "child", "1", false))
	parentDoc := newTestDocument(newStringField(t, "parent", "1", false))
	mustAddDocuments(t, w, childDoc, parentDoc)
	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcher(t, r)
	tq := search.NewTermQuery(index.NewTerm("child", "1"))
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("parent", "1")))
	mustCheckJoinIndex(t, s.GetIndexReader(), parentFilter)

	q := NewToParentBlockJoinQuery(tq, parentFilter, Avg)
	weight := mustCreateWeight(t, s, mustRewrite(t, s, q), search.COMPLETE, 1)
	sc := mustScorer(t, weight, mustLeaves(t, s.GetIndexReader())[0])
	assertAdvance(t, sc.Iterator(), 1, 1)
	mustClose(t, r, dir)
}

func TestBlockJoinAdvanceSingleParentNoChild(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewLogDocMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	parentDoc := newTestDocument(
		newStringField(t, "parent", "1", false),
		newStringField(t, "isparent", "yes", false))
	mustAddDocuments(t, w, parentDoc)

	// Add another doc so scorer is not null
	parentDoc = newTestDocument(
		newStringField(t, "parent", "2", false),
		newStringField(t, "isparent", "yes", false))
	childDoc := newTestDocument(newStringField(t, "child", "2", false))
	mustAddDocuments(t, w, childDoc, parentDoc)

	// Need single seg:
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcher(t, r)
	tq := search.NewTermQuery(index.NewTerm("child", "2"))
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("isparent", "yes")))
	mustCheckJoinIndex(t, s.GetIndexReader(), parentFilter)

	q := NewToParentBlockJoinQuery(tq, parentFilter, Avg)
	weight := mustCreateWeight(t, s, mustRewrite(t, s, q), search.COMPLETE, 1)
	sc := mustScorer(t, weight, mustLeaves(t, s.GetIndexReader())[0])
	assertAdvance(t, sc.Iterator(), 0, 2)
	mustClose(t, r, dir)
}

// randomSearchScoreMode renders RandomPicks.randomFrom(random(),
// org.apache.lucene.search.ScoreMode.values()).
func randomSearchScoreMode() search.ScoreMode {
	values := []search.ScoreMode{search.COMPLETE, search.COMPLETE_NO_SCORES, search.TOP_SCORES, search.TOP_DOCS, search.TOP_DOCS_WITH_SCORES}
	return values[random().Intn(len(values))]
}

// LUCENE-4968
func TestBlockJoinChildQueryNeverMatches(t *testing.T) {
	d := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, d, iwc)
	parent := newTestDocument(
		mustStoredStringField(t, "parentID", "0"),
		mustSortedDVField(t, "parentID", "0"),
		newTextField(t, "parentText", "text", false),
		newStringField(t, "isParent", "yes", false))

	child := newTestDocument(
		mustStoredStringField(t, "childID", "0"),
		newTextField(t, "childText", "text", false))

	// parent last:
	mustAddDocuments(t, w, child, parent)

	parent = newTestDocument(
		newTextField(t, "parentText", "text", false),
		newStringField(t, "isParent", "yes", false),
		mustStoredStringField(t, "parentID", "1"),
		mustSortedDVField(t, "parentID", "1"))

	// parent last:
	mustAddDocuments(t, w, parent)

	r := mustGetReader(t, w)
	mustClose(t, w)

	searcher := newSearcher(t, r)

	// never matches:
	var childQuery search.Query = search.NewTermQuery(index.NewTerm("childBogusField", "bogus"))
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("isParent", "yes")))
	mustCheckJoinIndex(t, r, parentsFilter)
	childJoinQuery := NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)

	weight := mustCreateWeight(t, searcher, mustRewrite(t, searcher, childJoinQuery), randomSearchScoreMode(), 1)
	scorer := mustScorer(t, weight, mustLeaves(t, searcher.GetIndexReader())[0])
	if scorer != nil {
		t.Fatal("expected a null scorer")
	}

	// never matches and produces a null scorer
	childQuery = search.NewTermQuery(index.NewTerm("bogus", "bogus"))
	childJoinQuery = NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)

	weight = mustCreateWeight(t, searcher, mustRewrite(t, searcher, childJoinQuery), randomSearchScoreMode(), 1)
	scorer = mustScorer(t, weight, mustLeaves(t, searcher.GetIndexReader())[0])
	if scorer != nil {
		t.Fatal("expected a null scorer")
	}

	mustClose(t, r, d)
}

func TestBlockJoinAdvanceSingleDeletedParentNoChild(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// First doc with 1 children
	parentDoc := newTestDocument(
		newStringField(t, "parent", "1", false),
		newStringField(t, "isparent", "yes", false))
	childDoc := newTestDocument(newStringField(t, "child", "1", false))
	mustAddDocuments(t, w, childDoc, parentDoc)

	parentDoc = newTestDocument(
		newStringField(t, "parent", "2", false),
		newStringField(t, "isparent", "yes", false))
	mustAddDocuments(t, w, parentDoc)

	if _, err := w.DeleteDocuments(index.NewTerm("parent", "2")); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}

	parentDoc = newTestDocument(
		newStringField(t, "parent", "2", false),
		newStringField(t, "isparent", "yes", false))
	childDoc = newTestDocument(newStringField(t, "child", "2", false))
	mustAddDocuments(t, w, childDoc, parentDoc)

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcher(t, r)

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("isparent", "yes")))
	mustCheckJoinIndex(t, r, parentsFilter)

	parentQuery := search.NewTermQuery(index.NewTerm("parent", "2"))

	parentJoinQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter)
	topdocs := mustSearch(t, s, parentJoinQuery, 3)
	assertInt64Equals(t, 1, topdocs.TotalHits.Value)

	mustClose(t, r, dir)
}

func TestBlockJoinIntersectionWithRandomApproximation(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	numBlocks := atLeast(100)
	for i := 0; i < numBlocks; i++ {
		docs := make([]*document.Document, 0)
		numChildren := random().Intn(3)
		for j := 0; j < numChildren; j++ {
			v := "baz"
			if random().Intn(2) == 0 {
				v = "bar"
			}
			docs = append(docs, newTestDocument(mustStringFieldPlain(t, "foo_child", v)))
		}
		v := "baz"
		if random().Intn(2) == 0 {
			v = "bar"
		}
		parent := newTestDocument(
			mustStringFieldPlain(t, "parent", "true"),
			mustStringFieldPlain(t, "foo_parent", v))
		docs = append(docs, parent)
		mustAddDocuments(t, w, docs...)
	}
	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)
	searcher.SetQueryCache(nil) // to have real advance() calls

	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("parent", "true")))
	toChild := NewToChildBlockJoinQuery(search.NewTermQuery(index.NewTerm("foo_parent", "bar")), parentsFilter)
	childQuery := search.NewTermQuery(index.NewTerm("foo_child", "baz"))

	bq1 := search.NewBooleanQueryBuilder().Add(toChild, search.MUST).Add(childQuery, search.MUST).Build()
	bq2 := search.NewBooleanQueryBuilder().
		Add(toChild, search.MUST).
		Add(testsearch.NewRandomApproximationQuery(childQuery, random()), search.MUST).
		Build()

	assertIntEquals(t, mustCount(t, searcher, bq1), mustCount(t, searcher, bq2))

	mustClose(t, searcher.GetIndexReader(), w, dir)
}

// mustStringFieldPlain renders new StringField(String, String, Store.NO).
func mustStringFieldPlain(t testing.TB, name, value string) *document.StringField {
	t.Helper()
	f, err := document.NewStringField(name, value, false)
	if err != nil {
		t.Fatalf("new StringField: %v", err)
	}
	return f
}

// LUCENE-6588
// delete documents to simulate FilteredQuery applying a filter as acceptDocs
func TestBlockJoinParentScoringBug(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	mustAddDocuments(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))

	mustAddDocuments(t, w, makeJob(t, "java", 2006), makeJob(t, "ruby", 2005), makeResume(t, "Frank", "United States"))
	if _, err := w.DeleteDocuments(index.NewTerm("skill", "java")); err != nil { // delete the first child of every parent
		t.Fatalf("deleteDocuments: %v", err)
	}

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	parentQuery := search.NewPrefixQuery(index.NewTerm("country", "United"))

	toChildQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter)

	hits := mustSearch(t, s, toChildQuery, 10)
	assertIntEquals(t, len(hits.ScoreDocs), 2)
	for i, hit := range hits.ScoreDocs {
		if hit.Score == 0.0 {
			t.Fatalf("Failed to calculate score for hit #%d", i)
		}
	}

	mustClose(t, r, dir)
}

func TestBlockJoinToChildBlockJoinQueryExplain(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	mustAddDocuments(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))

	mustAddDocuments(t, w, makeJob(t, "java", 2006), makeJob(t, "ruby", 2005), makeResume(t, "Frank", "United States"))
	if _, err := w.DeleteDocuments(index.NewTerm("skill", "java")); err != nil { // delete the first child of every parent
		t.Fatalf("deleteDocuments: %v", err)
	}

	r := mustGetReader(t, w)
	mustClose(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	// Create a filter that defines "parent" documents in the index - in this case resumes
	parentsFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	parentQuery := search.NewPrefixQuery(index.NewTerm("country", "United"))

	toChildQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter)

	hits := mustSearch(t, s, toChildQuery, 10)
	assertIntEquals(t, len(hits.ScoreDocs), 2)
	for _, hit := range hits.ScoreDocs {
		if v := mustExplain(t, s, toChildQuery, hit.Doc).GetValue(); hit.Score != v {
			t.Fatalf("explain value: expected %v, got %v", hit.Score, v)
		}
	}

	mustClose(t, r, dir)
}

func TestBlockJoinToChildInitialAdvanceParentButNoKids(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// degenerate case: first doc has no children
	mustAddDocument(t, w, makeResume(t, "first", "nokids"))
	mustAddDocuments(t, w, makeJob(t, "job", 42), makeResume(t, "second", "haskid"))

	// single segment
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	r := mustGetReader(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	mustClose(t, w)

	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	parentQuery := search.NewTermQuery(index.NewTerm("docType", "resume"))

	parentJoinQuery := NewToChildBlockJoinQuery(parentQuery, parentFilter)

	weight := mustCreateWeight(t, s, mustRewrite(t, s, parentJoinQuery), randomSearchScoreMode(), 1)
	advancingScorer := mustScorer(t, weight, mustLeaves(t, s.GetIndexReader())[0])
	nextDocScorer := mustScorer(t, weight, mustLeaves(t, s.GetIndexReader())[0])

	firstKid := mustNextDoc(t, nextDocScorer.Iterator())
	if firstKid == search.NO_MORE_DOCS {
		t.Fatal("firstKid not found")
	}
	assertAdvance(t, advancingScorer.Iterator(), 0, firstKid)

	mustClose(t, r, dir)
}

func TestBlockJoinMultiChildQueriesOfDiffParentLevels(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// randomly generate resume->jobs[]->qualifications[]
	numResumes := atLeast(100)
	for r := 0; r < numResumes; r++ {
		docs := make([]*document.Document, 0)

		rv := nextInt(1, 10)
		numJobs := atLeast(10)
		for j := 0; j < numJobs; j++ {
			jv := nextInt(-10, -1) // neg so no overlap with q (both used for "year")

			numQualifications := atLeast(10)
			for q := 0; q < numQualifications; q++ {
				docs = append(docs, makeQualification(t, "q"+strconv.Itoa(q)+"_rv"+strconv.Itoa(rv)+"_jv"+strconv.Itoa(jv), q))
			}
			docs = append(docs, makeJob(t, "j"+strconv.Itoa(j), jv))
		}
		docs = append(docs, makeResume(t, "r"+strconv.Itoa(r), "rv"+strconv.Itoa(rv)))
		mustAddDocuments(t, w, docs...)
	}

	r := mustGetReader(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	mustClose(t, w)

	resumeFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "resume")))
	// anything with a skill is a job
	jobFilter := NewQueryBitSetProducer(search.NewPrefixQuery(index.NewTerm("skill", "")))

	numQueryIters := atLeast(1)
	for i := 0; i < numQueryIters; i++ {
		qjv := nextInt(-10, -1)
		qrv := nextInt(1, 10)

		resumeQuery := NewToChildBlockJoinQuery(search.NewTermQuery(index.NewTerm("country", "rv"+strconv.Itoa(qrv))), resumeFilter)

		jobQuery := NewToChildBlockJoinQuery(intPointNewRangeQuery(t, "year", qjv, qjv), jobFilter)

		fullQuery := search.NewBooleanQueryBuilder()
		fullQuery.Add(jobQuery, search.MUST)
		fullQuery.Add(resumeQuery, search.MUST)

		hits := mustSearch(t, s, fullQuery.Build(), 100) // NOTE: totally possible that we'll get no matches

		for _, sd := range hits.ScoreDocs {
			// since we're looking for children of jobs, all results must be qualifications
			f := storedDocument(t, mustStoredFields(t, r), sd.Doc).Get("qualification")
			if f == nil {
				t.Fatalf("%d has no qualification", sd.Doc)
			}
			q := f.StringValue()
			if !strings.Contains(q, "jv"+strconv.Itoa(qjv)) {
				t.Fatalf("%s MUST contain jv%d", q, qjv)
			}
			if !strings.Contains(q, "rv"+strconv.Itoa(qrv)) {
				t.Fatalf("%s MUST contain rv%d", q, qrv)
			}
		}
	}

	mustClose(t, r, dir)
}

func TestBlockJoinScoreMode(t *testing.T) {
	sim := search.NewSimilarityBase(
		func(stats *search.LuceneBasicStats, freq, docLen float64) float64 { return freq },
		nil,
		func() string { return "TestSim" })
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetSimilarity(sim)
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	mustAddDocuments(t, w,
		newTestDocument(newTextField(t, "foo", "bar bar", false)),
		newTestDocument(newTextField(t, "foo", "bar", false)),
		newTestDocument(),
		newTestDocument(newStringFieldBytes(t, "type", []byte("parent"), false)))
	reader := mustGetReader(t, w)
	mustClose(t, w)
	searcher := newSearcher(t, reader)
	searcher.SetSimilarity(sim)
	parents := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("type", "parent")))
	for _, scoreMode := range joinScoreModes {
		query := NewToParentBlockJoinQuery(search.NewTermQuery(index.NewTerm("foo", "bar")), parents, scoreMode)
		topDocs := mustSearch(t, searcher, query, 10)
		assertInt64Equals(t, 1, topDocs.TotalHits.Value)
		assertIntEquals(t, 3, topDocs.ScoreDocs[0].Doc)
		var expectedScore float32
		switch scoreMode {
		case Avg:
			expectedScore = 1.5
		case Max:
			expectedScore = 2
		case Min:
			expectedScore = 1
		case None:
			expectedScore = 0
		case Total:
			expectedScore = 3
		default:
			t.Fatal(util.NewAssertionError(scoreMode))
		}
		if expectedScore != topDocs.ScoreDocs[0].Score {
			t.Fatalf("scoreMode %v: expected %v, got %v", scoreMode, expectedScore, topDocs.ScoreDocs[0].Score)
		}
	}
	mustClose(t, reader, dir)
}
