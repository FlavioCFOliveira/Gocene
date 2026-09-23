// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermGroupFacetCollector is an implementation of GroupFacetCollector that
// computes grouped facets based on the indexed terms from DocValues.
//
// Mirrors the public abstract class
// org.apache.lucene.search.grouping.TermGroupFacetCollector (Apache Lucene
// 10.5.0). The abstract class is rendered as this interface; its state lives
// in termGroupFacetCollector, embedded by the two package-private
// implementations SV and MV (termGroupFacetCollectorSV and
// termGroupFacetCollectorMV), which CreateTermGroupFacetCollector chooses
// between.
//
// lucene.experimental
type TermGroupFacetCollector interface {
	search.Collector
	search.LeafCollector

	// MergeSegmentResults renders the inherited
	// GroupFacetCollector.mergeSegmentResults(int, int, boolean).
	MergeSegmentResults(size, minCount int, orderByCount bool) (*GroupedFacetResult, error)
}

// CreateTermGroupFacetCollector is the factory method for creating the right
// implementation based on the fact whether the facet field contains multiple
// tokens per documents.
//
// groupField is the group field; facetField the facet field;
// facetFieldMultivalued whether the facet field has multiple tokens per
// document; facetPrefix the facet prefix a facet entry should start with to
// be included; initialSize the initial allocation size of the internal int
// set and group facet list, which should roughly match the total number of
// expected unique groups (be aware that the heap usage is 4 bytes *
// initialSize).
//
// Mirrors the static TermGroupFacetCollector.createTermGroupFacetCollector(
// String, String, boolean, BytesRef, int).
func CreateTermGroupFacetCollector(groupField, facetField string, facetFieldMultivalued bool,
	facetPrefix *util.BytesRef, initialSize int) TermGroupFacetCollector {
	if facetFieldMultivalued {
		return newTermGroupFacetCollectorMV(groupField, facetField, facetPrefix, initialSize)
	}
	return newTermGroupFacetCollectorSV(groupField, facetField, facetPrefix, initialSize)
}

// termGroupFacetCollector carries the state of the abstract class
// TermGroupFacetCollector.
type termGroupFacetCollector struct {
	*GroupFacetCollector

	groupedFacetHits        []groupedFacetHit
	segmentGroupedFacetHits *util.SentinelIntSet

	groupFieldTermsIndex index.SortedDocValues
}

// newTermGroupFacetCollector mirrors the package-private constructor
// TermGroupFacetCollector(String, String, BytesRef, int).
func newTermGroupFacetCollector(groupField, facetField string, facetPrefix *util.BytesRef,
	initialSize int) termGroupFacetCollector {
	return termGroupFacetCollector{
		GroupFacetCollector:     NewGroupFacetCollector(groupField, facetField, facetPrefix),
		groupedFacetHits:        make([]groupedFacetHit, 0, initialSize),
		segmentGroupedFacetHits: util.NewSentinelIntSet(initialSize, math.MinInt32),
	}
}

// groupedFacetHit renders the private record GroupedFacetHit(BytesRef
// groupValue, BytesRef facetValue).
type groupedFacetHit struct {
	groupValue *util.BytesRef
	facetValue *util.BytesRef
}

// segmentTermsEnum is the part of TermsEnum a TermGroupFacetCollector
// segment result drives: seekExact(long ord), term() and next().
type segmentTermsEnum interface {
	SeekExactOrd(ord int64) error
	Term() *spi.Term
	Next() (*spi.Term, error)
}

// termBytes renders the BytesRef a TermsEnum returns; a null term (the end of
// the enum) stays null.
func termBytes(term *spi.Term) *util.BytesRef {
	if term == nil {
		return nil
	}
	return term.Bytes
}

// facetEndPrefix renders
//
//	BytesRefBuilder facetEndPrefix = new BytesRefBuilder();
//	facetEndPrefix.append(facetPrefix);
//	facetEndPrefix.append(UnicodeUtil.BIG_TERM);
//	facetEndPrefix.get()
func facetEndPrefix(facetPrefix *util.BytesRef) *util.BytesRef {
	b := util.NewBytesRefBuilder()
	b.Append(facetPrefix)
	b.Append(util.BigTerm)
	return b.Get()
}

// deepCopyOrd renders BytesRef.deepCopyOf(values.lookupOrd(ord)).
func deepCopyOrd(lookup func(int) ([]byte, error), ord int) (*util.BytesRef, error) {
	b, err := lookup(ord)
	if err != nil {
		return nil, err
	}
	return util.BytesRefDeepCopyOf(util.NewBytesRef(b)), nil
}

// ---------------------------------------------------------------------------
// SV: implementation for single valued facet fields.
// ---------------------------------------------------------------------------

// termGroupFacetCollectorSV renders the package-private static class
// TermGroupFacetCollector.SV.
type termGroupFacetCollectorSV struct {
	termGroupFacetCollector

	facetFieldTermsIndex index.SortedDocValues
}

func newTermGroupFacetCollectorSV(groupField, facetField string, facetPrefix *util.BytesRef,
	initialSize int) *termGroupFacetCollectorSV {
	c := &termGroupFacetCollectorSV{
		termGroupFacetCollector: newTermGroupFacetCollector(groupField, facetField, facetPrefix, initialSize),
	}
	c.Outer = c
	c.CreateSegmentResult = c.createSegmentResult
	return c
}

// GetLeafCollector renders SimpleCollector.getLeafCollector: doSetNextReader,
// then return this.
func (c *termGroupFacetCollectorSV) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

// Collect renders SV.collect(int).
func (c *termGroupFacetCollectorSV) Collect(doc int) error {
	if doc > c.facetFieldTermsIndex.DocID() {
		if _, err := c.facetFieldTermsIndex.Advance(doc); err != nil {
			return err
		}
	}

	var facetOrd int
	if doc == c.facetFieldTermsIndex.DocID() {
		ord, err := c.facetFieldTermsIndex.OrdValue()
		if err != nil {
			return err
		}
		facetOrd = ord
	} else {
		facetOrd = -1
	}

	if facetOrd < c.startFacetOrd || facetOrd >= c.endFacetOrd {
		return nil
	}

	if doc > c.groupFieldTermsIndex.DocID() {
		if _, err := c.groupFieldTermsIndex.Advance(doc); err != nil {
			return err
		}
	}

	var groupOrd int
	if doc == c.groupFieldTermsIndex.DocID() {
		ord, err := c.groupFieldTermsIndex.OrdValue()
		if err != nil {
			return err
		}
		groupOrd = ord
	} else {
		groupOrd = -1
	}
	segmentGroupedFacetsIndex := groupOrd*(c.facetFieldTermsIndex.GetValueCount()+1) + facetOrd
	if c.segmentGroupedFacetHits.Exists(segmentGroupedFacetsIndex) {
		return nil
	}

	c.segmentTotalCount++
	c.segmentFacetCounts[facetOrd+1]++

	c.segmentGroupedFacetHits.Put(segmentGroupedFacetsIndex)

	var groupKey *util.BytesRef
	if groupOrd != -1 {
		k, err := deepCopyOrd(c.groupFieldTermsIndex.LookupOrd, groupOrd)
		if err != nil {
			return err
		}
		groupKey = k
	}

	var facetKey *util.BytesRef
	if facetOrd != -1 {
		k, err := deepCopyOrd(c.facetFieldTermsIndex.LookupOrd, facetOrd)
		if err != nil {
			return err
		}
		facetKey = k
	}

	c.groupedFacetHits = append(c.groupedFacetHits, groupedFacetHit{groupValue: groupKey, facetValue: facetKey})
	return nil
}

// DoSetNextReader renders SV.doSetNextReader(LeafReaderContext).
func (c *termGroupFacetCollectorSV) DoSetNextReader(context *index.LeafReaderContext) error {
	var err error
	c.groupFieldTermsIndex, err = index.GetSorted(context.LeafReader(), c.groupField)
	if err != nil {
		return err
	}
	c.facetFieldTermsIndex, err = index.GetSorted(context.LeafReader(), c.facetField)
	if err != nil {
		return err
	}

	// 1+ to allow for the -1 "not set":
	c.segmentFacetCounts = make([]int, c.facetFieldTermsIndex.GetValueCount()+1)
	c.segmentTotalCount = 0

	c.segmentGroupedFacetHits.Clear()
	for _, hit := range c.groupedFacetHits {
		facetOrd := -1
		if hit.facetValue != nil {
			facetOrd, err = index.LookupTerm(c.facetFieldTermsIndex, hit.facetValue)
			if err != nil {
				return err
			}
		}
		if hit.facetValue != nil && facetOrd < 0 {
			continue
		}

		groupOrd := -1
		if hit.groupValue != nil {
			groupOrd, err = index.LookupTerm(c.groupFieldTermsIndex, hit.groupValue)
			if err != nil {
				return err
			}
		}
		if hit.groupValue != nil && groupOrd < 0 {
			continue
		}

		segmentGroupedFacetsIndex := groupOrd*(c.facetFieldTermsIndex.GetValueCount()+1) + facetOrd
		c.segmentGroupedFacetHits.Put(segmentGroupedFacetsIndex)
	}

	if c.facetPrefix != nil {
		c.startFacetOrd, err = index.LookupTerm(c.facetFieldTermsIndex, c.facetPrefix)
		if err != nil {
			return err
		}
		if c.startFacetOrd < 0 {
			// Points to the ord one higher than facetPrefix
			c.startFacetOrd = -c.startFacetOrd - 1
		}
		c.endFacetOrd, err = index.LookupTerm(c.facetFieldTermsIndex, facetEndPrefix(c.facetPrefix))
		if err != nil {
			return err
		}
		if util.AssertsEnabled() && c.endFacetOrd >= 0 {
			return util.NewAssertionError("endFacetOrd < 0")
		}
		c.endFacetOrd = -c.endFacetOrd - 1 // Points to the ord one higher than facetEndPrefix
	} else {
		c.startFacetOrd = -1
		c.endFacetOrd = c.facetFieldTermsIndex.GetValueCount()
	}
	return nil
}

// CollectRange renders the default LeafCollector.collectRange(int, int).
func (c *termGroupFacetCollectorSV) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream renders the default LeafCollector.collect(DocIdStream).
func (c *termGroupFacetCollectorSV) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// createSegmentResult renders SV.createSegmentResult().
func (c *termGroupFacetCollectorSV) createSegmentResult() (SegmentResult, error) {
	return newSVSegmentResult(c.segmentFacetCounts, c.segmentTotalCount,
		index.NewSortedDocValuesTermsEnum(c.facetField, c.facetFieldTermsIndex), c.startFacetOrd, c.endFacetOrd)
}

// svSegmentResult renders the private static class SV.SegmentResult.
type svSegmentResult struct {
	BaseSegmentResult
	tenum *index.SortedDocValuesTermsEnum
}

func newSVSegmentResult(counts []int, total int, tenum *index.SortedDocValuesTermsEnum,
	startFacetOrd, endFacetOrd int) (*svSegmentResult, error) {
	r := &svSegmentResult{
		BaseSegmentResult: NewBaseSegmentResult(counts, total-counts[0], counts[0], endFacetOrd+1),
		tenum:             tenum,
	}
	if startFacetOrd == -1 {
		r.mergePos = 1
	} else {
		r.mergePos = startFacetOrd + 1
	}
	if r.mergePos < r.maxTermPos {
		if util.AssertsEnabled() && tenum == nil {
			return nil, util.NewAssertionError("tenum != null")
		}
		ord := startFacetOrd
		if startFacetOrd == -1 {
			ord = 0
		}
		if err := tenum.SeekExactOrd(ord); err != nil {
			return nil, err
		}
		r.mergeTerm = termBytes(tenum.Term())
	}
	return r, nil
}

// NextTerm renders SV.SegmentResult.nextTerm().
func (r *svSegmentResult) NextTerm() error {
	term, err := r.tenum.Next()
	if err != nil {
		return err
	}
	r.mergeTerm = termBytes(term)
	return nil
}

// ---------------------------------------------------------------------------
// MV: implementation for multi valued facet fields.
// ---------------------------------------------------------------------------

// termGroupFacetCollectorMV renders the package-private static class
// TermGroupFacetCollector.MV.
type termGroupFacetCollectorMV struct {
	termGroupFacetCollector

	facetFieldDocTermOrds index.SortedSetDocValues
	facetOrdTermsEnum     *index.SortedSetDocValuesTermsEnum
	facetFieldNumTerms    int
}

func newTermGroupFacetCollectorMV(groupField, facetField string, facetPrefix *util.BytesRef,
	initialSize int) *termGroupFacetCollectorMV {
	c := &termGroupFacetCollectorMV{
		termGroupFacetCollector: newTermGroupFacetCollector(groupField, facetField, facetPrefix, initialSize),
	}
	c.Outer = c
	c.CreateSegmentResult = c.createSegmentResult
	return c
}

// GetLeafCollector renders SimpleCollector.getLeafCollector: doSetNextReader,
// then return this.
func (c *termGroupFacetCollectorMV) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

// Collect renders MV.collect(int).
func (c *termGroupFacetCollectorMV) Collect(doc int) error {
	if doc > c.groupFieldTermsIndex.DocID() {
		if _, err := c.groupFieldTermsIndex.Advance(doc); err != nil {
			return err
		}
	}

	var groupOrd int
	if doc == c.groupFieldTermsIndex.DocID() {
		ord, err := c.groupFieldTermsIndex.OrdValue()
		if err != nil {
			return err
		}
		groupOrd = ord
	} else {
		groupOrd = -1
	}

	if c.facetFieldNumTerms == 0 {
		segmentGroupedFacetsIndex := groupOrd * (c.facetFieldNumTerms + 1)
		if c.facetPrefix != nil || c.segmentGroupedFacetHits.Exists(segmentGroupedFacetsIndex) {
			return nil
		}

		c.segmentTotalCount++
		c.segmentFacetCounts[c.facetFieldNumTerms]++

		c.segmentGroupedFacetHits.Put(segmentGroupedFacetsIndex)
		var groupKey *util.BytesRef
		if groupOrd != -1 {
			k, err := deepCopyOrd(c.groupFieldTermsIndex.LookupOrd, groupOrd)
			if err != nil {
				return err
			}
			groupKey = k
		}
		c.groupedFacetHits = append(c.groupedFacetHits, groupedFacetHit{groupValue: groupKey})
		return nil
	}

	if doc > c.facetFieldDocTermOrds.DocID() {
		if _, err := c.facetFieldDocTermOrds.Advance(doc); err != nil {
			return err
		}
	}
	empty := true
	if doc == c.facetFieldDocTermOrds.DocID() {
		for i := 0; i < c.facetFieldDocTermOrds.DocValueCount(); i++ {
			ord, err := c.facetFieldDocTermOrds.NextOrd()
			if err != nil {
				return err
			}
			if err := c.process(groupOrd, ord); err != nil {
				return err
			}
			empty = false
		}
	}

	if empty {
		// this facet ord is reserved for docs not containing facet field.
		return c.process(groupOrd, c.facetFieldNumTerms)
	}
	return nil
}

// process renders the private MV.process(int, int).
func (c *termGroupFacetCollectorMV) process(groupOrd, facetOrd int) error {
	if facetOrd < c.startFacetOrd || facetOrd >= c.endFacetOrd {
		return nil
	}

	segmentGroupedFacetsIndex := groupOrd*(c.facetFieldNumTerms+1) + facetOrd
	if c.segmentGroupedFacetHits.Exists(segmentGroupedFacetsIndex) {
		return nil
	}

	c.segmentTotalCount++
	c.segmentFacetCounts[facetOrd]++

	c.segmentGroupedFacetHits.Put(segmentGroupedFacetsIndex)

	var groupKey *util.BytesRef
	if groupOrd != -1 {
		k, err := deepCopyOrd(c.groupFieldTermsIndex.LookupOrd, groupOrd)
		if err != nil {
			return err
		}
		groupKey = k
	}

	var facetValue *util.BytesRef
	if facetOrd != c.facetFieldNumTerms {
		k, err := deepCopyOrd(c.facetFieldDocTermOrds.LookupOrd, facetOrd)
		if err != nil {
			return err
		}
		facetValue = k
	}
	c.groupedFacetHits = append(c.groupedFacetHits, groupedFacetHit{groupValue: groupKey, facetValue: facetValue})
	return nil
}

// DoSetNextReader renders MV.doSetNextReader(LeafReaderContext).
func (c *termGroupFacetCollectorMV) DoSetNextReader(context *index.LeafReaderContext) error {
	var err error
	c.groupFieldTermsIndex, err = index.GetSorted(context.LeafReader(), c.groupField)
	if err != nil {
		return err
	}
	c.facetFieldDocTermOrds, err = index.GetSortedSet(context.LeafReader(), c.facetField)
	if err != nil {
		return err
	}
	c.facetFieldNumTerms = c.facetFieldDocTermOrds.GetValueCount()
	if c.facetFieldNumTerms == 0 {
		c.facetOrdTermsEnum = nil
	} else {
		c.facetOrdTermsEnum = index.NewSortedSetDocValuesTermsEnum(c.facetField, c.facetFieldDocTermOrds)
	}
	// [facetFieldNumTerms() + 1] for all possible facet values and docs not containing facet
	// field
	c.segmentFacetCounts = make([]int, c.facetFieldNumTerms+1)
	c.segmentTotalCount = 0

	c.segmentGroupedFacetHits.Clear()
	for _, hit := range c.groupedFacetHits {
		groupOrd := -1
		if hit.groupValue != nil {
			groupOrd, err = index.LookupTerm(c.groupFieldTermsIndex, hit.groupValue)
			if err != nil {
				return err
			}
		}
		if hit.groupValue != nil && groupOrd < 0 {
			continue
		}

		var facetOrd int
		if hit.facetValue != nil {
			if c.facetOrdTermsEnum == nil {
				continue
			}
			found, err := c.facetOrdTermsEnum.SeekExact(index.NewTermFromBytesRef(c.facetField, hit.facetValue))
			if err != nil {
				return err
			}
			if !found {
				continue
			}
			facetOrd = int(c.facetOrdTermsEnum.Ord())
		} else {
			facetOrd = c.facetFieldNumTerms
		}

		// (facetFieldDocTermOrds.numTerms() + 1) for all possible facet values and docs not
		// containing facet field
		segmentGroupedFacetsIndex := groupOrd*(c.facetFieldNumTerms+1) + facetOrd
		c.segmentGroupedFacetHits.Put(segmentGroupedFacetsIndex)
	}

	if c.facetPrefix != nil {
		// A null term from seekCeil renders TermsEnum.SeekStatus.END.
		end := true
		if c.facetOrdTermsEnum != nil {
			term, err := c.facetOrdTermsEnum.SeekCeil(index.NewTermFromBytesRef(c.facetField, c.facetPrefix))
			if err != nil {
				return err
			}
			end = term == nil
		}

		if !end {
			c.startFacetOrd = int(c.facetOrdTermsEnum.Ord())
		} else {
			c.startFacetOrd = 0
			c.endFacetOrd = 0
			return nil
		}

		term, err := c.facetOrdTermsEnum.SeekCeil(index.NewTermFromBytesRef(c.facetField, facetEndPrefix(c.facetPrefix)))
		if err != nil {
			return err
		}
		if term != nil {
			c.endFacetOrd = int(c.facetOrdTermsEnum.Ord())
		} else {
			c.endFacetOrd = c.facetFieldNumTerms // Don't include null...
		}
	} else {
		c.startFacetOrd = 0
		c.endFacetOrd = c.facetFieldNumTerms + 1
	}
	return nil
}

// CollectRange renders the default LeafCollector.collectRange(int, int).
func (c *termGroupFacetCollectorMV) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream renders the default LeafCollector.collect(DocIdStream).
func (c *termGroupFacetCollectorMV) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// createSegmentResult renders MV.createSegmentResult().
func (c *termGroupFacetCollectorMV) createSegmentResult() (SegmentResult, error) {
	var tenum segmentTermsEnum
	if c.facetOrdTermsEnum != nil {
		tenum = c.facetOrdTermsEnum
	}
	return newMVSegmentResult(c.segmentFacetCounts, c.segmentTotalCount, c.facetFieldNumTerms,
		tenum, c.startFacetOrd, c.endFacetOrd)
}

// mvSegmentResult renders the private static class MV.SegmentResult.
type mvSegmentResult struct {
	BaseSegmentResult
	tenum segmentTermsEnum
}

func newMVSegmentResult(counts []int, total, missingCountIndex int, tenum segmentTermsEnum,
	startFacetOrd, endFacetOrd int) (*mvSegmentResult, error) {
	maxTermPos := endFacetOrd
	if endFacetOrd == missingCountIndex+1 {
		maxTermPos = missingCountIndex
	}
	r := &mvSegmentResult{
		BaseSegmentResult: NewBaseSegmentResult(counts, total-counts[missingCountIndex],
			counts[missingCountIndex], maxTermPos),
		tenum: tenum,
	}
	r.mergePos = startFacetOrd
	if tenum != nil {
		if err := tenum.SeekExactOrd(int64(r.mergePos)); err != nil {
			return nil, err
		}
		r.mergeTerm = termBytes(tenum.Term())
	}
	return r, nil
}

// NextTerm renders MV.SegmentResult.nextTerm().
func (r *mvSegmentResult) NextTerm() error {
	term, err := r.tenum.Next()
	if err != nil {
		return err
	}
	r.mergeTerm = termBytes(term)
	return nil
}
