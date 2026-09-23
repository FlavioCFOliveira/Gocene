// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// JoinUtil is the utility for query time joining.
//
// Port of org.apache.lucene.search.join.JoinUtil
// (lucene/join/src/java/org/apache/lucene/search/join/JoinUtil.java, Apache
// Lucene 10.5.0). The Java class is final with a private constructor ("No
// instances allowed") and only static members, rendered as the package-level
// CreateJoinQuery* functions below.
//
// Java overloads createJoinQuery four times; the Go names follow the overload
// rules: CreateJoinQuery (terms join), CreateJoinQueryWithNumericType
// (numeric join: adds numericType), CreateJoinQueryGlobalOrdinals (global
// ordinals join over an OrdinalMap) and CreateJoinQueryGlobalOrdinalsWithMinMax
// (adds min and max).
//
// @lucene.experimental

// Java numeric types accepted by CreateJoinQueryWithNumericType, rendering
// Integer.class, Long.class, Float.class and Double.class.
var (
	JoinNumericTypeInteger = reflect.TypeOf(int32(0))
	JoinNumericTypeLong    = reflect.TypeOf(int64(0))
	JoinNumericTypeFloat   = reflect.TypeOf(float32(0))
	JoinNumericTypeDouble  = reflect.TypeOf(float64(0))
)

// CreateJoinQuery is the method for query time joining.
//
// Execute the returned query with an IndexSearcher to retrieve all documents
// that have the same terms in the to field that match with documents matching
// the specified fromQuery and have the same terms in the from field.
//
// In the case a single document relates to more than one document the
// multipleValuesPerDocument option should be set to true. When the
// multipleValuesPerDocument is set to true only the score from the first
// encountered join value originating from the 'from' side is mapped into the
// 'to' side. Even in the case when a second join value related to a specific
// document yields a higher score. Obviously this doesn't apply in the case
// that ScoreMode None is used, since no scores are computed at all.
//
// Memory considerations: During joining all unique join values are kept in
// memory. On top of that when the scoreMode isn't set to None a float value
// per unique join value is kept in memory for computing scores. When
// scoreMode is set to Avg also an additional integer value is kept in memory
// per unique join value.
//
// Renders createJoinQuery(String fromField, boolean multipleValuesPerDocument,
// String toField, Query fromQuery, IndexSearcher fromSearcher, ScoreMode
// scoreMode).
func CreateJoinQuery(fromField string, multipleValuesPerDocument bool, toField string, fromQuery search.Query,
	fromSearcher *search.IndexSearcher, scoreMode ScoreMode) (search.Query, error) {

	var termsWithScoreCollector GenericTermsCollector

	if multipleValuesPerDocument {
		mvFunction := SortedSetDocValues(fromField)
		termsWithScoreCollector = CreateCollectorMV(mvFunction, scoreMode)
	} else {
		svFunction := SortedDocValues(fromField)
		termsWithScoreCollector = CreateCollectorSV(svFunction, scoreMode)
	}

	return createJoinQueryWithCollector(
		multipleValuesPerDocument,
		toField,
		fromQuery,
		fromField,
		fromSearcher,
		scoreMode,
		termsWithScoreCollector)
}

// longFloatFunction renders the private interface LongFloatFunction.
type longFloatFunction func(value int64) float32

// longFloatProcedure renders the private interface LongFloatProcedure.
type longFloatProcedure func(key int64, value float32)

// CreateJoinQueryWithNumericType is the method for query time joining for
// numeric fields. It supports multi- and single- values longs, ints, floats
// and longs. All considerations from CreateJoinQuery are applicable here too,
// though memory consumption might be higher.
//
// multipleValuesPerDocument: when true fromField might be SORTED_NUMERIC,
// otherwise fromField should be NUMERIC. toField should be an IntPoint,
// LongPoint, FloatPoint or DoublePoint field. numericType is one of
// JoinNumericTypeInteger, JoinNumericTypeLong, JoinNumericTypeFloat or
// JoinNumericTypeDouble (Java's Integer, Long, Float or Double class) and it
// should correspond to the toField type.
//
// Renders createJoinQuery(String fromField, boolean multipleValuesPerDocument,
// String toField, Class<? extends Number> numericType, Query fromQuery,
// IndexSearcher fromSearcher, ScoreMode scoreMode).
//
// PORT NOTE. Java keeps the join values in the lucene.internal hppc
// LongHashSet, LongFloatHashMap and LongIntHashMap; Gocene has no port of
// those containers, so Go maps hold the same set and maps. The values are
// sorted before use exactly as Java sorts joinValuesList, so nothing
// observable depends on the container.
func CreateJoinQueryWithNumericType(fromField string, multipleValuesPerDocument bool, toField string,
	numericType reflect.Type, fromQuery search.Query, fromSearcher *search.IndexSearcher,
	scoreMode ScoreMode) (search.Query, error) {

	joinValues := map[int64]struct{}{}
	aggregatedScores := map[int64]float32{}
	occurrences := map[int64]int32{}
	needsScore := scoreMode != None
	var scoreAggregator longFloatProcedure
	switch scoreMode {
	case Max:
		scoreAggregator = func(key int64, score float32) {
			if currentScore, ok := aggregatedScores[key]; !ok {
				aggregatedScores[key] = score
			} else {
				aggregatedScores[key] = float32(math.Max(float64(currentScore), float64(score)))
			}
		}
	case Min:
		scoreAggregator = func(key int64, score float32) {
			if currentScore, ok := aggregatedScores[key]; !ok {
				aggregatedScores[key] = score
			} else {
				aggregatedScores[key] = float32(math.Min(float64(currentScore), float64(score)))
			}
		}
	case Total:
		scoreAggregator = func(key int64, score float32) {
			aggregatedScores[key] += score
		}
	case Avg:
		scoreAggregator = func(key int64, score float32) {
			aggregatedScores[key] += score
			occurrences[key]++
		}
	default:
		scoreAggregator = func(key int64, score float32) {
			panic("UnsupportedOperationException")
		}
	}

	var joinScorer longFloatFunction
	if scoreMode == Avg {
		joinScorer = func(joinValue int64) float32 {
			aggregatedScore := aggregatedScores[joinValue]
			occurrence := occurrences[joinValue]
			return aggregatedScore / float32(occurrence)
		}
	} else {
		joinScorer = func(joinValue int64) float32 {
			return aggregatedScores[joinValue]
		}
	}

	var collector search.Collector
	if multipleValuesPerDocument {
		c := &numericJoinMVCollector{
			fromField: fromField, needsScore: needsScore,
			joinValues: joinValues, scoreAggregator: scoreAggregator,
		}
		c.Outer = c
		collector = c
	} else {
		c := &numericJoinSVCollector{
			fromField: fromField, needsScore: needsScore,
			joinValues: joinValues, scoreAggregator: scoreAggregator, lastDocID: -1,
		}
		c.Outer = c
		collector = c
	}
	if err := fromSearcher.SearchWithCollector(fromQuery, collector); err != nil {
		return nil, err
	}

	joinValuesList := make([]int64, 0, len(joinValues))
	for v := range joinValues {
		joinValuesList = append(joinValuesList, v)
	}
	sort.Slice(joinValuesList, func(i, j int) bool { return joinValuesList[i] < joinValuesList[j] })

	var bytesPerDim int
	var encode func(value int64, dest []byte)
	switch numericType {
	case JoinNumericTypeInteger:
		bytesPerDim = 4 // Integer.BYTES
		encode = func(value int64, dest []byte) {
			document.EncodeDimensionIntLucene(int32(value), dest, 0)
		}
	case JoinNumericTypeLong:
		bytesPerDim = 8 // Long.BYTES
		encode = func(value int64, dest []byte) {
			document.EncodeDimensionLongLucene(value, dest, 0)
		}
	case JoinNumericTypeFloat:
		bytesPerDim = 4 // Float.BYTES
		encode = func(value int64, dest []byte) {
			document.EncodeDimensionFloatLucene(math.Float32frombits(uint32(int32(value))), dest, 0)
		}
	case JoinNumericTypeDouble:
		bytesPerDim = 8 // Double.BYTES
		encode = func(value int64, dest []byte) {
			document.EncodeDimensionDoubleLucene(math.Float64frombits(uint64(value)), dest, 0)
		}
	default:
		return nil, fmt.Errorf("unsupported numeric type, only Integer, Long, Float and Double are supported")
	}

	encoded := util.NewBytesRef(make([]byte, bytesPerDim))
	stream := &numericJoinStream{
		values: joinValuesList, encoded: encoded, encode: encode,
		needsScore: needsScore, joinScorer: joinScorer,
	}

	toString := func(value []byte) string {
		return pointInSetIncludingScoreQueryToString(value, numericType)
	}
	if needsScore {
		q, err := NewPointInSetIncludingScoreQuery(
			scoreMode, fromQuery, multipleValuesPerDocument, toField, bytesPerDim, numericJoinScoreStream{stream}, toString)
		if err != nil {
			return nil, err
		}
		return q, nil
	}
	q, err := search.NewPointInSetQuery(toField, 1, bytesPerDim, numericJoinPointStream{stream}, toString)
	if err != nil {
		return nil, err
	}
	return q, nil
}

// numericJoinStream renders the anonymous PointInSetIncludingScoreQuery.Stream
// subclasses of CreateJoinQueryWithNumericType, which walk the sorted join
// values, encode each one into the shared encoded BytesRef and set the score.
// It also serves the PointInSetQuery (no-score) branch, whose Java stream is
// the same object.
type numericJoinStream struct {
	values     []int64
	upto       int
	encoded    *util.BytesRef
	encode     func(value int64, dest []byte)
	needsScore bool
	joinScorer longFloatFunction
	score      float32
}

// next renders BytesRef next(): it encodes the next sorted join value into the
// shared encoded BytesRef and sets the score, or returns nil when exhausted.
func (s *numericJoinStream) next() *util.BytesRef {
	if s.upto < len(s.values) {
		value := s.values[s.upto]
		s.upto++
		s.encode(value, s.encoded.Bytes)
		if s.needsScore {
			s.score = s.joinScorer(value)
		}
		return s.encoded
	}
	return nil
}

// numericJoinPointStream presents numericJoinStream as a
// PointInSetQuery.Stream (Gocene's search.PointInSetQueryStream).
type numericJoinPointStream struct{ *numericJoinStream }

// Next renders BytesRef next().
func (s numericJoinPointStream) Next() (*util.BytesRef, error) {
	return s.next(), nil
}

// numericJoinScoreStream presents numericJoinStream as a
// PointInSetIncludingScoreQuery.Stream (Gocene's
// PointInSetIncludingScoreQueryStream, whose Next returns the packed bytes).
type numericJoinScoreStream struct{ *numericJoinStream }

// Next renders BytesRef next().
func (s numericJoinScoreStream) Next() []byte {
	b := s.next()
	if b == nil {
		return nil
	}
	return b.ValidBytes()
}

// Score renders the protected float score field of
// PointInSetIncludingScoreQuery.Stream.
func (s numericJoinScoreStream) Score() float32 { return s.score }

// numericJoinMVCollector renders the anonymous SimpleCollector of the
// multiple-values branch of CreateJoinQueryWithNumericType.
type numericJoinMVCollector struct {
	search.BaseSimpleCollector
	fromField              string
	needsScore             bool
	joinValues             map[int64]struct{}
	scoreAggregator        longFloatProcedure
	sortedNumericDocValues index.SortedNumericDocValues
	scorer                 search.Scorable
}

func (c *numericJoinMVCollector) Collect(doc int) error {
	ok, err := c.sortedNumericDocValues.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if ok {
		count, err := c.sortedNumericDocValues.DocValueCount()
		if err != nil {
			return err
		}
		for i := 0; i < count; i++ {
			value, err := c.sortedNumericDocValues.NextValue()
			if err != nil {
				return err
			}
			c.joinValues[value] = struct{}{}
			if c.needsScore {
				score, err := c.scorer.Score()
				if err != nil {
					return err
				}
				c.scoreAggregator(value, score)
			}
		}
	}
	return nil
}

func (c *numericJoinMVCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *numericJoinMVCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	dv, err := index.GetSortedNumeric(context.LeafReader(), c.fromField)
	if err != nil {
		return err
	}
	c.sortedNumericDocValues = dv
	return nil
}

func (c *numericJoinMVCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *numericJoinMVCollector) ScoreMode() search.ScoreMode {
	if c.needsScore {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

func (c *numericJoinMVCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *numericJoinMVCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func (c *numericJoinMVCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

func (c *numericJoinMVCollector) Finish() error { return nil }

// numericJoinSVCollector renders the anonymous SimpleCollector of the
// single-value branch of CreateJoinQueryWithNumericType.
type numericJoinSVCollector struct {
	search.BaseSimpleCollector
	fromField        string
	needsScore       bool
	joinValues       map[int64]struct{}
	scoreAggregator  longFloatProcedure
	numericDocValues index.NumericDocValues
	scorer           search.Scorable
	lastDocID        int
}

func (c *numericJoinSVCollector) docsInOrder(docID int) bool {
	if docID < c.lastDocID {
		panic(util.NewAssertionError(fmt.Sprintf("docs out of order: lastDocID=%d vs docID=%d", c.lastDocID, docID)))
	}
	c.lastDocID = docID
	return true
}

func (c *numericJoinSVCollector) Collect(doc int) error {
	if util.AssertsEnabled() {
		c.docsInOrder(doc)
	}
	var value int64
	ok, err := c.numericDocValues.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if ok {
		value, err = c.numericDocValues.LongValue()
		if err != nil {
			return err
		}
	}
	c.joinValues[value] = struct{}{}
	if c.needsScore {
		score, err := c.scorer.Score()
		if err != nil {
			return err
		}
		c.scoreAggregator(value, score)
	}
	return nil
}

func (c *numericJoinSVCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *numericJoinSVCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	dv, err := index.GetNumeric(context.LeafReader(), c.fromField)
	if err != nil {
		return err
	}
	c.numericDocValues = dv
	c.lastDocID = -1
	return nil
}

func (c *numericJoinSVCollector) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

func (c *numericJoinSVCollector) ScoreMode() search.ScoreMode {
	if c.needsScore {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

func (c *numericJoinSVCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *numericJoinSVCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func (c *numericJoinSVCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

func (c *numericJoinSVCollector) Finish() error { return nil }

// createJoinQueryWithCollector renders the private createJoinQuery(boolean
// multipleValuesPerDocument, String toField, Query fromQuery, String
// fromField, IndexSearcher fromSearcher, ScoreMode scoreMode, final
// GenericTermsCollector collector).
func createJoinQueryWithCollector(multipleValuesPerDocument bool, toField string, fromQuery search.Query,
	fromField string, fromSearcher *search.IndexSearcher, scoreMode ScoreMode,
	collector GenericTermsCollector) (search.Query, error) {

	if err := fromSearcher.SearchWithCollector(fromQuery, collector); err != nil {
		return nil, err
	}
	switch scoreMode {
	case None:
		return newTermsQuery(
			toField,
			collector.GetCollectedTerms(),
			fromField,
			fromQuery,
			fromSearcher.GetTopReaderContext().ID()), nil
	case Total, Max, Min, Avg:
		return NewTermsIncludingScoreQuery(
			scoreMode,
			toField,
			multipleValuesPerDocument,
			collector.GetCollectedTerms(),
			collector.GetScoresPerTerm(),
			fromField,
			fromQuery,
			fromSearcher.GetTopReaderContext().ID()), nil
	default:
		return nil, fmt.Errorf("Score mode %s isn't supported.", scoreMode)
	}
}

// CreateJoinQueryGlobalOrdinals delegates to
// CreateJoinQueryGlobalOrdinalsWithMinMax, but disables the min and max
// filtering.
//
// joinField is the SortedDocValues field containing the join values;
// fromQuery is the query containing the actual user query (the fromQuery can
// only match "from" documents); toQuery is the query identifying all
// documents on the "to" side; searcher is the index searcher used to execute
// the from query; scoreMode instructs how scores from the fromQuery are
// mapped to the returned query; ordinalMap is the ordinal map constructed
// over the joinField (in case of a single segment index, no ordinal map needs
// to be provided).
//
// Renders createJoinQuery(String joinField, Query fromQuery, Query toQuery,
// IndexSearcher searcher, ScoreMode scoreMode, OrdinalMap ordinalMap).
func CreateJoinQueryGlobalOrdinals(joinField string, fromQuery, toQuery search.Query, searcher *search.IndexSearcher,
	scoreMode ScoreMode, ordinalMap *index.OrdinalMap) (search.Query, error) {
	return CreateJoinQueryGlobalOrdinalsWithMinMax(
		joinField, fromQuery, toQuery, searcher, scoreMode, ordinalMap, 0, math.MaxInt32)
}

// CreateJoinQueryGlobalOrdinalsWithMinMax is a query time join using global
// ordinals over a dedicated join field.
//
// This join has certain restrictions and requirements: 1) A document can only
// refer to one other document. (but can be referred by one or more documents)
// 2) Documents on each side of the join must be distinguishable. Typically
// this can be done by adding an extra field that identifies the "from" and
// "to" side and then the fromQuery and toQuery must take this into account.
// 3) There must be a single sorted doc values join field used by both the
// "from" and "to" documents. This join field should store the join values as
// UTF-8 strings. 4) An ordinal map must be provided that is created on top of
// the join field.
//
// Note: min and max filtering and the avg score mode will require this join
// to keep track of the number of times a document matches per join value.
// This will increase the per join cost in terms of execution time and memory.
//
// min is optionally the minimum number of "from" documents that are required
// to match for a "to" document to be a match (inclusive); max is optionally
// the maximum number of "from" documents that are allowed to match for a "to"
// document to be a match (inclusive). Setting min to 0 and max to
// math.MaxInt32 disables the min and max "from" documents filtering.
//
// Renders createJoinQuery(String joinField, Query fromQuery, Query toQuery,
// IndexSearcher searcher, ScoreMode scoreMode, OrdinalMap ordinalMap, int
// min, int max).
func CreateJoinQueryGlobalOrdinalsWithMinMax(joinField string, fromQuery, toQuery search.Query,
	searcher *search.IndexSearcher, scoreMode ScoreMode, ordinalMap *index.OrdinalMap,
	min, max int) (search.Query, error) {

	leaves, err := searcher.GetIndexReader().Leaves()
	if err != nil {
		return nil, err
	}
	numSegments := len(leaves)
	var valueCount int64
	if numSegments == 0 {
		return search.NewMatchNoDocsQuery("JoinUtil.createJoinQuery with no segments"), nil
	} else if numSegments == 1 {
		// No need to use the ordinal map, because there is just one segment.
		ordinalMap = nil
		leafReader := leaves[0].LeafReader()
		joinSortedDocValues, err := leafReader.GetSortedDocValues(joinField)
		if err != nil {
			return nil, err
		}
		if joinSortedDocValues != nil {
			valueCount = int64(joinSortedDocValues.GetValueCount())
		} else {
			return search.NewMatchNoDocsQuery("JoinUtil.createJoinQuery: no join values"), nil
		}
	} else {
		if ordinalMap == nil {
			return nil, fmt.Errorf("OrdinalMap is required, because there is more than 1 segment")
		}
		valueCount = ordinalMap.GetValueCount()
	}

	rewrittenFromQuery, err := searcher.Rewrite(fromQuery)
	if err != nil {
		return nil, err
	}
	rewrittenToQuery, err := searcher.Rewrite(toQuery)
	if err != nil {
		return nil, err
	}
	var globalOrdinalsWithScoreCollector *GlobalOrdinalsWithScoreCollector
	var collector search.Collector
	switch scoreMode {
	case Total:
		c, err := NewGlobalOrdinalsWithScoreCollectorSum(joinField, ordinalMap, valueCount, min, max)
		if err != nil {
			return nil, err
		}
		globalOrdinalsWithScoreCollector, collector = &c.GlobalOrdinalsWithScoreCollector, c
	case Min:
		c, err := NewGlobalOrdinalsWithScoreCollectorMin(joinField, ordinalMap, valueCount, min, max)
		if err != nil {
			return nil, err
		}
		globalOrdinalsWithScoreCollector, collector = &c.GlobalOrdinalsWithScoreCollector, c
	case Max:
		c, err := NewGlobalOrdinalsWithScoreCollectorMax(joinField, ordinalMap, valueCount, min, max)
		if err != nil {
			return nil, err
		}
		globalOrdinalsWithScoreCollector, collector = &c.GlobalOrdinalsWithScoreCollector, c
	case Avg:
		c, err := NewGlobalOrdinalsWithScoreCollectorAvg(joinField, ordinalMap, valueCount, min, max)
		if err != nil {
			return nil, err
		}
		globalOrdinalsWithScoreCollector, collector = &c.GlobalOrdinalsWithScoreCollector, c
	case None:
		if min <= 1 && max == math.MaxInt32 {
			globalOrdinalsCollector, err := NewGlobalOrdinalsCollector(joinField, ordinalMap, valueCount)
			if err != nil {
				return nil, err
			}
			if err := searcher.SearchWithCollector(rewrittenFromQuery, globalOrdinalsCollector); err != nil {
				return nil, err
			}
			return NewGlobalOrdinalsQuery(
				globalOrdinalsCollector.GetCollectedOrds(),
				joinField,
				ordinalMap,
				rewrittenToQuery,
				rewrittenFromQuery,
				searcher.GetTopReaderContext().ID()), nil
		}
		c, err := NewGlobalOrdinalsWithScoreCollectorNoScore(joinField, ordinalMap, valueCount, min, max)
		if err != nil {
			return nil, err
		}
		globalOrdinalsWithScoreCollector, collector = &c.GlobalOrdinalsWithScoreCollector, c
	default:
		return nil, fmt.Errorf("Score mode %s isn't supported.", scoreMode)
	}
	if err := searcher.SearchWithCollector(rewrittenFromQuery, collector); err != nil {
		return nil, err
	}
	return NewGlobalOrdinalsWithScoreQuery(
		globalOrdinalsWithScoreCollector,
		scoreMode,
		joinField,
		ordinalMap,
		rewrittenToQuery,
		rewrittenFromQuery,
		min,
		max,
		searcher.GetTopReaderContext().ID()), nil
}
