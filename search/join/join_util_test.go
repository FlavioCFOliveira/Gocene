// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// This file is the port of
// lucene/join/src/test/org/apache/lucene/search/join/TestJoinUtil.java
// (Apache Lucene 10.5.0).

// verbose renders LuceneTestCase.VERBOSE (false by default).
const verbose = false

// joinScoreModes renders ScoreMode.values() in declaration order.
var joinScoreModes = []ScoreMode{None, Avg, Max, Total, Min}

// --- field helpers (Java `new XxxField(...)` constructors) --------------------

func mustTextField(t testing.TB, name, value string, stored bool) *document.TextField {
	t.Helper()
	f, err := document.NewTextField(name, value, stored)
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	return f
}

func mustStringField(t testing.TB, name, value string, stored bool) *document.StringField {
	t.Helper()
	f, err := document.NewStringField(name, value, stored)
	if err != nil {
		t.Fatalf("new StringField: %v", err)
	}
	return f
}

func mustSortedDVField(t testing.TB, name, value string) *document.SortedDocValuesField {
	t.Helper()
	f, err := document.NewSortedDocValuesField(name, []byte(value))
	if err != nil {
		t.Fatalf("new SortedDocValuesField: %v", err)
	}
	return f
}

// mustSortedSetDVField renders new SortedSetDocValuesField(name, new BytesRef(value)).
func mustSortedSetDVField(t testing.TB, name, value string) *document.SortedSetDocValuesField {
	t.Helper()
	f, err := document.NewSortedSetDocValuesField(name, [][]byte{[]byte(value)})
	if err != nil {
		t.Fatalf("new SortedSetDocValuesField: %v", err)
	}
	return f
}

func mustNumericDVField(t testing.TB, name string, value int64) *document.NumericDocValuesField {
	t.Helper()
	f, err := document.NewNumericDocValuesField(name, value)
	if err != nil {
		t.Fatalf("new NumericDocValuesField: %v", err)
	}
	return f
}

// mustSortedNumericDVField renders new SortedNumericDocValuesField(name, value).
func mustSortedNumericDVField(t testing.TB, name string, value int64) *document.SortedNumericDocValuesField {
	t.Helper()
	f, err := document.NewSortedNumericDocValuesField(name, []int64{value})
	if err != nil {
		t.Fatalf("new SortedNumericDocValuesField: %v", err)
	}
	return f
}

func mustFloatDVField(t testing.TB, name string, value float32) *document.FloatDocValuesField {
	t.Helper()
	f, err := document.NewFloatDocValuesField(name, value)
	if err != nil {
		t.Fatalf("new FloatDocValuesField: %v", err)
	}
	return f
}

func mustDoubleDVField(t testing.TB, name string, value float64) *document.DoubleDocValuesField {
	t.Helper()
	f, err := document.NewDoubleDocValuesField(name, value)
	if err != nil {
		t.Fatalf("new DoubleDocValuesField: %v", err)
	}
	return f
}

// --- JoinUtil call helpers -----------------------------------------------------

func mustJoin(t testing.TB, fromField string, multi bool, toField string, fromQuery search.Query,
	s *search.IndexSearcher, scoreMode ScoreMode) search.Query {
	t.Helper()
	q, err := CreateJoinQuery(fromField, multi, toField, fromQuery, s, scoreMode)
	if err != nil {
		t.Fatalf("JoinUtil.createJoinQuery: %v", err)
	}
	return q
}

func mustJoinGlobal(t testing.TB, joinField string, fromQuery, toQuery search.Query, s *search.IndexSearcher,
	scoreMode ScoreMode, ordinalMap *index.OrdinalMap) search.Query {
	t.Helper()
	q, err := CreateJoinQueryGlobalOrdinals(joinField, fromQuery, toQuery, s, scoreMode, ordinalMap)
	if err != nil {
		t.Fatalf("JoinUtil.createJoinQuery: %v", err)
	}
	return q
}

func mustJoinGlobalMinMax(t testing.TB, joinField string, fromQuery, toQuery search.Query, s *search.IndexSearcher,
	scoreMode ScoreMode, ordinalMap *index.OrdinalMap, min, max int) search.Query {
	t.Helper()
	q, err := CreateJoinQueryGlobalOrdinalsWithMinMax(joinField, fromQuery, toQuery, s, scoreMode, ordinalMap, min, max)
	if err != nil {
		t.Fatalf("JoinUtil.createJoinQuery: %v", err)
	}
	return q
}

// mustOrdinalMap renders OrdinalMap.build(null, values, PackedInts.DEFAULT)
// over the SortedDocValues of field in every leaf of r.
func mustOrdinalMap(t testing.TB, r index.IndexReaderInterface, field string) *index.OrdinalMap {
	t.Helper()
	leaves := mustLeaves(t, r)
	values := make([]index.SortedDocValues, len(leaves))
	for _, ctx := range leaves {
		v, err := index.GetSorted(ctx.LeafReader(), field)
		if err != nil {
			t.Fatalf("DocValues.getSorted: %v", err)
		}
		values[ctx.Ord] = v
	}
	om, err := index.BuildOrdinalMapFromSortedValues(nil, values, packed.Default)
	if err != nil {
		t.Fatalf("OrdinalMap.build: %v", err)
	}
	return om
}

func mustExplain(t testing.TB, s *search.IndexSearcher, q search.Query, doc int) search.Explanation {
	t.Helper()
	e, err := s.Explain(q, doc)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	return e
}

// storedID renders searcher.storedFields().document(doc).get("id").
func storedID(t testing.TB, s *search.IndexSearcher, doc int) string {
	t.Helper()
	f := storedDocument(t, mustStoredFields(t, s), doc).Get("id")
	if f == nil {
		return ""
	}
	return f.StringValue()
}

func assertTopDoc(t testing.TB, result *search.TopDocs, totalHits int64, docs ...int) {
	t.Helper()
	if result.TotalHits.Value != totalHits {
		t.Fatalf("totalHits: expected %d, got %d", totalHits, result.TotalHits.Value)
	}
	for i, d := range docs {
		if result.ScoreDocs[i].Doc != d {
			t.Fatalf("scoreDocs[%d].doc: expected %d, got %d", i, d, result.ScoreDocs[i].Doc)
		}
	}
}

func TestJoinUtilSimple(t *testing.T) {
	const idField = "id"
	const toField = "productId"

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// 0
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "description", "random text", false),
		mustTextField(t, "name", "name1", false),
		mustTextField(t, idField, "1", false),
		mustSortedDVField(t, idField, "1")))

	// 1
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "10.0", false),
		mustTextField(t, idField, "2", false),
		mustSortedDVField(t, idField, "2"),
		mustTextField(t, toField, "1", false),
		mustSortedDVField(t, toField, "1")))

	// 2
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "20.0", false),
		mustTextField(t, idField, "3", false),
		mustSortedDVField(t, idField, "3"),
		mustTextField(t, toField, "1", false),
		mustSortedDVField(t, toField, "1")))

	// 3
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "description", "more random text", false),
		mustTextField(t, "name", "name2", false),
		mustTextField(t, idField, "4", false),
		mustSortedDVField(t, idField, "4")))
	mustCommit(t, w)

	// 4
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "10.0", false),
		mustTextField(t, idField, "5", false),
		mustSortedDVField(t, idField, "5"),
		mustTextField(t, toField, "4", false),
		mustSortedDVField(t, toField, "4")))

	// 5
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "20.0", false),
		mustTextField(t, idField, "6", false),
		mustSortedDVField(t, idField, "6"),
		mustTextField(t, toField, "4", false),
		mustSortedDVField(t, toField, "4")))

	indexSearcher := search.NewIndexSearcher(mustGetReader(t, w))
	mustClose(t, w)

	// Search for product
	joinQuery := mustJoin(t, idField, false, toField,
		search.NewTermQuery(index.NewTerm("name", "name2")), indexSearcher, None)
	result := mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 4, 5)

	joinQuery = mustJoin(t, idField, false, toField,
		search.NewTermQuery(index.NewTerm("name", "name1")), indexSearcher, None)
	result = mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 1, 2)

	// Search for offer
	joinQuery = mustJoin(t, toField, false, idField,
		search.NewTermQuery(index.NewTerm("id", "5")), indexSearcher, None)
	result = mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 1, 3)

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

func TestJoinUtilSimpleOrdinalsJoin(t *testing.T) {
	const idField = "id"
	const productIdField = "productId"
	// A field indicating to what type a document belongs, which is then used to distinguish
	// between documents during joining.
	const typeField = "type"
	// A single sorted doc values field that holds the join values for all document types.
	// Typically during indexing a schema will automatically create this field with the values
	const joinField = idField + productIdField

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// 0
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, idField, "1", false),
		mustTextField(t, typeField, "product", false),
		mustTextField(t, "description", "random text", false),
		mustTextField(t, "name", "name1", false),
		mustSortedDVField(t, joinField, "1")))

	// 1
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, productIdField, "1", false),
		mustTextField(t, typeField, "price", false),
		mustTextField(t, "price", "10.0", false),
		mustSortedDVField(t, joinField, "1")))

	// 2
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, productIdField, "1", false),
		mustTextField(t, typeField, "price", false),
		mustTextField(t, "price", "20.0", false),
		mustSortedDVField(t, joinField, "1")))

	// 3
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, idField, "2", false),
		mustTextField(t, typeField, "product", false),
		mustTextField(t, "description", "more random text", false),
		mustTextField(t, "name", "name2", false),
		mustSortedDVField(t, joinField, "2")))
	mustCommit(t, w)

	// 4
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, productIdField, "2", false),
		mustTextField(t, typeField, "price", false),
		mustTextField(t, "price", "10.0", false),
		mustSortedDVField(t, joinField, "2")))

	// 5
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, productIdField, "2", false),
		mustTextField(t, typeField, "price", false),
		mustTextField(t, "price", "20.0", false),
		mustSortedDVField(t, joinField, "2")))

	indexSearcher := search.NewIndexSearcher(mustGetReader(t, w))
	mustClose(t, w)

	r := indexSearcher.GetIndexReader()
	ordinalMap := mustOrdinalMap(t, r, joinField)

	var toQuery search.Query = search.NewTermQuery(index.NewTerm(typeField, "price"))
	var fromQuery search.Query = search.NewTermQuery(index.NewTerm("name", "name2"))
	// Search for product and return prices
	joinQuery := mustJoinGlobal(t, joinField, fromQuery, toQuery, indexSearcher, None, ordinalMap)
	result := mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 4, 5)

	fromQuery = search.NewTermQuery(index.NewTerm("name", "name1"))
	joinQuery = mustJoinGlobal(t, joinField, fromQuery, toQuery, indexSearcher, None, ordinalMap)
	result = mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 1, 2)

	// Search for prices and return products
	fromQuery = search.NewTermQuery(index.NewTerm("price", "20.0"))
	toQuery = search.NewTermQuery(index.NewTerm(typeField, "product"))
	joinQuery = mustJoinGlobal(t, joinField, fromQuery, toQuery, indexSearcher, None, ordinalMap)
	result = mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 0, 3)

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

func TestJoinUtilOrdinalsJoinExplainNoMatches(t *testing.T) {
	const idField = "id"
	const productIdField = "productId"
	const typeField = "type"
	const joinField = idField + productIdField

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	w := mustNewIndexWriter(t, dir, iwc)

	// 0
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, idField, "1", false),
		mustTextField(t, typeField, "product", false),
		mustTextField(t, "description", "random text", false),
		mustTextField(t, "name", "name1", false),
		mustSortedDVField(t, joinField, "1")))

	// 1
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, idField, "2", false),
		mustTextField(t, typeField, "product", false),
		mustTextField(t, "description", "random text", false),
		mustTextField(t, "name", "name2", false),
		mustSortedDVField(t, joinField, "2")))

	// 2
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, productIdField, "1", false),
		mustTextField(t, typeField, "price", false),
		mustTextField(t, "price", "10.0", false),
		mustSortedDVField(t, joinField, "1")))

	// 3
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, productIdField, "2", false),
		mustTextField(t, typeField, "price", false),
		mustTextField(t, "price", "20.0", false),
		mustSortedDVField(t, joinField, "1")))

	if random().Intn(2) == 0 {
		if err := w.Flush(); err != nil {
			t.Fatalf("flush: %v", err)
		}
	}

	// 4
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, productIdField, "3", false),
		mustTextField(t, typeField, "price", false),
		mustTextField(t, "price", "5.0", false),
		mustSortedDVField(t, joinField, "2")))

	// 5
	mustAddDocument(t, w, newTestDocument(mustTextField(t, "field", "value", false)))

	r := mustOpenDirectoryReaderFromWriter(t, w)
	indexSearcher := search.NewIndexSearcher(r)
	ordinalMap := mustOrdinalMap(t, r, joinField)

	toQuery := search.NewTermQuery(index.NewTerm("price", "5.0"))
	fromQuery := search.NewTermQuery(index.NewTerm("name", "name2"))

	for _, scoreMode := range joinScoreModes {
		joinQuery := mustJoinGlobal(t, joinField, fromQuery, toQuery, indexSearcher, scoreMode, ordinalMap)
		result := mustSearch(t, indexSearcher, joinQuery, 10)
		assertTopDoc(t, result, 1, 4) // doc with price: 5.0
		explanation := mustExplain(t, indexSearcher, joinQuery, 4)
		if !explanation.IsMatch() {
			t.Fatalf("scoreMode=%v: doc 4 explanation must match", scoreMode)
		}
		if explanation.GetDescription() != "A match, join value 2" {
			t.Fatalf("scoreMode=%v: description %q", scoreMode, explanation.GetDescription())
		}

		explanation = mustExplain(t, indexSearcher, joinQuery, 3)
		if explanation.IsMatch() {
			t.Fatalf("scoreMode=%v: doc 3 explanation must not match", scoreMode)
		}
		if explanation.GetDescription() != "Not a match, join value 1" {
			t.Fatalf("scoreMode=%v: description %q", scoreMode, explanation.GetDescription())
		}

		explanation = mustExplain(t, indexSearcher, joinQuery, 5)
		if explanation.IsMatch() {
			t.Fatalf("scoreMode=%v: doc 5 explanation must not match", scoreMode)
		}
		if explanation.GetDescription() != "Not a match" {
			t.Fatalf("scoreMode=%v: description %q", scoreMode, explanation.GetDescription())
		}
	}

	mustClose(t, w, indexSearcher.GetIndexReader(), dir)
}

func TestJoinUtilRandomOrdinalsJoin(t *testing.T) {
	context := createContext(t, 128, false, true)
	searchIters := atLeast(1)
	indexSearcher := context.searcher
	for i := 0; i < searchIters; i++ {
		if verbose {
			fmt.Println("search iter=" + strconv.Itoa(i))
		}
		r := random().Intn(len(context.randomUniqueValues))
		from := context.randomFrom[r]
		randomValue := context.randomUniqueValues[r]
		expectedResult := createExpectedResult(t, randomValue, from, indexSearcher.GetIndexReader(), context)

		actualQuery := search.NewTermQuery(index.NewTerm("value", randomValue))
		scoreMode := joinScoreModes[random().Intn(len(joinScoreModes))]

		var joinQuery search.Query
		if from {
			fromQuery := search.NewBooleanQueryBuilder()
			fromQuery.Add(search.NewTermQuery(index.NewTerm("type", "from")), search.FILTER)
			fromQuery.Add(actualQuery, search.MUST)
			toQuery := search.NewTermQuery(index.NewTerm("type", "to"))
			joinQuery = mustJoinGlobal(t, "join_field", fromQuery.Build(), toQuery, indexSearcher, scoreMode,
				context.ordinalMap)
		} else {
			fromQuery := search.NewBooleanQueryBuilder()
			fromQuery.Add(search.NewTermQuery(index.NewTerm("type", "to")), search.FILTER)
			fromQuery.Add(actualQuery, search.MUST)
			toQuery := search.NewTermQuery(index.NewTerm("type", "from"))
			joinQuery = mustJoinGlobal(t, "join_field", fromQuery.Build(), toQuery, indexSearcher, scoreMode,
				context.ordinalMap)
		}

		searchResults := searchBitSetAndTopDocs(t, indexSearcher, joinQuery)
		actualResult := searchResults[0].(*util.FixedBitSet)
		assertBitSet(t, expectedResult, actualResult, indexSearcher)
		expectedTopDocs := createExpectedTopDocs(randomValue, from, scoreMode, context)
		actualTopDocs := searchResults[1].(*search.TopDocs)
		assertTopDocs(t, expectedTopDocs, actualTopDocs, scoreMode, indexSearcher, joinQuery)
	}
	context.close(t)
}

// searchBitSetAndTopDocs renders
//
//	indexSearcher.search(joinQuery, new MultiCollectorManager(
//	    new BitSetCollectorManager(indexSearcher.getIndexReader().maxDoc()),
//	    new TopScoreDocCollectorManager(10, null, Integer.MAX_VALUE)))
func searchBitSetAndTopDocs(t testing.TB, indexSearcher *search.IndexSearcher, joinQuery search.Query) []any {
	t.Helper()
	topManager, err := search.NewTopScoreDocCollectorManager(10, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("new TopScoreDocCollectorManager: %v", err)
	}
	mcm, err := search.NewMultiCollectorManager(
		search.AsAnyCollectorManager[*bitSetCollector, *util.FixedBitSet](
			&bitSetCollectorManager{maxDoc: indexSearcher.GetIndexReader().MaxDoc()}),
		search.AsAnyCollectorManager[*search.TopScoreDocCollector, *search.TopDocs](topManager))
	if err != nil {
		t.Fatalf("new MultiCollectorManager: %v", err)
	}
	results, err := search.SearchWithCollectorManager[search.Collector, []any](indexSearcher, joinQuery, mcm)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return results
}

func TestJoinUtilMinMaxScore(t *testing.T) {
	const priceField = "price"
	priceQuery := numericDocValuesScoreQuery(priceField)

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(
		testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	iw := newRandomIndexWriterWithConfig(t, dir, iwc)

	lowestScoresPerParent := map[string]float32{}
	highestScoresPerParent := map[string]float32{}
	numParents := nextInt(16, 64)
	for p := 0; p < numParents; p++ {
		parentID := strconv.Itoa(p)
		mustAddDocument(t, iw, newTestDocument(
			mustStringField(t, "id", parentID, true),
			mustStringField(t, "type", "to", false),
			mustSortedDVField(t, "join_field", parentID)))
		numChildren := nextInt(2, 16)
		lowest := math.MaxInt32
		highest := math.MinInt32
		for c := 0; c < numChildren; c++ {
			childID := strconv.Itoa(p + c)
			price := random().Intn(1000)
			mustAddDocument(t, iw, newTestDocument(
				mustStringField(t, "id", childID, true),
				mustStringField(t, "type", "from", false),
				mustSortedDVField(t, "join_field", parentID),
				mustNumericDVField(t, priceField, int64(price))))
			lowest = min(lowest, price)
			highest = max(highest, price)
		}
		lowestScoresPerParent[parentID] = float32(lowest)
		highestScoresPerParent[parentID] = float32(highest)
	}
	mustClose(t, iw)

	searcher := search.NewIndexSearcher(mustOpenDirectoryReader(t, dir))
	ordinalMap := mustOrdinalMap(t, searcher.GetIndexReader(), "join_field")
	fromQuery := search.NewBooleanQueryBuilder()
	fromQuery.Add(priceQuery, search.MUST)
	toQuery := search.NewTermQuery(index.NewTerm("type", "to"))
	joinQuery := mustJoinGlobal(t, "join_field", fromQuery.Build(), toQuery, searcher, Min, ordinalMap)
	topDocs := mustSearch(t, searcher, joinQuery, numParents)
	if topDocs.TotalHits.Value != int64(numParents) {
		t.Fatalf("totalHits: expected %d, got %d", numParents, topDocs.TotalHits.Value)
	}
	for _, scoreDoc := range topDocs.ScoreDocs {
		id := storedID(t, searcher, scoreDoc.Doc)
		if lowestScoresPerParent[id] != scoreDoc.Score {
			t.Fatalf("min score of parent %s: expected %v, got %v", id, lowestScoresPerParent[id], scoreDoc.Score)
		}
	}
	checkBoost(t, joinQuery, searcher)

	joinQuery = mustJoinGlobal(t, "join_field", fromQuery.Build(), toQuery, searcher, Max, ordinalMap)
	topDocs = mustSearch(t, searcher, joinQuery, numParents)
	if topDocs.TotalHits.Value != int64(numParents) {
		t.Fatalf("totalHits: expected %d, got %d", numParents, topDocs.TotalHits.Value)
	}
	for _, scoreDoc := range topDocs.ScoreDocs {
		id := storedID(t, searcher, scoreDoc.Doc)
		if highestScoresPerParent[id] != scoreDoc.Score {
			t.Fatalf("max score of parent %s: expected %v, got %v", id, highestScoresPerParent[id], scoreDoc.Score)
		}
	}
	checkBoost(t, joinQuery, searcher)

	mustClose(t, searcher.GetIndexReader(), dir)
}

// numericDocValuesScoreQuery renders the static numericDocValuesScoreQuery(String):
// "FunctionQuery would be helpful, but join module doesn't depend on queries module."
func numericDocValuesScoreQuery(field string) search.Query {
	return &numericDVScoreQuery{field: field, fieldQuery: search.NewFieldExistsQuery(field),
		identity: int(identityHashCodes.Add(1))}
}

// identityHashCodes renders System.identityHashCode: a distinct value per
// object.
var identityHashCodes atomic.Int32

// numericDVScoreQuery is the anonymous Query of numericDocValuesScoreQuery.
type numericDVScoreQuery struct {
	field      string
	fieldQuery search.Query
	identity   int
}

func (q *numericDVScoreQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	fieldWeight, err := q.fieldQuery.CreateWeight(searcher, search.COMPLETE_NO_SCORES, boost)
	if err != nil {
		return nil, err
	}
	return &numericDVScoreWeight{BaseWeight: search.NewBaseWeight(q), field: q.field, fieldWeight: fieldWeight}, nil
}

// Rewrite renders the inherited Query.rewrite(IndexSearcher): return this.
func (q *numericDVScoreQuery) Rewrite(*search.IndexSearcher) (search.Query, error) { return q, nil }

func (q *numericDVScoreQuery) Visit(search.QueryVisitor) {}

func (q *numericDVScoreQuery) ToString(field string) string {
	return q.fieldQuery.(interface{ ToString(string) string }).ToString(field)
}

func (q *numericDVScoreQuery) String() string { return q.ToString("") }

func (q *numericDVScoreQuery) Equals(o spi.Query) bool {
	other, ok := o.(*numericDVScoreQuery)
	return ok && other == q
}

// HashCode renders System.identityHashCode(this).
func (q *numericDVScoreQuery) HashCode() int { return q.identity }

// numericDVScoreWeight is the anonymous Weight of numericDocValuesScoreQuery.
type numericDVScoreWeight struct {
	*search.BaseWeight
	field       string
	fieldWeight search.Weight
}

func (w *numericDVScoreWeight) Explain(*index.LeafReaderContext, int) (search.Explanation, error) {
	return nil, nil
}

func (w *numericDVScoreWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	fieldScorer, err := w.fieldWeight.Scorer(context)
	if err != nil {
		return nil, err
	}
	if fieldScorer == nil {
		return nil, nil
	}
	price, err := context.LeafReader().GetNumericDocValues(w.field)
	if err != nil {
		return nil, err
	}
	scorer := &priceFilterScorer{FilterScorer: search.NewFilterScorer(fieldScorer), in: fieldScorer, price: price}
	return search.NewDefaultScorerSupplier(scorer), nil
}

// Scorer renders the inherited Weight.scorer(LeafReaderContext).
func (w *numericDVScoreWeight) Scorer(context *index.LeafReaderContext) (search.Scorer, error) {
	ss, err := w.ScorerSupplier(context)
	if err != nil || ss == nil {
		return nil, err
	}
	return ss.Get(math.MaxInt64)
}

// BulkScorer renders the inherited Weight.bulkScorer(LeafReaderContext).
func (w *numericDVScoreWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	ss, err := w.ScorerSupplier(context)
	if err != nil || ss == nil {
		return nil, err
	}
	return search.DefaultScorerSupplierBulkScorer(ss)
}

func (w *numericDVScoreWeight) IsCacheable(*index.LeafReaderContext) bool { return false }

// priceFilterScorer is the anonymous FilterScorer that scores by the price
// doc value.
type priceFilterScorer struct {
	*search.FilterScorer
	in    search.Scorer
	price index.NumericDocValues
}

func (s *priceFilterScorer) Score() (float32, error) {
	advanced, err := s.price.Advance(s.in.DocID())
	if err != nil {
		return 0, err
	}
	if advanced != s.in.DocID() {
		return 0, fmt.Errorf("assertEquals failed: expected %d, got %d", s.in.DocID(), advanced)
	}
	v, err := s.price.LongValue()
	if err != nil {
		return 0, err
	}
	return float32(v), nil
}

func (s *priceFilterScorer) GetMaxScore(int) (float32, error) { return float32(math.Inf(1)), nil }

func TestJoinUtilMinMaxDocs(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(
		testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	iw := newRandomIndexWriterWithConfig(t, dir, iwc)

	minChildDocsPerParent := 2
	maxChildDocsPerParent := 16
	numParents := nextInt(16, 64)
	childDocsPerParent := make([]int, numParents)
	for p := 0; p < numParents; p++ {
		parentID := strconv.Itoa(p)
		mustAddDocument(t, iw, newTestDocument(
			mustStringField(t, "id", parentID, true),
			mustStringField(t, "type", "to", false),
			mustSortedDVField(t, "join_field", parentID)))
		numChildren := nextInt(minChildDocsPerParent, maxChildDocsPerParent)
		childDocsPerParent[p] = numChildren
		for c := 0; c < numChildren; c++ {
			childID := strconv.Itoa(p + c)
			mustAddDocument(t, iw, newTestDocument(
				mustStringField(t, "id", childID, true),
				mustStringField(t, "type", "from", false),
				mustSortedDVField(t, "join_field", parentID)))
		}
	}
	mustClose(t, iw)

	searcher := search.NewIndexSearcher(mustOpenDirectoryReader(t, dir))
	ordinalMap := mustOrdinalMap(t, searcher.GetIndexReader(), "join_field")
	fromQuery := search.NewTermQuery(index.NewTerm("type", "from"))
	toQuery := search.NewTermQuery(index.NewTerm("type", "to"))

	iters := nextInt(3, 9)
	for i := 1; i <= iters; i++ {
		scoreMode := joinScoreModes[random().Intn(len(joinScoreModes))]
		minV := nextInt(minChildDocsPerParent, maxChildDocsPerParent-1)
		maxV := nextInt(minV, maxChildDocsPerParent)
		if verbose {
			fmt.Printf("iter=%d\nscoreMode=%v\nmin=%d\nmax=%d\n", i, scoreMode, minV, maxV)
		}
		joinQuery := mustJoinGlobalMinMax(t, "join_field", fromQuery, toQuery, searcher, scoreMode, ordinalMap, minV, maxV)
		totalHits := mustCount(t, searcher, joinQuery)
		expectedCount := 0
		for _, numChildDocs := range childDocsPerParent {
			if numChildDocs >= minV && numChildDocs <= maxV {
				expectedCount++
			}
		}
		if expectedCount != totalHits {
			t.Fatalf("iter %d: expected %d hits, got %d", i, expectedCount, totalHits)
		}
		checkBoost(t, joinQuery, searcher)
	}
	mustClose(t, searcher.GetIndexReader(), dir)
}

func TestJoinUtilRewrite(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	mustAddDocument(t, w, newTestDocument(mustSortedDVField(t, "join_field", "abc")))
	mustAddDocument(t, w, newTestDocument(mustSortedDVField(t, "join_field", "abd")))
	reader := mustGetReader(t, w)
	searcher := newSearcher(t, reader)
	ordMap, err := index.BuildOrdinalMapFromSortedValues(nil, []index.SortedDocValues{}, 0)
	if err != nil {
		t.Fatalf("OrdinalMap.build: %v", err)
	}
	{
		joinQuery := mustJoinGlobalMinMax(t, "join_field", search.MatchNoDocsQueryInstance,
			search.MatchNoDocsQueryInstance, searcher, joinScoreModes[random().Intn(len(joinScoreModes))],
			ordMap, 0, math.MaxInt32)
		mustSearch(t, searcher, joinQuery, 1) // no exception due to missing rewrites
	}
	{
		joinQuery := mustJoinGlobalMinMax(t, "join_field", search.MatchNoDocsQueryInstance,
			search.MatchNoDocsQueryInstance, searcher, None, ordMap, 1, math.MaxInt32)
		rewritten := mustRewrite(t, searcher, joinQuery)
		// should simplify to GlobalOrdinalsQuery since min is set to 1
		if _, ok := rewritten.(*GlobalOrdinalsQuery); !ok {
			t.Fatalf("expected GlobalOrdinalsQuery, got %T", rewritten)
		}
		mustSearch(t, searcher, joinQuery, 1) // no exception due to missing rewrites
	}
	mustClose(t, reader, w, dir)
}

// TermsWithScoreCollector.MV.Avg forgets to grow beyond
// TermsWithScoreCollector.INITIAL_ARRAY_SIZE
func TestJoinUtilOverflowTermsWithScoreCollector(t *testing.T) {
	test300spartans(t, true, Avg)
}

func TestJoinUtilOverflowTermsWithScoreCollectorRandom(t *testing.T) {
	test300spartans(t, random().Intn(2) == 0, joinScoreModes[random().Intn(len(joinScoreModes))])
}

func test300spartans(t *testing.T, multipleValues bool, scoreMode ScoreMode) {
	const idField = "id"
	const toField = "productId"

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// 0
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "description", "random text", false),
		mustTextField(t, "name", "name1", false),
		mustTextField(t, idField, "0", false),
		mustSortedDVField(t, idField, "0")))

	doc := newTestDocument(mustTextField(t, "price", "10.0", false))
	if multipleValues {
		for i := 0; i < 300; i++ {
			doc.Add(mustSortedSetDVField(t, toField, strconv.Itoa(i)))
		}
	} else {
		doc.Add(mustSortedDVField(t, toField, "0"))
	}
	mustAddDocument(t, w, doc)

	indexSearcher := search.NewIndexSearcher(mustGetReader(t, w))
	mustClose(t, w)

	// Search for product
	joinQuery := mustJoin(t, toField, multipleValues, idField,
		search.NewTermQuery(index.NewTerm("price", "10.0")), indexSearcher, scoreMode)

	result := mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 1, 0)

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

// LUCENE-5487: verify a join query inside a SHOULD BQ will still use the join
// query's optimized BulkScorers
func TestJoinUtilInsideBooleanQuery(t *testing.T) {
	const idField = "id"
	const toField = "productId"

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// 0
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "description", "random text", false),
		mustTextField(t, "name", "name1", false),
		mustTextField(t, idField, "7", false),
		mustSortedDVField(t, idField, "7")))

	// 1
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "10.0", false),
		mustTextField(t, idField, "2", false),
		mustSortedDVField(t, idField, "2"),
		mustTextField(t, toField, "7", false)))

	// 2
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "20.0", false),
		mustTextField(t, idField, "3", false),
		mustSortedDVField(t, idField, "3"),
		mustTextField(t, toField, "7", false)))

	// 3
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "description", "more random text", false),
		mustTextField(t, "name", "name2", false),
		mustTextField(t, idField, "0", false),
		mustSortedDVField(t, idField, "0")))
	mustCommit(t, w)

	// 4
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "10.0", false),
		mustTextField(t, idField, "5", false),
		mustSortedDVField(t, idField, "5"),
		mustTextField(t, toField, "0", false)))

	// 5
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "price", "20.0", false),
		mustTextField(t, idField, "6", false),
		mustSortedDVField(t, idField, "6"),
		mustTextField(t, toField, "0", false)))

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	// single segment test, that relies on sequential collection, the searcher is also created
	// without an executor
	indexSearcher := search.NewIndexSearcher(mustGetReader(t, w))
	mustClose(t, w)

	// Search for product
	joinQuery := mustJoin(t, idField, false, toField,
		search.NewTermQuery(index.NewTerm("description", "random")), indexSearcher, Avg)

	bq := search.NewBooleanQueryBuilder()
	bq.Add(joinQuery, search.SHOULD)
	bq.Add(search.NewTermQuery(index.NewTerm("id", "3")), search.SHOULD)

	if _, err := search.SearchWithCollectorManager[*sawFiveCollector, struct{}](
		indexSearcher, bq.Build(), sawFiveCollectorManager{}); err != nil {
		t.Fatalf("search: %v", err)
	}

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

// sawFiveCollectorManager is the anonymous CollectorManager<SimpleCollector, Void>
// of testInsideBooleanQuery.
type sawFiveCollectorManager struct{}

func (sawFiveCollectorManager) NewCollector() (*sawFiveCollector, error) {
	c := &sawFiveCollector{}
	c.Outer = c
	return c, nil
}

func (sawFiveCollectorManager) Reduce(collectors []*sawFiveCollector) (struct{}, error) {
	if util.AssertsEnabled() && len(collectors) != 1 {
		return struct{}{}, util.NewAssertionError("concurrent execution is not supported by this test")
	}
	return struct{}{}, nil
}

// sawFiveCollector is the anonymous SimpleCollector of testInsideBooleanQuery.
type sawFiveCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	sawFive bool
}

func (c *sawFiveCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	return c, nil
}

func (c *sawFiveCollector) Collect(docID int) error {
	// Hairy / evil (depends on how BooleanScorer
	// stores temporarily collected docIDs by
	// appending to head of linked list):
	if docID == 5 {
		c.sawFive = true
	} else if docID == 1 {
		if c.sawFive {
			return fmt.Errorf("optimized bulkScorer was not used for join query embedded in boolean query!")
		}
	}
	return nil
}

func (c *sawFiveCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (c *sawFiveCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *sawFiveCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func TestJoinUtilSimpleWithScoring(t *testing.T) {
	const idField = "id"
	const toField = "movieId"

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// 0
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "description", "A random movie", false),
		mustTextField(t, "name", "Movie 1", false),
		mustTextField(t, idField, "1", false),
		mustSortedDVField(t, idField, "1")))

	// 1
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "subtitle", "The first subtitle of this movie", false),
		mustTextField(t, idField, "2", false),
		mustSortedDVField(t, idField, "2"),
		mustTextField(t, toField, "1", false),
		mustSortedDVField(t, toField, "1")))

	// 2
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "subtitle", "random subtitle; random event movie", false),
		mustTextField(t, idField, "3", false),
		mustSortedDVField(t, idField, "3"),
		mustTextField(t, toField, "1", false),
		mustSortedDVField(t, toField, "1")))

	// 3
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "description", "A second random movie", false),
		mustTextField(t, "name", "Movie 2", false),
		mustTextField(t, idField, "4", false),
		mustSortedDVField(t, idField, "4")))
	mustCommit(t, w)

	// 4
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "subtitle", "a very random event happened during christmas night", false),
		mustTextField(t, idField, "5", false),
		mustSortedDVField(t, idField, "5"),
		mustTextField(t, toField, "4", false),
		mustSortedDVField(t, toField, "4")))

	// 5
	mustAddDocument(t, w, newTestDocument(
		mustTextField(t, "subtitle", "movie end movie test 123 test 123 random", false),
		mustTextField(t, idField, "6", false),
		mustSortedDVField(t, idField, "6"),
		mustTextField(t, toField, "4", false),
		mustSortedDVField(t, toField, "4")))

	indexSearcher := search.NewIndexSearcher(mustGetReader(t, w))
	mustClose(t, w)

	// Search for movie via subtitle
	joinQuery := mustJoin(t, toField, false, idField,
		search.NewTermQuery(index.NewTerm("subtitle", "random")), indexSearcher, Max)
	result := mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 0, 3)
	checkBoost(t, joinQuery, indexSearcher)

	// Score mode max.
	joinQuery = mustJoin(t, toField, false, idField,
		search.NewTermQuery(index.NewTerm("subtitle", "movie")), indexSearcher, Max)
	result = mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 3, 0)
	checkBoost(t, joinQuery, indexSearcher)

	// Score mode total
	joinQuery = mustJoin(t, toField, false, idField,
		search.NewTermQuery(index.NewTerm("subtitle", "movie")), indexSearcher, Total)
	result = mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 0, 3)
	checkBoost(t, joinQuery, indexSearcher)

	// Score mode avg
	joinQuery = mustJoin(t, toField, false, idField,
		search.NewTermQuery(index.NewTerm("subtitle", "movie")), indexSearcher, Avg)
	result = mustSearch(t, indexSearcher, joinQuery, 10)
	assertTopDoc(t, result, 2, 3, 0)
	checkBoost(t, joinQuery, indexSearcher)

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

func checkBoost(t testing.TB, query search.Query, searcher *search.IndexSearcher) {
	t.Helper()
	result := mustSearch(t, searcher, query, 10)
	if len(result.ScoreDocs) == 0 {
		return
	}
	boostedQuery := search.NewBoostQuery(query, 10)
	boostedResult := mustSearch(t, searcher, boostedQuery, 10)
	if d := math.Abs(float64(result.ScoreDocs[0].Score*10 - boostedResult.ScoreDocs[0].Score)); d > 0.000001 {
		t.Fatalf("boosted score: expected %v, got %v", result.ScoreDocs[0].Score*10, boostedResult.ScoreDocs[0].Score)
	}
	queryUtilsCheckExplanations(t, boostedQuery, searcher)
}

// pickTwoScoreModes renders EnumSet.allOf(ScoreMode.class), RandomPicks.randomFrom,
// set.remove(first), RandomPicks.randomFrom: two distinct score modes.
func pickTwoScoreModes(r *rand.Rand) (ScoreMode, ScoreMode) {
	scoreModes := append([]ScoreMode(nil), joinScoreModes...)
	i := r.Intn(len(scoreModes))
	scoreMode1 := scoreModes[i]
	scoreModes = append(scoreModes[:i], scoreModes[i+1:]...)
	scoreMode2 := scoreModes[r.Intn(len(scoreModes))]
	return scoreMode1, scoreMode2
}

func TestJoinUtilEquals(t *testing.T) {
	numDocs := atLeast(50)
	dir := newDirectory()
	defer mustClose(t, dir)
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	defer mustClose(t, w)
	multiValued := random().Intn(2) == 0
	joinField := "svField"
	if multiValued {
		joinField = "mvField"
	}
	for id := 0; id < numDocs; id++ {
		doc := newTestDocument(
			mustTextField(t, "id", strconv.Itoa(id), false),
			mustTextField(t, "name", "name"+strconv.Itoa(id%7), false))
		if multiValued {
			numValues := 1 + random().Intn(2)
			for i := 0; i < numValues; i++ {
				doc.Add(mustSortedSetDVField(t, joinField, strconv.Itoa(random().Intn(13))))
			}
		} else {
			doc.Add(mustSortedDVField(t, joinField, strconv.Itoa(random().Intn(13))))
		}
		mustAddDocument(t, w, doc)
	}

	scoreMode1, scoreMode2 := pickTwoScoreModes(random())

	var x search.Query
	func() {
		r := mustGetReader(t, w)
		defer mustClose(t, r)
		indexSearcher := search.NewIndexSearcher(r)
		x = mustJoin(t, joinField, multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)
		if !x.Equals(mustJoin(t, joinField, multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
			t.Fatal("identical calls to createJoinQuery")
		}

		if x.Equals(mustJoin(t, joinField, multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode2)) {
			t.Fatalf("score mode (%v != %v), but queries are equal", scoreMode1, scoreMode2)
		}

		if x.Equals(mustJoin(t, joinField, multiValued, "other_field",
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
			t.Fatal("from fields (joinField != \"other_field\") but queries equals")
		}

		if x.Equals(mustJoin(t, "other_field", multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
			t.Fatal("from fields (\"other_field\" != joinField) but queries equals")
		}

		if x.Equals(mustJoin(t, "other_field", multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name6")), indexSearcher, scoreMode1)) {
			t.Fatal("fromQuery (name:name5 != name:name6) but queries equals")
		}
	}()

	for i := 0; i < 13; i++ {
		doc := newTestDocument(
			mustTextField(t, "id", "new_id", false),
			mustTextField(t, "name", "name5", false))
		if multiValued {
			numValues := 1 + random().Intn(2)
			for j := 0; j < numValues; j++ {
				doc.Add(mustSortedSetDVField(t, joinField, strconv.Itoa(i)))
			}
		} else {
			doc.Add(mustSortedDVField(t, joinField, strconv.Itoa(i)))
		}
		mustAddDocument(t, w, doc)
	}
	r := mustGetReader(t, w)
	defer mustClose(t, r)
	indexSearcher := search.NewIndexSearcher(r)
	if x.Equals(mustJoin(t, joinField, multiValued, joinField,
		search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
		t.Fatal("Query shouldn't be equal, because different index readers ")
	}
}

func TestJoinUtilEquals_globalOrdinalsJoin(t *testing.T) {
	numDocs := atLeast(50)
	dir := newDirectory()
	defer mustClose(t, dir)
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	defer mustClose(t, w)
	const joinField = "field"
	for id := 0; id < numDocs; id++ {
		mustAddDocument(t, w, newTestDocument(
			mustTextField(t, "id", strconv.Itoa(id), false),
			mustTextField(t, "name", "name"+strconv.Itoa(id%7), false),
			mustSortedDVField(t, joinField, strconv.Itoa(random().Intn(13)))))
	}

	scoreMode1, scoreMode2 := pickTwoScoreModes(random())

	var x search.Query
	func() {
		r := mustGetReader(t, w)
		defer mustClose(t, r)
		ordinalMap := mustOrdinalMap(t, r, joinField)
		indexSearcher := search.NewIndexSearcher(r)
		x = mustJoinGlobal(t, joinField, search.NewTermQuery(index.NewTerm("name", "name5")),
			search.Instance, indexSearcher, scoreMode1, ordinalMap)
		if !x.Equals(mustJoinGlobal(t, joinField, search.NewTermQuery(index.NewTerm("name", "name5")),
			search.Instance, indexSearcher, scoreMode1, ordinalMap)) {
			t.Fatal("identical calls to createJoinQuery")
		}

		if x.Equals(mustJoinGlobal(t, joinField, search.NewTermQuery(index.NewTerm("name", "name5")),
			search.Instance, indexSearcher, scoreMode2, ordinalMap)) {
			t.Fatalf("score mode (%v != %v), but queries are equal", scoreMode1, scoreMode2)
		}
		if x.Equals(mustJoinGlobal(t, joinField, search.NewTermQuery(index.NewTerm("name", "name6")),
			search.Instance, indexSearcher, scoreMode1, ordinalMap)) {
			t.Fatal("fromQuery (name:name5 != name:name6) but queries equals")
		}
	}()

	for i := 0; i < 13; i++ {
		mustAddDocument(t, w, newTestDocument(
			mustTextField(t, "id", "new_id", false),
			mustTextField(t, "name", "name5", false),
			mustSortedDVField(t, joinField, strconv.Itoa(i))))
	}
	r := mustGetReader(t, w)
	defer mustClose(t, r)
	ordinalMap := mustOrdinalMap(t, r, joinField)
	indexSearcher := search.NewIndexSearcher(r)
	if x.Equals(mustJoinGlobal(t, joinField, search.NewTermQuery(index.NewTerm("name", "name5")),
		search.Instance, indexSearcher, scoreMode1, ordinalMap)) {
		t.Fatal("Query shouldn't be equal, because different index readers ")
	}
}

func mustJoinNumeric(t testing.TB, fromField string, multi bool, toField string, fromQuery search.Query,
	s *search.IndexSearcher, scoreMode ScoreMode) search.Query {
	t.Helper()
	q, err := CreateJoinQueryWithNumericType(fromField, multi, toField, JoinNumericTypeInteger, fromQuery, s, scoreMode)
	if err != nil {
		t.Fatalf("JoinUtil.createJoinQuery: %v", err)
	}
	return q
}

func TestJoinUtilEquals_numericJoin(t *testing.T) {
	numDocs := atLeast(50)
	dir := newDirectory()
	defer mustClose(t, dir)
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	defer mustClose(t, w)
	multiValued := random().Intn(2) == 0
	joinField := "svField"
	if multiValued {
		joinField = "mvField"
	}
	for id := 0; id < numDocs; id++ {
		doc := newTestDocument(
			mustTextField(t, "id", strconv.Itoa(id), false),
			mustTextField(t, "name", "name"+strconv.Itoa(id%7), false))
		if multiValued {
			numValues := 1 + random().Intn(2)
			for i := 0; i < numValues; i++ {
				doc.Add(document.NewIntPoint(joinField, int32(random().Intn(13))))
				doc.Add(mustSortedNumericDVField(t, joinField, int64(random().Intn(13))))
			}
		} else {
			doc.Add(document.NewIntPoint(joinField, int32(random().Intn(13))))
			doc.Add(mustNumericDVField(t, joinField, int64(random().Intn(13))))
		}
		mustAddDocument(t, w, doc)
	}

	scoreMode1, scoreMode2 := pickTwoScoreModes(random())

	var x search.Query
	func() {
		r := mustGetReader(t, w)
		defer mustClose(t, r)
		indexSearcher := search.NewIndexSearcher(r)
		x = mustJoinNumeric(t, joinField, multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)
		if !x.Equals(mustJoinNumeric(t, joinField, multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
			t.Fatal("identical calls to createJoinQuery")
		}

		if x.Equals(mustJoinNumeric(t, joinField, multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode2)) {
			t.Fatalf("score mode (%v != %v), but queries are equal", scoreMode1, scoreMode2)
		}

		if x.Equals(mustJoinNumeric(t, joinField, multiValued, "other_field",
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
			t.Fatal("from fields (joinField != \"other_field\") but queries equals")
		}

		if x.Equals(mustJoinNumeric(t, "other_field", multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
			t.Fatal("from fields (\"other_field\" != joinField) but queries equals")
		}

		if x.Equals(mustJoinNumeric(t, "other_field", multiValued, joinField,
			search.NewTermQuery(index.NewTerm("name", "name6")), indexSearcher, scoreMode1)) {
			t.Fatal("fromQuery (name:name5 != name:name6) but queries equals")
		}
	}()

	for i := 14; i < 26; i++ {
		doc := newTestDocument(
			mustTextField(t, "id", "new_id", false),
			mustTextField(t, "name", "name5", false))
		if multiValued {
			numValues := 1 + random().Intn(2)
			for j := 0; j < numValues; j++ {
				doc.Add(mustSortedNumericDVField(t, joinField, int64(i)))
				doc.Add(document.NewIntPoint(joinField, int32(i)))
			}
		} else {
			doc.Add(mustNumericDVField(t, joinField, int64(i)))
			doc.Add(document.NewIntPoint(joinField, int32(i)))
		}
		mustAddDocument(t, w, doc)
	}
	r := mustGetReader(t, w)
	defer mustClose(t, r)
	indexSearcher := search.NewIndexSearcher(r)
	if x.Equals(mustJoinNumeric(t, joinField, multiValued, joinField,
		search.NewTermQuery(index.NewTerm("name", "name5")), indexSearcher, scoreMode1)) {
		t.Fatal("Query shouldn't be equal, because new join values have been indexed")
	}
}

func TestJoinUtilSingleValueRandomJoin(t *testing.T) {
	maxIndexIter := atLeast(1)
	maxSearchIter := atLeast(1)
	executeRandomJoin(t, false, maxIndexIter, maxSearchIter, nextInt(87, 764))
}

// This test really takes more time, that is why the number of iterations are smaller.
func TestJoinUtilMultiValueRandomJoin(t *testing.T) {
	maxIndexIter := atLeast(1)
	maxSearchIter := atLeast(1)
	executeRandomJoin(t, true, maxIndexIter, maxSearchIter, nextInt(11, 57))
}

func executeRandomJoin(t *testing.T, multipleValuesPerDocument bool, maxIndexIter, maxSearchIter,
	numberOfDocumentsToIndex int) {
	for indexIter := 1; indexIter <= maxIndexIter; indexIter++ {
		if verbose {
			fmt.Printf("TEST: indexIter=%d numDocs=%d\n", indexIter, numberOfDocumentsToIndex)
		}
		context := createContext(t, numberOfDocumentsToIndex, multipleValuesPerDocument, false)
		indexSearcher := context.searcher
		for searchIter := 1; searchIter <= maxSearchIter; searchIter++ {
			r := random().Intn(len(context.randomUniqueValues))
			from := context.randomFrom[r]
			randomValue := context.randomUniqueValues[r]
			expectedResult := createExpectedResult(t, randomValue, from, indexSearcher.GetIndexReader(), context)

			actualQuery := search.NewTermQuery(index.NewTerm("value", randomValue))
			scoreMode := joinScoreModes[random().Intn(len(joinScoreModes))]

			var joinQuery search.Query
			{
				// single val can be handled by multiple-vals
				multiValsQuery := multipleValuesPerDocument || random().Intn(2) == 0
				fromField := "to"
				toField := "from"
				if from {
					fromField = "from"
					toField = "to"
				}

				surpriseMe := random().Intn(2)
				switch surpriseMe {
				case 0:
					var numType = JoinNumericTypeDouble
					suffix := "DOUBLE"
					if random().Intn(2) == 0 {
						numType = JoinNumericTypeInteger
						suffix = "INT"
					} else if random().Intn(2) == 0 {
						numType = JoinNumericTypeFloat
						suffix = "FLOAT"
					} else if random().Intn(2) == 0 {
						numType = JoinNumericTypeLong
						suffix = "LONG"
					}
					q, err := CreateJoinQueryWithNumericType(fromField+suffix, multiValsQuery, toField+suffix,
						numType, actualQuery, indexSearcher, scoreMode)
					if err != nil {
						t.Fatalf("JoinUtil.createJoinQuery: %v", err)
					}
					joinQuery = q
				case 1:
					joinQuery = mustJoin(t, fromField, multiValsQuery, toField, actualQuery, indexSearcher, scoreMode)
				default:
					t.Fatalf("unexpected value %d", surpriseMe)
				}
			}

			// Need to know all documents that have matches. TopDocs doesn't give me that and then
			// I'd be also testing TopDocsCollector...
			searchResults := searchBitSetAndTopDocs(t, indexSearcher, joinQuery)
			// Asserting bit set...
			assertBitSet(t, expectedResult, searchResults[0].(*util.FixedBitSet), indexSearcher)
			// Asserting TopDocs...
			expectedTopDocs := createExpectedTopDocs(randomValue, from, scoreMode, context)
			actualTopDocs := searchResults[1].(*search.TopDocs)
			assertTopDocs(t, expectedTopDocs, actualTopDocs, scoreMode, indexSearcher, joinQuery)
		}
		context.close(t)
	}
}

func assertBitSet(t testing.TB, expectedResult, actualResult *util.FixedBitSet, indexSearcher *search.IndexSearcher) {
	t.Helper()
	if !expectedResult.Equals(actualResult) {
		t.Fatalf("bit sets differ: expected cardinality %d, actual cardinality %d",
			expectedResult.Cardinality(), actualResult.Cardinality())
	}
}

func assertTopDocs(t testing.TB, expectedTopDocs, actualTopDocs *search.TopDocs, scoreMode ScoreMode,
	indexSearcher *search.IndexSearcher, joinQuery search.Query) {
	t.Helper()
	if expectedTopDocs.TotalHits.Value != actualTopDocs.TotalHits.Value {
		t.Fatalf("totalHits: expected %d, got %d", expectedTopDocs.TotalHits.Value, actualTopDocs.TotalHits.Value)
	}
	if len(expectedTopDocs.ScoreDocs) != len(actualTopDocs.ScoreDocs) {
		t.Fatalf("scoreDocs.length: expected %d, got %d", len(expectedTopDocs.ScoreDocs), len(actualTopDocs.ScoreDocs))
	}
	if scoreMode == None {
		return
	}

	for i := range expectedTopDocs.ScoreDocs {
		if expectedTopDocs.ScoreDocs[i].Doc != actualTopDocs.ScoreDocs[i].Doc {
			t.Fatalf("scoreDocs[%d].doc: expected %d, got %d", i, expectedTopDocs.ScoreDocs[i].Doc,
				actualTopDocs.ScoreDocs[i].Doc)
		}
		if expectedTopDocs.ScoreDocs[i].Score != actualTopDocs.ScoreDocs[i].Score {
			t.Fatalf("scoreDocs[%d].score: expected %v, got %v", i, expectedTopDocs.ScoreDocs[i].Score,
				actualTopDocs.ScoreDocs[i].Score)
		}
		explanation := mustExplain(t, indexSearcher, joinQuery, expectedTopDocs.ScoreDocs[i].Doc)
		if expectedTopDocs.ScoreDocs[i].Score != explanation.GetValue() {
			t.Fatalf("explanation[%d] value: expected %v, got %v", i, expectedTopDocs.ScoreDocs[i].Score,
				explanation.GetValue())
		}
	}
}

// shuffle renders Collections.shuffle(List, Random).
func shuffle(list []string, r *rand.Rand) {
	for i := len(list); i > 1; i-- {
		j := r.Intn(i)
		list[i-1], list[j] = list[j], list[i-1]
	}
}

func createContext(t *testing.T, nDocs int, multipleValuesPerDocument, globalOrdinalJoin bool) *indexIterationContext {
	t.Helper()
	if globalOrdinalJoin && multipleValuesPerDocument {
		t.Fatal("ordinal join doesn't support multiple join values per document")
	}

	dir := newDirectory()
	rnd := random()
	iwc := newIndexWriterConfigWithAnalyzer(
		testanalysis.NewMockAnalyzer(testanalysis.KEYWORD, false, 0, nil, true))
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	context := newIndexIterationContext()
	numRandomValues := nDocs / nextInt(1, 4)
	context.randomUniqueValues = make([]string, numRandomValues)
	trackSet := map[string]struct{}{}
	context.randomFrom = make([]bool, numRandomValues)
	for i := 0; i < numRandomValues; i++ {
		var uniqueRandomValue string
		for {
			// the trick is to generate values which will be ordered similarly for string,
			// ints&longs, positive nums makes it easier
			//
			// Additionally in order to avoid precision loss when joining via a float field we
			// can't generate values higher than 0xFFFFFF, so we can't use Integer#MAX_VALUE as
			// upper bound here:
			nextIntV := rnd.Intn(0xFFFFFF)
			uniqueRandomValue = fmt.Sprintf("%08x", nextIntV)
			if util.AssertsEnabled() {
				parsed, err := strconv.ParseUint(uniqueRandomValue, 16, 32)
				if err != nil || int(parsed) != nextIntV {
					t.Fatal(util.NewAssertionError(uniqueRandomValue))
				}
			}
			if _, seen := trackSet[uniqueRandomValue]; uniqueRandomValue != "" && !seen {
				break
			}
		}

		// Generate unique values and empty strings aren't allowed.
		trackSet[uniqueRandomValue] = struct{}{}

		context.randomFrom[i] = rnd.Intn(2) == 0
		context.randomUniqueValues[i] = uniqueRandomValue
	}

	randomUniqueValuesReplica := append([]string(nil), context.randomUniqueValues...)

	docs := make([]*randomDoc, nDocs)
	for i := 0; i < nDocs; i++ {
		id := strconv.Itoa(i)
		randomI := rnd.Intn(len(context.randomUniqueValues))
		value := context.randomUniqueValues[randomI]
		doc := newTestDocument(
			newTextField(t, "id", id, true),
			newTextField(t, "value", value, false))

		from := context.randomFrom[randomI]
		numberOfLinkValues := 1
		if multipleValuesPerDocument {
			numberOfLinkValues = min(2+rnd.Intn(10), len(context.randomUniqueValues))
		}
		docs[i] = newRandomDoc(id, numberOfLinkValues)
		if globalOrdinalJoin {
			typeValue := "to"
			if from {
				typeValue = "from"
			}
			doc.Add(newStringField(t, "type", typeValue, false))
		}
		var subValues []string
		{
			start := 0
			if len(randomUniqueValuesReplica) != numberOfLinkValues {
				start = rnd.Intn(len(randomUniqueValuesReplica) - numberOfLinkValues)
			}
			subValues = randomUniqueValuesReplica[start : start+numberOfLinkValues]
			shuffle(subValues, rnd)
		}
		for _, linkValue := range subValues {
			if util.AssertsEnabled() && contains(docs[i].linkValues, linkValue) {
				t.Fatal(util.NewAssertionError(linkValue))
			}
			docs[i].linkValues = append(docs[i].linkValues, linkValue)
			if from {
				context.fromDocuments[linkValue] = append(context.fromDocuments[linkValue], docs[i])
				context.randomValueFromDocs[value] = append(context.randomValueFromDocs[value], docs[i])
				addLinkFields(t, doc, "from", linkValue, multipleValuesPerDocument, globalOrdinalJoin)
			} else {
				context.toDocuments[linkValue] = append(context.toDocuments[linkValue], docs[i])
				context.randomValueToDocs[value] = append(context.randomValueToDocs[value], docs[i])
				addLinkFields(t, doc, "to", linkValue, multipleValuesPerDocument, globalOrdinalJoin)
			}
		}

		mustAddDocument(t, w, doc)
		if rnd.Intn(10) == 4 {
			mustCommit(t, w)
		}
		if verbose {
			fmt.Printf("Added document[%s]: %v\n", docs[i].id, doc)
		}
	}

	if rnd.Intn(2) == 0 {
		if verbose {
			fmt.Println("TEST: now force merge")
		}
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}
	mustClose(t, w)

	// Pre-compute all possible hits for all unique random values. On top of this also compute
	// all possible score for any ScoreMode.
	topLevelReader := mustOpenDirectoryReader(t, dir)
	searcher := newSearcher(t, topLevelReader)
	for i, uniqueRandomValue := range context.randomUniqueValues {
		var fromField, toField string
		var queryVals map[string]map[int]*joinScore
		if context.randomFrom[i] {
			fromField = "from"
			toField = "to"
			queryVals = context.fromHitsToJoinScore
		} else {
			fromField = "to"
			toField = "from"
			queryVals = context.toHitsToJoinScore
		}
		var joinValueToJoinScores map[string]*joinScore
		var err error
		valueQuery := search.NewTermQuery(index.NewTerm("value", uniqueRandomValue))
		if multipleValuesPerDocument {
			joinValueToJoinScores, err = search.SearchWithCollectorManager[*sortedSetJoinScoreCollector, map[string]*joinScore](
				searcher, valueQuery, sortedSetJoinScoreCollectorManager{fromField: fromField})
		} else {
			joinValueToJoinScores, err = search.SearchWithCollectorManager[*sortedDocValuesJoinScoreCollector, map[string]*joinScore](
				searcher, valueQuery, sortedDocValuesJoinScoreCollectorManager{fromField: fromField})
		}
		if err != nil {
			t.Fatalf("search: %v", err)
		}

		docToJoinScore := map[int]*joinScore{}
		if multipleValuesPerDocument {
			terms, err := index.MultiTermsGetTerms(topLevelReader, toField)
			if err != nil {
				t.Fatalf("MultiTerms.getTerms: %v", err)
			}
			if terms != nil {
				joinValues := make([]string, 0, len(joinValueToJoinScores))
				for k := range joinValueToJoinScores {
					joinValues = append(joinValues, k)
				}
				sort.Strings(joinValues)
				for _, joinValue := range joinValues {
					termsEnum, err := terms.Iterator()
					if err != nil {
						t.Fatalf("terms.iterator: %v", err)
					}
					found, err := termsEnum.SeekExact(index.NewTermFromBytes(toField, []byte(joinValue)))
					if err != nil {
						t.Fatalf("seekExact: %v", err)
					}
					if found {
						postingsEnum, err := termsEnum.Postings(spi.PostingsFlagNone)
						if err != nil {
							t.Fatalf("postings: %v", err)
						}
						js := joinValueToJoinScores[joinValue]
						for doc := mustNextDoc(t, postingsEnum); doc != search.NO_MORE_DOCS; doc = mustNextDoc(t, postingsEnum) {
							// First encountered join value determines the score.
							// Something to keep in mind for many-to-many relations.
							if _, ok := docToJoinScore[doc]; !ok {
								docToJoinScore[doc] = js
							}
						}
					}
				}
			}
		} else {
			merged, err := search.SearchWithCollectorManager[*docToJoinScoreCollector, map[int]*joinScore](
				searcher, search.Instance,
				docToJoinScoreCollectorManager{toField: toField, joinValueToJoinScores: joinValueToJoinScores})
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			for k, v := range merged {
				docToJoinScore[k] = v
			}
		}
		queryVals[uniqueRandomValue] = docToJoinScore
	}

	if globalOrdinalJoin {
		context.ordinalMap = mustOrdinalMap(t, topLevelReader, "join_field")
	}

	context.searcher = searcher
	context.dir = dir
	return context
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func addLinkFields(t testing.TB, doc *document.Document, fieldName, linkValue string,
	multipleValuesPerDocument, globalOrdinalJoin bool) {
	t.Helper()
	doc.Add(newTextField(t, fieldName, linkValue, false))

	parsed, err := strconv.ParseUint(linkValue, 16, 32)
	if err != nil {
		t.Fatalf("Integer.parseUnsignedInt(%q): %v", linkValue, err)
	}
	linkInt := int32(uint32(parsed))
	doc.Add(document.NewIntPoint(fieldName+"INT", linkInt))
	doc.Add(document.NewFloatPoint(fieldName+"FLOAT", float32(linkInt)))

	linkLong := int64(linkInt)<<32 | int64(linkInt)
	doc.Add(document.NewLongPoint(fieldName+"LONG", linkLong))
	doc.Add(document.NewDoublePoint(fieldName+"DOUBLE", float64(linkLong)))

	if multipleValuesPerDocument {
		doc.Add(mustSortedSetDVField(t, fieldName, linkValue))
		doc.Add(mustSortedNumericDVField(t, fieldName+"INT", int64(linkInt)))
		doc.Add(mustSortedNumericDVField(t, fieldName+"FLOAT", int64(int32(math.Float32bits(float32(linkInt))))))
		doc.Add(mustSortedNumericDVField(t, fieldName+"LONG", linkLong))
		doc.Add(mustSortedNumericDVField(t, fieldName+"DOUBLE", int64(math.Float64bits(float64(linkLong)))))
	} else {
		doc.Add(mustSortedDVField(t, fieldName, linkValue))
		doc.Add(mustNumericDVField(t, fieldName+"INT", int64(linkInt)))
		doc.Add(mustFloatDVField(t, fieldName+"FLOAT", float32(linkInt)))
		doc.Add(mustNumericDVField(t, fieldName+"LONG", linkLong))
		doc.Add(mustDoubleDVField(t, fieldName+"DOUBLE", float64(linkLong)))
	}
	if globalOrdinalJoin {
		doc.Add(mustSortedDVField(t, "join_field", linkValue))
	}
}

func createExpectedTopDocs(queryValue string, from bool, scoreMode ScoreMode,
	context *indexIterationContext) *search.TopDocs {
	var hitsToJoinScores map[int]*joinScore
	if from {
		hitsToJoinScores = context.fromHitsToJoinScore[queryValue]
	} else {
		hitsToJoinScores = context.toHitsToJoinScore[queryValue]
	}
	type hit struct {
		key   int
		value *joinScore
	}
	hits := make([]hit, 0, len(hitsToJoinScores))
	for k, v := range hitsToJoinScores {
		hits = append(hits, hit{k, v})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		score1 := hits[i].value.score(scoreMode)
		score2 := hits[j].value.score(scoreMode)
		if cmp := javaFloatCompare(score2, score1); cmp != 0 {
			return cmp < 0
		}
		return hits[i].key-hits[j].key < 0
	})
	scoreDocs := make([]*search.ScoreDoc, min(10, len(hits)))
	for i := range scoreDocs {
		scoreDocs[i] = spi.NewScoreDoc(hits[i].key, hits[i].value.score(scoreMode), -1)
	}
	return spi.NewTopDocs(spi.NewTotalHits(int64(len(hits)), spi.EQUAL_TO), scoreDocs)
}

// javaFloatCompare renders Float.compare(float, float).
func javaFloatCompare(f1, f2 float32) int {
	if f1 < f2 {
		return -1
	}
	if f1 > f2 {
		return 1
	}
	b1 := int32(math.Float32bits(f1))
	b2 := int32(math.Float32bits(f2))
	if f1 != f1 {
		b1 = 0x7fc00000
	}
	if f2 != f2 {
		b2 = 0x7fc00000
	}
	switch {
	case b1 == b2:
		return 0
	case b1 < b2:
		return -1
	default:
		return 1
	}
}

func createExpectedResult(t testing.TB, queryValue string, from bool, topLevelReader index.IndexReaderInterface,
	context *indexIterationContext) *util.FixedBitSet {
	t.Helper()
	var randomValueDocs, linkValueDocuments map[string][]*randomDoc
	if from {
		randomValueDocs = context.randomValueFromDocs
		linkValueDocuments = context.toDocuments
	} else {
		randomValueDocs = context.randomValueToDocs
		linkValueDocuments = context.fromDocuments
	}

	expectedResult := mustFixedBitSet(t, topLevelReader.MaxDoc())
	matchingDocs, ok := randomValueDocs[queryValue]
	if !ok {
		return mustFixedBitSet(t, topLevelReader.MaxDoc())
	}

	for _, matchingDoc := range matchingDocs {
		for _, linkValue := range matchingDoc.linkValues {
			otherMatchingDocs, ok := linkValueDocuments[linkValue]
			if !ok {
				continue
			}

			for _, otherSideDoc := range otherMatchingDocs {
				postingsEnum, err := index.MultiTermsGetTermPostingsEnumWithFlags(
					topLevelReader, "id", []byte(otherSideDoc.id), 0)
				if err != nil {
					t.Fatalf("MultiTerms.getTermPostingsEnum: %v", err)
				}
				if util.AssertsEnabled() && postingsEnum == nil {
					t.Fatal(util.NewAssertionError("postingsEnum != null"))
				}
				doc := mustNextDoc(t, postingsEnum)
				expectedResult.Set(doc)
			}
		}
	}
	return expectedResult
}

func mustFixedBitSet(t testing.TB, numBits int) *util.FixedBitSet {
	t.Helper()
	bs, err := util.NewFixedBitSet(numBits)
	if err != nil {
		t.Fatalf("new FixedBitSet: %v", err)
	}
	return bs
}

// indexIterationContext renders the private static class IndexIterationContext.
type indexIterationContext struct {
	randomUniqueValues  []string
	randomFrom          []bool
	fromDocuments       map[string][]*randomDoc
	toDocuments         map[string][]*randomDoc
	randomValueFromDocs map[string][]*randomDoc
	randomValueToDocs   map[string][]*randomDoc

	fromHitsToJoinScore map[string]map[int]*joinScore
	toHitsToJoinScore   map[string]map[int]*joinScore

	ordinalMap *index.OrdinalMap

	dir      store.Directory
	searcher *search.IndexSearcher
}

func newIndexIterationContext() *indexIterationContext {
	return &indexIterationContext{
		fromDocuments:       map[string][]*randomDoc{},
		toDocuments:         map[string][]*randomDoc{},
		randomValueFromDocs: map[string][]*randomDoc{},
		randomValueToDocs:   map[string][]*randomDoc{},
		fromHitsToJoinScore: map[string]map[int]*joinScore{},
		toHitsToJoinScore:   map[string]map[int]*joinScore{},
	}
}

func (c *indexIterationContext) close(t testing.TB) {
	t.Helper()
	mustClose(t, c.searcher.GetIndexReader(), c.dir)
}

// randomDoc renders the private static class RandomDoc.
type randomDoc struct {
	id         string
	linkValues []string
}

func newRandomDoc(id string, numberOfLinkValues int) *randomDoc {
	return &randomDoc{id: id, linkValues: make([]string, 0, numberOfLinkValues)}
}

// joinScore renders the private static class JoinScore.
type joinScore struct {
	minScore float32
	maxScore float32
	total    float32
	count    int
}

func newJoinScore() *joinScore {
	return &joinScore{minScore: float32(math.Inf(1)), maxScore: float32(math.Inf(-1))}
}

func (js *joinScore) addScore(score float32) {
	if score > js.maxScore {
		js.maxScore = score
	}
	if score < js.minScore {
		js.minScore = score
	}
	js.total += score
	js.count++
}

func (js *joinScore) score(mode ScoreMode) float32 {
	switch mode {
	case None:
		return 1
	case Total:
		return js.total
	case Avg:
		return js.total / float32(js.count)
	case Min:
		return js.minScore
	case Max:
		return js.maxScore
	}
	panic(fmt.Sprintf("Unsupported ScoreMode: %v", mode))
}

// bitSetCollector renders the private static class BitSetCollector.
type bitSetCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	bitSet  *util.FixedBitSet
	docBase int
}

func newBitSetCollector(bitSet *util.FixedBitSet) *bitSetCollector {
	c := &bitSetCollector{bitSet: bitSet}
	c.Outer = c
	return c
}

func (c *bitSetCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *bitSetCollector) Collect(doc int) error {
	c.bitSet.Set(c.docBase + doc)
	return nil
}

func (c *bitSetCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	c.docBase = context.DocBase
	return nil
}

func (c *bitSetCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (c *bitSetCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *bitSetCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// joinValueCollector renders the abstract static class JoinValueCollector.
type joinValueCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	localMap  map[string]*joinScore
	fromField string
	scorer    search.Scorable
}

func (c *joinValueCollector) SetScorer(s search.Scorable) error {
	c.scorer = s
	return nil
}

func (c *joinValueCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

// mergeJoinValueMaps renders the static JoinValueCollector.mergeJoinValueMaps.
func mergeJoinValueMaps(collectors []*joinValueCollector) map[string]*joinScore {
	merged := map[string]*joinScore{}
	for _, c := range collectors {
		for key, src := range c.localMap {
			existing, ok := merged[key]
			if !ok {
				merged[key] = src
				continue
			}
			if src.minScore < existing.minScore {
				existing.minScore = src.minScore
			}
			if src.maxScore > existing.maxScore {
				existing.maxScore = src.maxScore
			}
			existing.total += src.total
			existing.count += src.count
		}
	}
	return merged
}

// sortedSetJoinScoreCollector renders the private static class SortedSetJoinScoreCollector.
type sortedSetJoinScoreCollector struct {
	joinValueCollector
	docTermOrds index.SortedSetDocValues
}

func (c *sortedSetJoinScoreCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *sortedSetJoinScoreCollector) Collect(doc int) error {
	if doc > c.docTermOrds.DocID() {
		if _, err := c.docTermOrds.Advance(doc); err != nil {
			return err
		}
	}
	if doc == c.docTermOrds.DocID() {
		for j := 0; j < c.docTermOrds.DocValueCount(); j++ {
			ord, err := c.docTermOrds.NextOrd()
			if err != nil {
				return err
			}
			joinValue, err := c.docTermOrds.LookupOrd(ord)
			if err != nil {
				return err
			}
			js, ok := c.localMap[string(joinValue)]
			if !ok {
				js = newJoinScore()
				c.localMap[string(joinValue)] = js
			}
			score, err := c.scorer.Score()
			if err != nil {
				return err
			}
			js.addScore(score)
		}
	}
	return nil
}

func (c *sortedSetJoinScoreCollector) DoSetNextReader(ctx *index.LeafReaderContext) error {
	v, err := index.GetSortedSet(ctx.LeafReader(), c.fromField)
	if err != nil {
		return err
	}
	c.docTermOrds = v
	return nil
}

func (c *sortedSetJoinScoreCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *sortedSetJoinScoreCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// sortedSetJoinScoreCollectorManager is the anonymous CollectorManager of
// createContext (multi-valued branch).
type sortedSetJoinScoreCollectorManager struct{ fromField string }

func (m sortedSetJoinScoreCollectorManager) NewCollector() (*sortedSetJoinScoreCollector, error) {
	c := &sortedSetJoinScoreCollector{joinValueCollector: joinValueCollector{
		localMap: map[string]*joinScore{}, fromField: m.fromField}}
	c.Outer = c
	return c, nil
}

func (m sortedSetJoinScoreCollectorManager) Reduce(collectors []*sortedSetJoinScoreCollector) (map[string]*joinScore, error) {
	base := make([]*joinValueCollector, len(collectors))
	for i, c := range collectors {
		base[i] = &c.joinValueCollector
	}
	return mergeJoinValueMaps(base), nil
}

// sortedDocValuesJoinScoreCollector renders the private static class
// SortedDocValuesJoinScoreCollector.
type sortedDocValuesJoinScoreCollector struct {
	joinValueCollector
	terms index.SortedDocValues
}

func (c *sortedDocValuesJoinScoreCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *sortedDocValuesJoinScoreCollector) Collect(doc int) error {
	found, err := c.terms.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	ord, err := c.terms.OrdValue()
	if err != nil {
		return err
	}
	joinValue, err := c.terms.LookupOrd(ord)
	if err != nil {
		return err
	}
	js, ok := c.localMap[string(joinValue)]
	if !ok {
		js = newJoinScore()
		c.localMap[string(joinValue)] = js
	}
	score, err := c.scorer.Score()
	if err != nil {
		return err
	}
	if verbose {
		fmt.Printf("expected val=%s expected score=%v\n", joinValue, score)
	}
	js.addScore(score)
	return nil
}

func (c *sortedDocValuesJoinScoreCollector) DoSetNextReader(ctx *index.LeafReaderContext) error {
	v, err := index.GetSorted(ctx.LeafReader(), c.fromField)
	if err != nil {
		return err
	}
	c.terms = v
	return nil
}

func (c *sortedDocValuesJoinScoreCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *sortedDocValuesJoinScoreCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// sortedDocValuesJoinScoreCollectorManager is the anonymous CollectorManager
// of createContext (single-valued branch).
type sortedDocValuesJoinScoreCollectorManager struct{ fromField string }

func (m sortedDocValuesJoinScoreCollectorManager) NewCollector() (*sortedDocValuesJoinScoreCollector, error) {
	c := &sortedDocValuesJoinScoreCollector{joinValueCollector: joinValueCollector{
		localMap: map[string]*joinScore{}, fromField: m.fromField}}
	c.Outer = c
	return c, nil
}

func (m sortedDocValuesJoinScoreCollectorManager) Reduce(collectors []*sortedDocValuesJoinScoreCollector) (map[string]*joinScore, error) {
	base := make([]*joinValueCollector, len(collectors))
	for i, c := range collectors {
		base[i] = &c.joinValueCollector
	}
	return mergeJoinValueMaps(base), nil
}

// docToJoinScoreCollector renders the private static class DocToJoinScoreCollector.
type docToJoinScoreCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	localMap              map[int]*joinScore
	toField               string
	joinValueToJoinScores map[string]*joinScore
	terms                 index.SortedDocValues
	docBase               int
}

func (c *docToJoinScoreCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *docToJoinScoreCollector) Collect(doc int) error {
	var joinValue []byte
	found, err := c.terms.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if found {
		ord, err := c.terms.OrdValue()
		if err != nil {
			return err
		}
		joinValue, err = c.terms.LookupOrd(ord)
		if err != nil {
			return err
		}
	} else {
		joinValue = []byte{}
	}
	js, ok := c.joinValueToJoinScores[string(joinValue)]
	if !ok {
		return nil
	}
	c.localMap[c.docBase+doc] = js
	return nil
}

func (c *docToJoinScoreCollector) DoSetNextReader(ctx *index.LeafReaderContext) error {
	v, err := index.GetSorted(ctx.LeafReader(), c.toField)
	if err != nil {
		return err
	}
	c.terms = v
	c.docBase = ctx.DocBase
	return nil
}

func (c *docToJoinScoreCollector) SetScorer(search.Scorable) error { return nil }

func (c *docToJoinScoreCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (c *docToJoinScoreCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *docToJoinScoreCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// docToJoinScoreCollectorManager is the anonymous CollectorManager of
// createContext (single-valued doc-to-score branch).
type docToJoinScoreCollectorManager struct {
	toField               string
	joinValueToJoinScores map[string]*joinScore
}

func (m docToJoinScoreCollectorManager) NewCollector() (*docToJoinScoreCollector, error) {
	c := &docToJoinScoreCollector{localMap: map[int]*joinScore{}, toField: m.toField,
		joinValueToJoinScores: m.joinValueToJoinScores}
	c.Outer = c
	return c, nil
}

func (m docToJoinScoreCollectorManager) Reduce(collectors []*docToJoinScoreCollector) (map[int]*joinScore, error) {
	merged := map[int]*joinScore{}
	for _, c := range collectors {
		for k, v := range c.localMap {
			merged[k] = v
		}
	}
	return merged, nil
}

// bitSetCollectorManager renders the private static class BitSetCollectorManager.
type bitSetCollectorManager struct {
	maxDoc int
}

func (m *bitSetCollectorManager) NewCollector() (*bitSetCollector, error) {
	bs, err := util.NewFixedBitSet(m.maxDoc)
	if err != nil {
		return nil, err
	}
	return newBitSetCollector(bs), nil
}

func (m *bitSetCollectorManager) Reduce(collectors []*bitSetCollector) (*util.FixedBitSet, error) {
	result, err := util.NewFixedBitSet(m.maxDoc)
	if err != nil {
		return nil, err
	}
	for _, c := range collectors {
		if err := result.Or(c.bitSet); err != nil {
			return nil, err
		}
	}
	return result, nil
}
