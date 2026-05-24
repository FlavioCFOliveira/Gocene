// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.plain.histograms.HistogramCollector.
package histograms

import (
	"fmt"
	"math"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/internal/hppc"
)

// NumericDocValuesEx is the minimal interface over
// org.apache.lucene.index.NumericDocValues that HistogramCollector needs.
//
// Gocene's index.NumericDocValues interface uses a different API than
// Java (no AdvanceExact / LongValue / DocIDRunEnd). Callers that hold an
// index.NumericDocValues must provide a bridge that implements this
// interface.
type NumericDocValuesEx interface {
	// AdvanceExact advances to doc and returns true when the doc has a value.
	AdvanceExact(doc int) (bool, error)
	// LongValue returns the value for the most recently advanced document.
	LongValue() (int64, error)
	// DocIDRunEnd returns the highest doc ID (exclusive) such that all docs
	// in [currentDocID, DocIDRunEnd) are guaranteed to have a value.
	DocIDRunEnd() (int, error)
	// LongValues fills dst[0:n] with values for docs in docBuffer[0:n].
	// The missing sentinel is used for docs without a value.
	LongValues(n int, docBuffer []int, dst []int64, missingSentinel int64) error
}

// SortedNumericDocValuesEx is the minimal interface over
// org.apache.lucene.index.SortedNumericDocValues that HistogramCollector
// needs.
type SortedNumericDocValuesEx interface {
	// AdvanceExact advances to doc and returns true when the doc has a value.
	AdvanceExact(doc int) (bool, error)
	// DocValueCount returns the number of values for the current document.
	DocValueCount() int
	// NextValue returns the next value for the current document.
	NextValue() (int64, error)
}

// DocValuesSkipperEx is the full interface over
// org.apache.lucene.index.DocValuesSkipper that HistogramLeafCollector
// needs.
//
// Gocene's index.DocValuesSkipper only exposes SkipTo/GetDocID. Callers
// that hold a richer skipper must provide a bridge that implements this
// interface.
type DocValuesSkipperEx interface {
	// Advance moves the skipper to the given document.
	Advance(target int) error
	// MaxDocID returns the inclusive upper bound of document IDs at level.
	MaxDocID(level int) int
	// MinDocID returns the inclusive lower bound of document IDs at level.
	MinDocID(level int) int
	// NumLevels returns the number of skip levels.
	NumLevels() int
	// MinValue returns the minimum field value at the given level.
	MinValue(level int) int64
	// MaxValue returns the maximum field value at the given level.
	MaxValue(level int) int64
	// DocCount returns the number of documents that have a value at level.
	DocCount(level int) int
}

// HistogramLeafContext carries the per-segment resources needed by
// HistogramCollector.GetLeafCollector. Callers (typically
// HistogramCollectorManager) must populate it from the segment reader.
type HistogramLeafContext struct {
	// FieldDocValuesType is either "NUMERIC" or "SORTED_NUMERIC"; any
	// other value causes an error to be returned from GetLeafCollector.
	FieldDocValuesType string
	// NumericValues is the NumericDocValues for the field, or nil.
	NumericValues NumericDocValuesEx
	// SortedNumericValues is the SortedNumericDocValues for the field, or nil.
	SortedNumericValues SortedNumericDocValuesEx
	// Skipper is the DocValuesSkipperEx for the field, or nil.
	Skipper DocValuesSkipperEx
	// PointValues is the HistogramPointValues for the field, or nil.
	PointValues HistogramPointValues
	// MaxDoc is the maximum doc ID (exclusive) in this segment.
	MaxDoc int
	// MatchAll is true when the query matches all documents in this segment.
	MatchAll bool
	// PointRangeFilter is non-nil when the query is a PointRangeQuery
	// (or an equivalent wrapped query) on this field.
	PointRangeFilter *PointRangeFilter
	// SegmentKey is an opaque identifier for the segment; used by
	// HistogramCollectorManager to track which segments have been
	// bulk-collected via the PointTree path.
	SegmentKey any
}

// HistogramCollector is a Collector that accumulates per-bucket counts.
//
// Mirrors org.apache.lucene.sandbox.facet.plain.histograms.HistogramCollector.
type HistogramCollector struct {
	field       string
	bucketWidth int64
	maxBuckets  int
	counts      hppc.LongIntHashMap
	// leafBulkCollected is a pointer shared across all collectors created
	// by the same HistogramCollectorManager to track segments already
	// processed via the PointTree bulk-collection path.
	leafBulkCollected *sync.Map
}

// newHistogramCollector creates an internal HistogramCollector.
func newHistogramCollector(
	field string,
	bucketWidth int64,
	maxBuckets int,
	leafBulkCollected *sync.Map,
) *HistogramCollector {
	return &HistogramCollector{
		field:             field,
		bucketWidth:       bucketWidth,
		maxBuckets:        maxBuckets,
		counts:            make(hppc.LongIntHashMap),
		leafBulkCollected: leafBulkCollected,
	}
}

// GetCounts returns the accumulated histogram counts. The map key is
// floor(value / bucketWidth).
func (hc *HistogramCollector) GetCounts() hppc.LongIntHashMap {
	return hc.counts
}

// GetLeafCollector returns a LeafCollector for the given segment context.
//
// ctx must carry the per-segment resources (doc values, skipper, point
// values). Returns a wrapped ErrCollectionTerminated when the segment
// contributes no documents (field absent or bulk-collected).
//
// Mirrors HistogramCollector.getLeafCollector(LeafReaderContext).
func (hc *HistogramCollector) GetLeafCollector(ctx *HistogramLeafContext) (HistogramLeafCollector, error) {
	if ctx.FieldDocValuesType == "" {
		// Field absent in this segment — signal early termination.
		return nil, ErrCollectionTerminated
	}
	if ctx.FieldDocValuesType != "NUMERIC" && ctx.FieldDocValuesType != "SORTED_NUMERIC" {
		return nil, fmt.Errorf(
			"HistogramCollector.GetLeafCollector: expected numeric field, got %q",
			ctx.FieldDocValuesType,
		)
	}

	// Attempt PointTree bulk collection when available.
	if ctx.PointValues != nil && (ctx.MatchAll || ctx.PointRangeFilter != nil) {
		if CanCollectEfficiently(ctx.PointValues, hc.bucketWidth) {
			// Only one collector should bulk-collect per segment.
			bulkMap := hc.leafBulkCollected
			if bulkMap == nil {
				bulkMap = &sync.Map{}
			}
			if _, loaded := bulkMap.LoadOrStore(ctx.SegmentKey, true); !loaded {
				if err := Collect(ctx.PointValues, ctx.PointRangeFilter, hc.bucketWidth, hc.counts, hc.maxBuckets); err != nil && !errIsTerminated(err) {
					return nil, fmt.Errorf("HistogramCollector.GetLeafCollector: bulk collect: %w", err)
				}
			}
			return nil, ErrCollectionTerminated
		}
	}

	// Fall back to doc-values iteration.
	if ctx.SortedNumericValues != nil {
		singleton := ctx.NumericValues
		if singleton == nil {
			// Multi-valued path.
			return &histogramNaiveLeafCollector{
				values:      ctx.SortedNumericValues,
				bucketWidth: hc.bucketWidth,
				maxBuckets:  hc.maxBuckets,
				counts:      hc.counts,
			}, nil
		}
		// Single-valued path with optional skipper optimisation.
		if ctx.Skipper != nil {
			leafMinBucket := floorDiv(ctx.Skipper.MinValue(0), hc.bucketWidth)
			leafMaxBucket := floorDiv(ctx.Skipper.MaxValue(0), hc.bucketWidth)
			if leafMaxBucket-leafMinBucket <= 1024 {
				return newHistogramLeafCollector(singleton, ctx.Skipper, hc.bucketWidth, hc.maxBuckets, hc.counts), nil
			}
		}
		return &histogramNaiveSingleValuedLeafCollector{
			values:      singleton,
			bucketWidth: hc.bucketWidth,
			maxBuckets:  hc.maxBuckets,
			counts:      hc.counts,
		}, nil
	}

	return nil, fmt.Errorf("HistogramCollector.GetLeafCollector: no doc-values available for field %q", hc.field)
}

// HistogramLeafCollector is the per-segment collector interface.
// The optional Finish() allows HistogramLeafCollector to flush dense
// intermediate arrays at the end of segment collection.
type HistogramLeafCollector interface {
	// Collect is called for each matching document.
	Collect(doc int) error
	// Finish is called after all matching documents have been collected.
	Finish() error
}

// CheckMaxBuckets panics when size exceeds maxBuckets.
//
// Mirrors HistogramCollector.checkMaxBuckets (static helper).
func CheckMaxBuckets(size, maxBuckets int) {
	if size > maxBuckets {
		panic(fmt.Sprintf(
			"Collected %d buckets, which is more than the configured max number of buckets: %d",
			size, maxBuckets,
		))
	}
}

// errIsTerminated reports whether err wraps ErrCollectionTerminated.
func errIsTerminated(err error) bool {
	return err != nil && (err == ErrCollectionTerminated)
}

// histogramNaiveLeafCollector is used for multi-valued fields (SORTED_NUMERIC
// without an unwrapped singleton).
//
// Mirrors HistogramCollector.HistogramNaiveLeafCollector.
type histogramNaiveLeafCollector struct {
	values      SortedNumericDocValuesEx
	bucketWidth int64
	maxBuckets  int
	counts      hppc.LongIntHashMap
}

func (c *histogramNaiveLeafCollector) Collect(doc int) error {
	ok, err := c.values.AdvanceExact(doc)
	if err != nil {
		return fmt.Errorf("histogramNaiveLeafCollector.Collect: %w", err)
	}
	if !ok {
		return nil
	}
	n := c.values.DocValueCount()
	var prevBucket int64 = math.MinInt64
	for i := 0; i < n; i++ {
		v, err := c.values.NextValue()
		if err != nil {
			return fmt.Errorf("histogramNaiveLeafCollector.Collect: %w", err)
		}
		bucket := floorDiv(v, c.bucketWidth)
		if bucket != prevBucket {
			c.counts[bucket]++
			CheckMaxBuckets(len(c.counts), c.maxBuckets)
			prevBucket = bucket
		}
	}
	return nil
}

func (c *histogramNaiveLeafCollector) Finish() error { return nil }

// histogramNaiveSingleValuedLeafCollector is used for single-valued fields
// when no DocValuesSkipper is available or when too many buckets exist.
//
// Mirrors HistogramCollector.HistogramNaiveSingleValuedLeafCollector.
type histogramNaiveSingleValuedLeafCollector struct {
	values      NumericDocValuesEx
	bucketWidth int64
	maxBuckets  int
	counts      hppc.LongIntHashMap
	docBuffer   [64]int
	valueBuffer [64]int64
}

func (c *histogramNaiveSingleValuedLeafCollector) Collect(doc int) error {
	ok, err := c.values.AdvanceExact(doc)
	if err != nil {
		return fmt.Errorf("histogramNaiveSingleValuedLeafCollector.Collect: %w", err)
	}
	if !ok {
		return nil
	}
	v, err := c.values.LongValue()
	if err != nil {
		return fmt.Errorf("histogramNaiveSingleValuedLeafCollector.Collect: %w", err)
	}
	bucket := floorDiv(v, c.bucketWidth)
	c.counts[bucket]++
	CheckMaxBuckets(len(c.counts), c.maxBuckets)
	return nil
}

func (c *histogramNaiveSingleValuedLeafCollector) Finish() error { return nil }

// histogramLeafCollector is the optimized collector that leverages the
// DocValuesSkipperEx to skip over blocks of documents that all fall into
// the same bucket.
//
// Mirrors HistogramCollector.HistogramLeafCollector.
type histogramLeafCollector struct {
	values        NumericDocValuesEx
	skipper       DocValuesSkipperEx
	bucketWidth   int64
	maxBuckets    int
	counts        []int
	leafMinBucket int64
	collCounts    hppc.LongIntHashMap
	// upToInclusive is the highest doc ID (inclusive) for which we know the
	// bucket membership without re-querying the skipper.
	upToInclusive   int
	upToSameBucket  bool
	upToBucketIndex int
}

func newHistogramLeafCollector(
	values NumericDocValuesEx,
	skipper DocValuesSkipperEx,
	bucketWidth int64,
	maxBuckets int,
	collCounts hppc.LongIntHashMap,
) *histogramLeafCollector {
	leafMinBucket := floorDiv(skipper.MinValue(0), bucketWidth)
	leafMaxBucket := floorDiv(skipper.MaxValue(0), bucketWidth)
	size := int(leafMaxBucket - leafMinBucket + 1)
	return &histogramLeafCollector{
		values:        values,
		skipper:       skipper,
		bucketWidth:   bucketWidth,
		maxBuckets:    maxBuckets,
		counts:        make([]int, size),
		leafMinBucket: leafMinBucket,
		collCounts:    collCounts,
		upToInclusive: -1,
	}
}

func (c *histogramLeafCollector) advanceSkipper(doc int) error {
	if err := c.skipper.Advance(doc); err != nil {
		return err
	}
	c.upToSameBucket = false

	if c.skipper.MinDocID(0) > doc {
		c.upToInclusive = c.skipper.MinDocID(0) - 1
		return nil
	}

	c.upToInclusive = c.skipper.MaxDocID(0)

	for level := 0; level < c.skipper.NumLevels(); level++ {
		totalDocs := c.skipper.MaxDocID(level) - c.skipper.MinDocID(level) + 1
		minBucket := floorDiv(c.skipper.MinValue(level), c.bucketWidth)
		maxBucket := floorDiv(c.skipper.MaxValue(level), c.bucketWidth)
		if c.skipper.DocCount(level) == totalDocs && minBucket == maxBucket {
			c.upToInclusive = c.skipper.MaxDocID(level)
			c.upToSameBucket = true
			c.upToBucketIndex = int(minBucket - c.leafMinBucket)
		} else {
			break
		}
	}
	return nil
}

func (c *histogramLeafCollector) Collect(doc int) error {
	if doc > c.upToInclusive {
		if err := c.advanceSkipper(doc); err != nil {
			return fmt.Errorf("histogramLeafCollector.Collect: %w", err)
		}
	}
	if c.upToSameBucket {
		c.counts[c.upToBucketIndex]++
		return nil
	}
	ok, err := c.values.AdvanceExact(doc)
	if err != nil {
		return fmt.Errorf("histogramLeafCollector.Collect: %w", err)
	}
	if ok {
		v, err := c.values.LongValue()
		if err != nil {
			return fmt.Errorf("histogramLeafCollector.Collect: %w", err)
		}
		bucket := floorDiv(v, c.bucketWidth)
		c.counts[int(bucket-c.leafMinBucket)]++
	}
	return nil
}

func (c *histogramLeafCollector) Finish() error {
	for i, cnt := range c.counts {
		if cnt != 0 {
			key := c.leafMinBucket + int64(i)
			c.collCounts[key] += int32(cnt)
		}
	}
	CheckMaxBuckets(len(c.collCounts), c.maxBuckets)
	return nil
}
