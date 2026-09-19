// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/join/src/java/org/apache/lucene/search/join/TermsWithScoreCollector.java

package join

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// initialArraySize mirrors
// `private static final int INITIAL_ARRAY_SIZE = 0` of TermsWithScoreCollector.
const initialArraySize = 0

// TermsWithScoreCollector renders the abstract class
// org.apache.lucene.search.join.TermsWithScoreCollector<DV> as an interface,
// because Java's `static TermsWithScoreCollector<?> create(...)` returns the
// wildcard type and a Go generic type cannot be named without binding its type
// argument. The concrete members of the Java class live in
// BaseTermsWithScoreCollector below.
type TermsWithScoreCollector interface {
	// SimpleCollector renders `extends DocValuesTermsCollector<DV>`, which in
	// turn extends org.apache.lucene.search.SimpleCollector.
	search.SimpleCollector

	// GenericTermsCollector renders `implements GenericTermsCollector`.
	GenericTermsCollector
}

// BaseTermsWithScoreCollector carries the concrete members of the abstract
// class org.apache.lucene.search.join.TermsWithScoreCollector<DV>.
type BaseTermsWithScoreCollector[DV any] struct {
	DocValuesTermsCollector[DV]

	// collectedTerms renders `final BytesRefHash collectedTerms = new BytesRefHash()`.
	collectedTerms *util.BytesRefHash

	// scoreMode renders `final ScoreMode scoreMode`.
	scoreMode ScoreMode

	// scorer renders `Scorable scorer`.
	scorer search.Scorable

	// scoreSums renders `float[] scoreSums = new float[INITIAL_ARRAY_SIZE]`.
	scoreSums []float32
}

// NewBaseTermsWithScoreCollector mirrors
// `TermsWithScoreCollector(Function<DV> docValuesCall, ScoreMode scoreMode)`.
func NewBaseTermsWithScoreCollector[DV any](
	docValuesCall DocValuesTermsCollectorFunction[DV],
	scoreMode ScoreMode,
) *BaseTermsWithScoreCollector[DV] {
	c := &BaseTermsWithScoreCollector[DV]{
		DocValuesTermsCollector: *NewDocValuesTermsCollector(docValuesCall),
		collectedTerms:          util.NewBytesRefHash(),
		scoreMode:               scoreMode,
		scoreSums:               make([]float32, initialArraySize),
	}
	// Arrays.fill over an INITIAL_ARRAY_SIZE-long array; INITIAL_ARRAY_SIZE is
	// zero, so both fills are no-ops in Java too. Ported as written.
	if scoreMode == Min {
		fillFloat32(c.scoreSums, 0, len(c.scoreSums), float32(math.Inf(1)))
	} else if scoreMode == Max {
		fillFloat32(c.scoreSums, 0, len(c.scoreSums), float32(math.Inf(-1)))
	}
	return c
}

// fillFloat32 renders java.util.Arrays.fill(float[], int, int, float).
func fillFloat32(array []float32, fromIndex, toIndex int, value float32) {
	for i := fromIndex; i < toIndex; i++ {
		array[i] = value
	}
}

// isUnsetScore renders `Float.compare(existing, 0.0f) == 0`, which is true only
// for positive zero: Float.compare falls through to floatToIntBits, and
// floatToIntBits(0.0f) is 0, so negative zero and NaN both compare non-equal.
func isUnsetScore(existing float32) bool {
	return math.Float32bits(existing) == math.Float32bits(0)
}

// GetCollectedTerms mirrors `public BytesRefHash getCollectedTerms()`.
func (c *BaseTermsWithScoreCollector[DV]) GetCollectedTerms() *util.BytesRefHash {
	return c.collectedTerms
}

// GetScoresPerTerm mirrors `public float[] getScoresPerTerm()`.
func (c *BaseTermsWithScoreCollector[DV]) GetScoresPerTerm() []float32 {
	return c.scoreSums
}

// SetScorer mirrors `public void setScorer(Scorable scorer)`.
func (c *BaseTermsWithScoreCollector[DV]) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	return nil
}

// ScoreMode mirrors `public org.apache.lucene.search.ScoreMode scoreMode()`,
// which returns ScoreMode.COMPLETE.
func (c *BaseTermsWithScoreCollector[DV]) ScoreMode() search.ScoreMode {
	return search.COMPLETE
}

// CreateTermsWithScoreCollector chooses the right TermsWithScoreCollector
// implementation.
//
// Mirrors
// `static TermsWithScoreCollector<?> create(String field, boolean multipleValuesPerDocument, ScoreMode scoreMode)`.
//
//   - field is the field to collect terms for;
//   - multipleValuesPerDocument states whether the field to collect terms for
//     has multiple values per document.
func CreateTermsWithScoreCollector(
	field string,
	multipleValuesPerDocument bool,
	scoreMode ScoreMode,
) TermsWithScoreCollector {
	if multipleValuesPerDocument {
		switch scoreMode {
		case Avg:
			return NewTermsWithScoreCollectorMVAvg(SortedSetDocValues(field))
		case Max, Min, None, Total:
			fallthrough
		default:
			return NewTermsWithScoreCollectorMV(SortedSetDocValues(field), scoreMode)
		}
	}
	switch scoreMode {
	case Avg:
		return NewTermsWithScoreCollectorSVAvg(SortedDocValues(field))
	case Max, Min, None, Total:
		fallthrough
	default:
		return NewTermsWithScoreCollectorSV(SortedDocValues(field), scoreMode)
	}
}

// TermsWithScoreCollectorSV is the impl that works with a single value per
// document.
//
// Mirrors `static class SV extends TermsWithScoreCollector<SortedDocValues>`.
type TermsWithScoreCollectorSV struct {
	BaseTermsWithScoreCollector[index.SortedDocValues]
}

// NewTermsWithScoreCollectorSV mirrors
// `SV(Function<SortedDocValues> docValuesCall, ScoreMode scoreMode)`.
func NewTermsWithScoreCollectorSV(
	docValuesCall DocValuesTermsCollectorFunction[index.SortedDocValues],
	scoreMode ScoreMode,
) *TermsWithScoreCollectorSV {
	c := &TermsWithScoreCollectorSV{
		BaseTermsWithScoreCollector: *NewBaseTermsWithScoreCollector(docValuesCall, scoreMode),
	}
	c.Outer = c
	return c
}

// Collect mirrors `public void collect(int doc)` of TermsWithScoreCollector.SV.
func (c *TermsWithScoreCollectorSV) Collect(doc int) error {
	var value *util.BytesRef
	exists, err := c.docValues.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if exists {
		ordValue, err := c.docValues.OrdValue()
		if err != nil {
			return err
		}
		bytes, err := c.docValues.LookupOrd(ordValue)
		if err != nil {
			return err
		}
		value = util.WrapBytes(bytes)
	} else {
		// `new BytesRef(BytesRef.EMPTY_BYTES)`.
		value = util.NewBytesRefEmpty()
	}
	ord, err := c.collectedTerms.Add(value)
	if err != nil {
		return err
	}
	if ord < 0 {
		ord = -ord - 1
	} else {
		if ord >= len(c.scoreSums) {
			begin := len(c.scoreSums)
			// ArrayUtil.grow(float[] array) is grow(array, 1 + array.length).
			c.scoreSums = util.GrowFloat32(c.scoreSums, 1+len(c.scoreSums))
			if c.scoreMode == Min {
				fillFloat32(c.scoreSums, begin, len(c.scoreSums), float32(math.Inf(1)))
			} else if c.scoreMode == Max {
				fillFloat32(c.scoreSums, begin, len(c.scoreSums), float32(math.Inf(-1)))
			}
		}
	}

	current, err := c.scorer.Score()
	if err != nil {
		return err
	}
	existing := c.scoreSums[ord]
	if isUnsetScore(existing) {
		c.scoreSums[ord] = current
	} else {
		switch c.scoreMode {
		case Total:
			c.scoreSums[ord] = c.scoreSums[ord] + current
		case Min:
			if current < existing {
				c.scoreSums[ord] = current
			}
		case Max:
			if current > existing {
				c.scoreSums[ord] = current
			}
		case None, Avg:
			fallthrough
		default:
			// Java throws AssertionError, which is unchecked; collect(int) can
			// report only through its error return in Go.
			return fmt.Errorf("AssertionError: unexpected: %v", c.scoreMode)
		}
	}
	return nil
}

// CollectRange carries the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorSV) CollectRange(minDoc, maxDoc int) error {
	return search.DefaultCollectRange(c, minDoc, maxDoc)
}

// CollectStream carries the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorSV) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// TermsWithScoreCollectorSVAvg mirrors `static class Avg extends SV`.
type TermsWithScoreCollectorSVAvg struct {
	TermsWithScoreCollectorSV

	// scoreCounts renders `int[] scoreCounts = new int[INITIAL_ARRAY_SIZE]`.
	// getScoresPerTerm assigns null to it once the averages are computed; a Go
	// nil slice carries that state, and make() never yields nil, so the
	// zero-length initial value stays distinguishable from it.
	scoreCounts []int32
}

// NewTermsWithScoreCollectorSVAvg mirrors `Avg(Function<SortedDocValues> docValuesCall)`.
func NewTermsWithScoreCollectorSVAvg(
	docValuesCall DocValuesTermsCollectorFunction[index.SortedDocValues],
) *TermsWithScoreCollectorSVAvg {
	c := &TermsWithScoreCollectorSVAvg{
		TermsWithScoreCollectorSV: *NewTermsWithScoreCollectorSV(docValuesCall, Avg),
		scoreCounts:               make([]int32, initialArraySize),
	}
	c.Outer = c
	return c
}

// Collect mirrors `public void collect(int doc)` of TermsWithScoreCollector.SV.Avg.
func (c *TermsWithScoreCollectorSVAvg) Collect(doc int) error {
	var value *util.BytesRef
	exists, err := c.docValues.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if exists {
		ordValue, err := c.docValues.OrdValue()
		if err != nil {
			return err
		}
		bytes, err := c.docValues.LookupOrd(ordValue)
		if err != nil {
			return err
		}
		value = util.WrapBytes(bytes)
	} else {
		// `new BytesRef(BytesRef.EMPTY_BYTES)`.
		value = util.NewBytesRefEmpty()
	}
	ord, err := c.collectedTerms.Add(value)
	if err != nil {
		return err
	}
	if ord < 0 {
		ord = -ord - 1
	} else {
		if ord >= len(c.scoreSums) {
			c.scoreSums = util.GrowFloat32(c.scoreSums, 1+len(c.scoreSums))
			c.scoreCounts = util.GrowInt32(c.scoreCounts, 1+len(c.scoreCounts))
		}
	}

	current, err := c.scorer.Score()
	if err != nil {
		return err
	}
	existing := c.scoreSums[ord]
	if isUnsetScore(existing) {
		c.scoreSums[ord] = current
		c.scoreCounts[ord] = 1
	} else {
		c.scoreSums[ord] = c.scoreSums[ord] + current
		c.scoreCounts[ord]++
	}
	return nil
}

// GetScoresPerTerm mirrors `public float[] getScoresPerTerm()` of
// TermsWithScoreCollector.SV.Avg, which divides the sums by the counts once and
// then drops the counts.
func (c *TermsWithScoreCollectorSVAvg) GetScoresPerTerm() []float32 {
	if c.scoreCounts != nil {
		for i := 0; i < len(c.scoreCounts); i++ {
			c.scoreSums[i] = c.scoreSums[i] / float32(c.scoreCounts[i])
		}
		c.scoreCounts = nil
	}
	return c.scoreSums
}

// CollectRange carries the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorSVAvg) CollectRange(minDoc, maxDoc int) error {
	return search.DefaultCollectRange(c, minDoc, maxDoc)
}

// CollectStream carries the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorSVAvg) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// TermsWithScoreCollectorMV is the impl that works with multiple values per
// document.
//
// Mirrors `static class MV extends TermsWithScoreCollector<SortedSetDocValues>`.
type TermsWithScoreCollectorMV struct {
	BaseTermsWithScoreCollector[index.SortedSetDocValues]
}

// NewTermsWithScoreCollectorMV mirrors
// `MV(Function<SortedSetDocValues> docValuesCall, ScoreMode scoreMode)`.
func NewTermsWithScoreCollectorMV(
	docValuesCall DocValuesTermsCollectorFunction[index.SortedSetDocValues],
	scoreMode ScoreMode,
) *TermsWithScoreCollectorMV {
	c := &TermsWithScoreCollectorMV{
		BaseTermsWithScoreCollector: *NewBaseTermsWithScoreCollector(docValuesCall, scoreMode),
	}
	c.Outer = c
	return c
}

// Collect mirrors `public void collect(int doc)` of TermsWithScoreCollector.MV.
func (c *TermsWithScoreCollectorMV) Collect(doc int) error {
	exists, err := c.docValues.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	for i := 0; i < c.docValues.DocValueCount(); i++ {
		ord, err := c.docValues.NextOrd()
		if err != nil {
			return err
		}
		bytes, err := c.docValues.LookupOrd(ord)
		if err != nil {
			return err
		}
		termID, err := c.collectedTerms.Add(util.WrapBytes(bytes))
		if err != nil {
			return err
		}
		if termID < 0 {
			termID = -termID - 1
		} else {
			if termID >= len(c.scoreSums) {
				begin := len(c.scoreSums)
				// ArrayUtil.grow(float[] array) is grow(array, 1 + array.length).
				c.scoreSums = util.GrowFloat32(c.scoreSums, 1+len(c.scoreSums))
				if c.scoreMode == Min {
					fillFloat32(c.scoreSums, begin, len(c.scoreSums), float32(math.Inf(1)))
				} else if c.scoreMode == Max {
					fillFloat32(c.scoreSums, begin, len(c.scoreSums), float32(math.Inf(-1)))
				}
			}
		}

		switch c.scoreMode {
		case Total:
			score, err := c.scorer.Score()
			if err != nil {
				return err
			}
			c.scoreSums[termID] += score
		case Min:
			score, err := c.scorer.Score()
			if err != nil {
				return err
			}
			c.scoreSums[termID] = min(c.scoreSums[termID], score)
		case Max:
			score, err := c.scorer.Score()
			if err != nil {
				return err
			}
			c.scoreSums[termID] = max(c.scoreSums[termID], score)
		case Avg, None:
			fallthrough
		default:
			// Java throws AssertionError, which is unchecked; collect(int) can
			// report only through its error return in Go.
			return fmt.Errorf("AssertionError: unexpected: %v", c.scoreMode)
		}
	}
	return nil
}

// CollectRange carries the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorMV) CollectRange(minDoc, maxDoc int) error {
	return search.DefaultCollectRange(c, minDoc, maxDoc)
}

// CollectStream carries the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorMV) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// TermsWithScoreCollectorMVAvg mirrors `static class Avg extends MV`.
type TermsWithScoreCollectorMVAvg struct {
	TermsWithScoreCollectorMV

	// scoreCounts renders `int[] scoreCounts = new int[INITIAL_ARRAY_SIZE]`.
	// getScoresPerTerm assigns null to it once the averages are computed; a Go
	// nil slice carries that state, and make() never yields nil, so the
	// zero-length initial value stays distinguishable from it.
	scoreCounts []int32
}

// NewTermsWithScoreCollectorMVAvg mirrors `Avg(Function<SortedSetDocValues> docValuesCall)`.
func NewTermsWithScoreCollectorMVAvg(
	docValuesCall DocValuesTermsCollectorFunction[index.SortedSetDocValues],
) *TermsWithScoreCollectorMVAvg {
	c := &TermsWithScoreCollectorMVAvg{
		TermsWithScoreCollectorMV: *NewTermsWithScoreCollectorMV(docValuesCall, Avg),
		scoreCounts:               make([]int32, initialArraySize),
	}
	c.Outer = c
	return c
}

// Collect mirrors `public void collect(int doc)` of TermsWithScoreCollector.MV.Avg.
func (c *TermsWithScoreCollectorMVAvg) Collect(doc int) error {
	exists, err := c.docValues.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	for i := 0; i < c.docValues.DocValueCount(); i++ {
		ord, err := c.docValues.NextOrd()
		if err != nil {
			return err
		}
		bytes, err := c.docValues.LookupOrd(ord)
		if err != nil {
			return err
		}
		termID, err := c.collectedTerms.Add(util.WrapBytes(bytes))
		if err != nil {
			return err
		}
		if termID < 0 {
			termID = -termID - 1
		} else {
			if termID >= len(c.scoreSums) {
				c.scoreSums = util.GrowFloat32(c.scoreSums, 1+len(c.scoreSums))
				c.scoreCounts = util.GrowInt32(c.scoreCounts, 1+len(c.scoreCounts))
			}
		}

		score, err := c.scorer.Score()
		if err != nil {
			return err
		}
		c.scoreSums[termID] += score
		c.scoreCounts[termID]++
	}
	return nil
}

// GetScoresPerTerm mirrors `public float[] getScoresPerTerm()` of
// TermsWithScoreCollector.MV.Avg.
func (c *TermsWithScoreCollectorMVAvg) GetScoresPerTerm() []float32 {
	if c.scoreCounts != nil {
		for i := 0; i < len(c.scoreCounts); i++ {
			c.scoreSums[i] = c.scoreSums[i] / float32(c.scoreCounts[i])
		}
		c.scoreCounts = nil
	}
	return c.scoreSums
}

// CollectRange carries the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorMVAvg) CollectRange(minDoc, maxDoc int) error {
	return search.DefaultCollectRange(c, minDoc, maxDoc)
}

// CollectStream carries the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (c *TermsWithScoreCollectorMVAvg) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// interface compliance
var (
	_ TermsWithScoreCollector = (*TermsWithScoreCollectorSV)(nil)
	_ TermsWithScoreCollector = (*TermsWithScoreCollectorSVAvg)(nil)
	_ TermsWithScoreCollector = (*TermsWithScoreCollectorMV)(nil)
	_ TermsWithScoreCollector = (*TermsWithScoreCollectorMVAvg)(nil)
)
