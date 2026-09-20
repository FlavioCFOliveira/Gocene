// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/comparators/TermOrdValComparator.java
//
// termOrdValComparator sorts by the term ordinal of a SortedDocValues field.
// Within a single segment, the document order is the ordinal order, so
// comparisons are integer ordinal subtractions. Across segments the ordinals
// are not comparable, so the comparator caches the resolved term bytes per slot
// (alongside the segment generation that produced them) and falls back to a
// byte comparison when two slots come from different segments — exactly as
// Lucene's TermOrdValComparator.compare does.

// sortedDocValuesReader is the narrow leaf-reader view needed to obtain the
// per-field SortedDocValues iterator. SegmentReader and LeafReader satisfy it.
type sortedDocValuesReader interface {
	GetSortedDocValues(field string) (index.SortedDocValues, error)
}

// termOrdValComparator is the SortFieldTypeString field comparator.
//
// Mirrors org.apache.lucene.search.comparators.TermOrdValComparator.
type termOrdValComparator struct {
	BaseFieldComparator

	field string

	// ords[slot] is the term ordinal cached for the slot, valid only for the
	// segment recorded in readerGen[slot]. values[slot] is the resolved term
	// bytes (nil for a missing value), comparable across segments.
	ords      []int
	values    [][]byte
	readerGen []int

	// missingSortCmp is +1 when missing values sort last, -1 when first. This is
	// the sign returned by compare when exactly one of the two values is missing.
	missingSortCmp int
	// missingOrd is the sentinel ordinal substituted for a missing value within a
	// segment: MaxInt when missing-last, -1 when missing-first. It is recomputed
	// per leaf because the previous-leaf value would otherwise leak.
	missingOrd int

	// currentReaderGen increments on every getLeafComparator call so a stale
	// per-slot ord (cached in a previous segment) is detected.
	currentReaderGen int

	// bottom state, refreshed by SetBottom.
	bottomSlot       int
	bottomOrd        int
	bottomValue      []byte
	bottomSameReader bool

	// topValue is the term recorded by SetTopValue (nil means the last doc of
	// the prior search was missing this value). topOrd / topSameReader are its
	// per-leaf resolution, recomputed by setReader exactly as Lucene's
	// TermOrdValLeafComparator constructor does.
	topValue      []byte
	topOrd        int
	topSameReader bool

	termsIndex index.SortedDocValues

	// dvSource, when non-nil, overrides the default field lookup in setReader,
	// mirroring TermOrdValComparator.getSortedDocValues being overridden by
	// ToParentBlockJoinSortField to feed a BlockJoinSelector-wrapped iterator.
	dvSource SortedDocValuesSource
}

// newTermOrdValComparator constructs the comparator. sortMissingLast selects the
// missing-value placement (true: missing sorts after every present value).
func newTermOrdValComparator(numHits int, field string, sortMissingLast bool) *termOrdValComparator {
	c := &termOrdValComparator{
		field:      field,
		ords:       make([]int, numHits),
		values:     make([][]byte, numHits),
		readerGen:  make([]int, numHits),
		bottomSlot: -1,
	}
	if sortMissingLast {
		c.missingSortCmp = 1
	} else {
		c.missingSortCmp = -1
	}
	return c
}

// compare orders two queue slots. Same-segment slots compare by raw ordinal;
// cross-segment slots compare by cached term bytes with the missing sentinel.
//
// Mirrors TermOrdValComparator.compare.
func (c *termOrdValComparator) Compare(slot1, slot2 int) int {
	if c.readerGen[slot1] == c.readerGen[slot2] {
		return c.ords[slot1] - c.ords[slot2]
	}
	v1, v2 := c.values[slot1], c.values[slot2]
	if v1 == nil {
		if v2 == nil {
			return 0
		}
		return c.missingSortCmp
	} else if v2 == nil {
		return -c.missingSortCmp
	}
	return bytes.Compare(v1, v2)
}

func (c *termOrdValComparator) Value(slot int) any {
	v := c.values[slot]
	if v == nil {
		return nil
	}
	// Return a copy so callers cannot mutate the cached term bytes.
	out := make([]byte, len(v))
	copy(out, v)
	return out
}

// setReader binds the comparator to a new segment. It increments the reader
// generation, resolves the SortedDocValues iterator, and recomputes missingOrd.
//
// Mirrors TermOrdValComparator.getLeafComparator.
func (c *termOrdValComparator) setReader(reader IndexReader) error {
	c.currentReaderGen++
	c.termsIndex = nil
	if c.dvSource != nil {
		dv, err := c.dvSource.SortedDocValues(reader, c.field)
		if err != nil {
			return err
		}
		c.termsIndex = dv
	} else if r, ok := reader.(sortedDocValuesReader); ok {
		dv, err := r.GetSortedDocValues(c.field)
		if err != nil {
			return err
		}
		c.termsIndex = dv
	}
	if c.missingSortCmp == 1 {
		c.missingOrd = maxInt
	} else {
		c.missingOrd = -1
	}
	// Recompute topOrd / topSameReader for the new segment.
	if c.topValue != nil {
		ord, err := c.lookupTerm(c.topValue)
		if err != nil {
			return err
		}
		if ord >= 0 {
			c.topSameReader = true
			c.topOrd = ord
		} else {
			c.topSameReader = false
			c.topOrd = -ord - 2
		}
	} else {
		c.topOrd = c.missingOrd
		c.topSameReader = true
	}
	// If a bottom was set on a previous leaf, recompute its per-leaf ord.
	if c.bottomSlot != -1 {
		return c.SetBottom(c.bottomSlot)
	}
	return nil
}

// GetLeafComparator binds this comparator to the segment and returns itself as
// its own per-leaf view.
//
// Mirrors TermOrdValComparator.getLeafComparator(LeafReaderContext), which
// increments currentReaderGen and builds a TermOrdValLeafComparator; this port
// carries no per-leaf skipping state, so the single instance is the leaf
// comparator.
func (c *termOrdValComparator) GetLeafComparator(context *index.LeafReaderContext) (LeafFieldComparator, error) {
	if err := c.setReader(leafReaderOf(context)); err != nil {
		return nil, err
	}
	return c, nil
}

// SetTopValue records the top term for CompareTop. A nil value is meaningful:
// it means the last doc of the prior search was missing this value.
//
// Mirrors TermOrdValComparator.setTopValue(BytesRef).
func (c *termOrdValComparator) SetTopValue(value any) { c.topValue = topValueBytes(value) }

// CompareValues orders two cached term values, with missing sorting according
// to the comparator's missing-value placement.
//
// Mirrors TermOrdValComparator.compareValues(BytesRef, BytesRef), which
// overrides the FieldComparator default.
func (c *termOrdValComparator) CompareValues(first, second any) int {
	val1 := bytesRefValue(first, "CompareValues")
	val2 := bytesRefValue(second, "CompareValues")
	if val1 == nil {
		if val2 == nil {
			return 0
		}
		return c.missingSortCmp
	} else if val2 == nil {
		return -c.missingSortCmp
	}
	return bytes.Compare(val1, val2)
}

// getOrdForDoc returns the term ordinal of doc, or -1 when the document has no
// value for the field. Callers advance with increasing doc ids.
func (c *termOrdValComparator) getOrdForDoc(doc int) (int, error) {
	if c.termsIndex == nil {
		return -1, nil
	}
	exists, err := c.termsIndex.AdvanceExact(doc)
	if err != nil {
		return 0, err
	}
	if !exists {
		return -1, nil
	}
	return c.termsIndex.OrdValue()
}

// SetBottom caches the bottom slot's ordinal. When the bottom slot's value came
// from a different segment, the cross-segment comparison must use bytes, so we
// resolve the bottom term's ordinal in the current segment by binary search
// over LookupOrd (Lucene uses lookupTerm); a not-found term yields an insertion
// point so compareBottom can still order correctly.
//
// Mirrors TermOrdValComparator.setBottom.
func (c *termOrdValComparator) SetBottom(slot int) error {
	c.bottomSlot = slot
	c.bottomValue = c.values[slot]
	if c.currentReaderGen == c.readerGen[slot] {
		c.bottomOrd = c.ords[slot]
		c.bottomSameReader = true
		return nil
	}
	if c.bottomValue == nil {
		c.bottomOrd = c.missingOrd
		c.bottomSameReader = true
		c.readerGen[slot] = c.currentReaderGen
		return nil
	}
	ord, err := c.lookupTerm(c.bottomValue)
	if err != nil {
		return err
	}
	if ord < 0 {
		// Insertion point: -ord-1 is the index of the first term greater than the
		// bottom value; Lucene stores -ord-2 so the not-equal branch in
		// compareBottom is taken. We mirror that exactly.
		c.bottomOrd = -ord - 2
		c.bottomSameReader = false
	} else {
		c.bottomOrd = ord
		c.bottomSameReader = true
		c.readerGen[slot] = c.currentReaderGen
		c.ords[slot] = c.bottomOrd
	}
	return nil
}

// CompareBottom returns the Java sign convention (>0 if doc is better than the
// bottom). Same-reader compares by ordinal; otherwise it accounts for the
// insertion-point ambiguity exactly as Lucene does.
//
// Mirrors TermOrdValComparator.compareBottom.
func (c *termOrdValComparator) CompareBottom(doc int) (int, error) {
	docOrd, err := c.getOrdForDoc(doc)
	if err != nil {
		return 0, err
	}
	if docOrd == -1 {
		docOrd = c.missingOrd
	}
	if c.bottomSameReader {
		return c.bottomOrd - docOrd, nil
	}
	if c.bottomOrd >= docOrd {
		// bottom term sorts after doc's term (bottomOrd was the lower bound).
		return 1, nil
	}
	return -1, nil
}

// CompareTop compares the top term recorded by SetTopValue with doc's term.
// Same-reader compares by ordinal; otherwise the insertion point stored in
// topOrd is a lower bound, so the equal case means doc sorts before the top.
//
// Mirrors TermOrdValComparator.TermOrdValLeafComparator.compareTop.
func (c *termOrdValComparator) CompareTop(doc int) (int, error) {
	ord, err := c.getOrdForDoc(doc)
	if err != nil {
		return 0, err
	}
	if ord == -1 {
		ord = c.missingOrd
	}
	if c.topSameReader {
		// ord is precisely comparable, even in the equal case.
		return c.topOrd - ord, nil
	} else if ord <= c.topOrd {
		// The equals case always means doc is < value (because topOrd was set
		// to the lower bound).
		return 1, nil
	}
	return -1, nil
}

// Copy resolves and caches doc's ordinal and term bytes into the slot.
//
// Mirrors TermOrdValComparator.copy.
func (c *termOrdValComparator) Copy(slot, doc int) error {
	ord, err := c.getOrdForDoc(doc)
	if err != nil {
		return err
	}
	if ord == -1 {
		ord = c.missingOrd
		c.values[slot] = nil
	} else {
		term, err := c.termsIndex.LookupOrd(ord)
		if err != nil {
			return err
		}
		cp := make([]byte, len(term))
		copy(cp, term)
		c.values[slot] = cp
	}
	c.ords[slot] = ord
	c.readerGen[slot] = c.currentReaderGen
	return nil
}

func (c *termOrdValComparator) SetScorer(Scorable) error                       { return nil }
func (c *termOrdValComparator) CompetitiveIterator() (DocIdSetIterator, error) { return nil, nil }
func (c *termOrdValComparator) SetHitsThresholdReached() error                 { return nil }

// lookupTerm performs a binary search over the segment's ordinals for the given
// term bytes. It returns the matching ordinal, or a negative insertion point
// (-(insertionPoint)-1) when the term is absent — the same contract as Lucene's
// SortedDocValues.lookupTerm, ported as index.SortedDocValuesLookupTerm.
func (c *termOrdValComparator) lookupTerm(target []byte) (int, error) {
	if c.termsIndex == nil {
		return -1, nil
	}
	return index.SortedDocValuesLookupTerm(c.termsIndex, target)
}

// maxInt is the platform-independent maximum int used as the missing-last
// ordinal sentinel (matching Java Integer.MAX_VALUE semantics for ordering).
const maxInt = int(^uint(0) >> 1)
