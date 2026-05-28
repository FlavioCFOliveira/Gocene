// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// XYPointDistanceComparator compares documents by Euclidean distance from a
// cartesian origin (x, y). It mirrors the package-private
// org.apache.lucene.document.XYPointDistanceComparator and reads the
// SortedNumericDocValues stream produced by XYDocValuesField — upper 32 bits
// of each long carry the sortable-int-encoded x, lower 32 bits carry y.
//
// The comparator caches a pre-encoded bounding box derived from the current
// bottom-of-queue distance; uncompetitive candidates are rejected on the
// integer-encoded coordinates without paying the Euclidean cost. After 1024
// setBottom calls the box is refreshed only every 64 calls, matching the
// adversarial-input guard in Lucene (worst-case backwards distance order).
//
// Although the constructor takes float32 coordinates (to match Lucene's
// signature), they are widened to float64 internally so the squared-distance
// arithmetic and the bottom-distance bookkeeping use the same precision as
// the LatLon sibling. The bounding-box pre-encoding does cast back to float32
// before calling geo.FromXYPointDistance, which itself accepts only float32.
//
// Concurrency: not safe for concurrent use; TopFieldCollector owns one
// instance per slice.
type XYPointDistanceComparator struct {
	field string
	x     float64
	y     float64

	// values holds one distance per priority-queue slot. The value stored is
	// the Euclidean distance (not a sort key), matching Lucene's choice not
	// to use a squared-distance sort key — see the Javadoc note about the
	// square-root being needed to avoid numerical issues where squared
	// distances differ but actual distances are equal.
	values   []float64
	bottom   float64
	topValue float64

	currentDocs index.SortedNumericDocValues

	// Pre-encoded competitive bounding box for the bottom distance on the
	// priority queue. The defaults span the full 32-bit range so the first
	// CompareBottom call (before any SetBottom) never spuriously rejects.
	minX int32
	maxX int32
	minY int32
	maxY int32

	// setBottomCounter counts setBottom invocations for the adversary guard.
	setBottomCounter int

	// currentValues caches the decoded values for the current docID, so that
	// CompareBottom and the sortKey helper do not refetch.
	currentValues []int64
	valuesDocID   int
}

// NewXYPointDistanceComparator builds a comparator backed by numHits slots
// targeting the SortedNumericDocValues stream of field. The Java constructor
// takes float32 coordinates; we keep that signature and widen internally.
func NewXYPointDistanceComparator(field string, x, y float32, numHits int) *XYPointDistanceComparator {
	return &XYPointDistanceComparator{
		field:         field,
		x:             float64(x),
		y:             float64(y),
		values:        make([]float64, numHits),
		minX:          math.MinInt32,
		maxX:          math.MaxInt32,
		minY:          math.MinInt32,
		maxY:          math.MaxInt32,
		currentValues: make([]int64, 0, 4),
		valuesDocID:   -1,
	}
}

// SetScorer is a no-op, matching the Java reference.
func (c *XYPointDistanceComparator) SetScorer(_ Scorable) error { return nil }

// Compare returns the natural ordering of the two slots' distances. A smaller
// value means closer to the origin.
func (c *XYPointDistanceComparator) Compare(slot1, slot2 int) int {
	return compareFloat64(c.values[slot1], c.values[slot2])
}

// SetBottom records the slot of the queue's weakest entry and refreshes the
// pre-encoded competitive bounding box. The bounding box is rebuilt on every
// call for the first 1024 invocations, then every 64 calls thereafter, in
// line with the Java adversary guard.
//
// The Java guard uses `bottom < Float.MAX_VALUE`; we use the same comparison
// against math.MaxFloat32 (widened to float64). When bottom is +Inf or larger
// than MaxFloat32 the box is left untouched at the full-range defaults.
func (c *XYPointDistanceComparator) SetBottom(slot int) error {
	c.bottom = c.values[slot]
	if c.bottom < math.MaxFloat32 &&
		(c.setBottomCounter < 1024 || (c.setBottomCounter&0x3F) == 0x3F) {
		rect, err := geo.FromXYPointDistance(float32(c.x), float32(c.y), float32(c.bottom))
		if err == nil {
			c.minX = geo.XYEncode(rect.MinX())
			c.maxX = geo.XYEncode(rect.MaxX())
			c.minY = geo.XYEncode(rect.MinY())
			c.maxY = geo.XYEncode(rect.MaxY())
		}
	}
	c.setBottomCounter++
	return nil
}

// SetTopValue stores the top reference distance used by CompareTop.
func (c *XYPointDistanceComparator) SetTopValue(value float64) {
	c.topValue = value
}

// Value returns the Euclidean distance stored in slot. Unlike the LatLon
// sibling, no sort-key → metres conversion is required: the stored value is
// already the distance.
func (c *XYPointDistanceComparator) Value(slot int) float64 {
	return c.values[slot]
}

// CompareBottom returns a positive integer when doc is more competitive than
// the current bottom, zero on tie, and a negative value when doc cannot
// improve the queue. The implementation mirrors the Java logic: bounding-box
// rejection first, then exact Euclidean distance on survivors.
func (c *XYPointDistanceComparator) CompareBottom(doc int) (int, error) {
	if c.currentDocs == nil {
		return compareFloat64(c.bottom, math.Inf(1)), nil
	}
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, fmt.Errorf("xy point sort: advance to %d: %w", doc, err)
		}
	}
	if doc < c.currentDocs.DocID() {
		return compareFloat64(c.bottom, math.Inf(1)), nil
	}
	if err := c.setValues(); err != nil {
		return 0, err
	}
	cmp := -1
	for _, encoded := range c.currentValues {
		xBits := int32(encoded >> 32)
		if xBits < c.minX || xBits > c.maxX {
			continue
		}
		yBits := int32(encoded)
		if yBits < c.minY || yBits > c.maxY {
			continue
		}
		docX := float64(geo.XYDecode(xBits))
		docY := float64(geo.XYDecode(yBits))
		diffX := c.x - docX
		diffY := c.y - docY
		distance := math.Sqrt(diffX*diffX + diffY*diffY)
		thisCmp := compareFloat64(c.bottom, distance)
		if thisCmp > cmp {
			cmp = thisCmp
		}
		if cmp > 0 {
			return cmp, nil
		}
	}
	return cmp, nil
}

// Copy stores the distance for doc into slot, computed across all of the
// document's multi-valued points (the minimum wins).
func (c *XYPointDistanceComparator) Copy(slot, doc int) error {
	dist, err := c.sortKey(doc)
	if err != nil {
		return err
	}
	c.values[slot] = dist
	return nil
}

// CompareTop compares the top reference distance against the distance to doc.
// Unlike the LatLon sibling, no unit conversion is needed: both values are
// Euclidean distances in the same units as the input coordinates.
func (c *XYPointDistanceComparator) CompareTop(doc int) (int, error) {
	docDist, err := c.sortKey(doc)
	if err != nil {
		return 0, err
	}
	return compareFloat64(c.topValue, docDist), nil
}

// CompetitiveIterator returns nil — the comparator does not expose a
// competitive iterator optimisation, matching the Java reference.
func (c *XYPointDistanceComparator) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}

// SetHitsThresholdReached is a no-op for this comparator.
func (c *XYPointDistanceComparator) SetHitsThresholdReached() {}

// GetLeafComparator binds the comparator to ctx by resolving the
// SortedNumericDocValues stream for the configured field. The Java reference
// invokes XYDocValuesField.checkCompatible(FieldInfo) before the lookup; that
// helper has not been ported in Gocene yet, so we follow the LatLon sibling
// and defer field-type validation to the leaf reader. A reader that does not
// expose the doc-values surface falls back to an empty stream, mirroring
// Lucene's DocValues.getSortedNumeric null-defence path.
func (c *XYPointDistanceComparator) GetLeafComparator(ctx *index.LeafReaderContext) (*XYPointDistanceComparator, error) {
	if ctx == nil {
		return nil, errors.New("leaf reader context must not be nil")
	}
	type sortedNumericProvider interface {
		GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
	}
	c.valuesDocID = -1
	leaf := ctx.LeafReader()
	if leaf == nil {
		c.currentDocs = index.EmptySortedNumeric()
		return c, nil
	}
	provider, ok := leaf.(sortedNumericProvider)
	if !ok {
		c.currentDocs = index.EmptySortedNumeric()
		return c, nil
	}
	dv, err := provider.GetSortedNumericDocValues(c.field)
	if err != nil {
		return nil, fmt.Errorf("xy point sort: read doc values %q: %w", c.field, err)
	}
	if dv == nil {
		c.currentDocs = index.EmptySortedNumeric()
	} else {
		c.currentDocs = dv
	}
	return c, nil
}

// setValues materialises the per-document multi-values into currentValues,
// re-fetching only when the docID changes. Mirrors the Java setValues hook
// that drains SortedNumericDocValues.nextValue into a long[] cache.
func (c *XYPointDistanceComparator) setValues() error {
	if c.currentDocs == nil {
		c.currentValues = c.currentValues[:0]
		return nil
	}
	docID := c.currentDocs.DocID()
	if c.valuesDocID == docID {
		return nil
	}
	c.valuesDocID = docID
	// currentDocs is already positioned on docID by the comparator's
	// SetScorer/Advance path; CollectSortedNumericValues drains the
	// per-doc values via DocValueCount + NextValue.
	vals, err := index.CollectSortedNumericValues(c.currentDocs)
	if err != nil {
		return fmt.Errorf("xy point sort: get values @doc=%d: %w", docID, err)
	}
	if cap(c.currentValues) < len(vals) {
		c.currentValues = make([]int64, len(vals))
	} else {
		c.currentValues = c.currentValues[:len(vals)]
	}
	copy(c.currentValues, vals)
	return nil
}

// sortKey returns the minimum Euclidean distance across the multi-valued
// points of doc; +Inf when doc has no values in the current leaf or the
// stream cannot be advanced to doc.
func (c *XYPointDistanceComparator) sortKey(doc int) (float64, error) {
	if c.currentDocs == nil {
		return math.Inf(1), nil
	}
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, fmt.Errorf("xy point sort: advance to %d: %w", doc, err)
		}
	}
	if doc != c.currentDocs.DocID() {
		return math.Inf(1), nil
	}
	if err := c.setValues(); err != nil {
		return 0, err
	}
	minValue := math.Inf(1)
	for _, encoded := range c.currentValues {
		docX := float64(geo.XYDecode(int32(encoded >> 32)))
		docY := float64(geo.XYDecode(int32(encoded)))
		diffX := c.x - docX
		diffY := c.y - docY
		dist := math.Sqrt(diffX*diffX + diffY*diffY)
		if dist < minValue {
			minValue = dist
		}
	}
	return minValue, nil
}
