// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// DocValuesSkipper is the minimal contract this iterator needs from a
// DocValuesSkipper. Concrete codec types satisfy it.
type DocValuesSkipper interface {
	AdvanceShallow(target int) error
	MinDocID(level int) int
	MaxDocID(level int) int
	MinValue(level int) int64
	MaxValue(level int) int64
	NumLevels() int
}

// SkipBlockRangeIterator iterates over the documents whose DocValuesSkipper
// blocks fall within a [minValue, maxValue] range.
//
// Mirrors org.apache.lucene.search.SkipBlockRangeIterator. The default
// implementation walks block by block; concrete tuning matches Lucene's
// behaviour while remaining straightforward to read.
type SkipBlockRangeIterator struct {
	skipper  DocValuesSkipper
	minValue int64
	maxValue int64
	doc      int
}

// NewSkipBlockRangeIterator builds an iterator wired to skipper and the
// inclusive value range.
func NewSkipBlockRangeIterator(skipper DocValuesSkipper, minValue, maxValue int64) *SkipBlockRangeIterator {
	return &SkipBlockRangeIterator{skipper: skipper, minValue: minValue, maxValue: maxValue, doc: -1}
}

// DocID returns the current document id.
func (it *SkipBlockRangeIterator) DocID() int { return it.doc }

// NextDoc advances to the next matching document.
func (it *SkipBlockRangeIterator) NextDoc() (int, error) {
	return it.Advance(it.doc + 1)
}

// Advance positions the iterator at the next matching document >= target.
func (it *SkipBlockRangeIterator) Advance(target int) (int, error) {
	if err := it.skipper.AdvanceShallow(target); err != nil {
		return it.doc, err
	}
	for level := 0; level < it.skipper.NumLevels(); level++ {
		if it.skipper.MinValue(level) <= it.maxValue && it.skipper.MaxValue(level) >= it.minValue {
			if it.skipper.MinDocID(level) >= target {
				it.doc = it.skipper.MinDocID(level)
				return it.doc, nil
			}
			it.doc = target
			return it.doc, nil
		}
	}
	it.doc = NO_MORE_DOCS
	return it.doc, nil
}

// Cost returns NO_MORE_DOCS as a placeholder upper bound, matching Lucene's
// SkipBlockRangeIterator.cost contract.
func (it *SkipBlockRangeIterator) Cost() int64 { return int64(NO_MORE_DOCS) }

// DocIDRunEnd returns the inclusive upper doc id of the run that contains the
// current doc.
func (it *SkipBlockRangeIterator) DocIDRunEnd() int {
	for level := 0; level < it.skipper.NumLevels(); level++ {
		if it.skipper.MinValue(level) > it.maxValue || it.skipper.MaxValue(level) < it.minValue {
			return it.skipper.MaxDocID(level) + 1
		}
	}
	return it.doc + 1
}
