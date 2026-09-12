package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterSortedDocValues is a wrapper for SortedDocValues that filters documents
// using a DocIdSet.
//
// This is the Go port of org.apache.lucene.index.FilterSortedDocValues.
type FilterSortedDocValues struct {
	in       SortedDocValues
	filter   util.DocIdSet
	filterIt util.DocIdSetIterator
}

// NewFilterSortedDocValues creates a new FilterSortedDocValues wrapper.
func NewFilterSortedDocValues(in SortedDocValues, filter util.DocIdSet) *FilterSortedDocValues {
	var it util.DocIdSetIterator
	if filter != nil {
		it = filter.Iterator()
	}
	return &FilterSortedDocValues{
		in:       in,
		filter:   filter,
		filterIt: it,
	}
}

// DocID returns the current document ID.
func (f *FilterSortedDocValues) DocID() int {
	return f.in.DocID()
}

// NextDoc advances to the next document that is both present in the wrapped
// SortedDocValues and present in the filter DocIdSet.
func (f *FilterSortedDocValues) NextDoc() (int, error) {
	for {
		doc, err := f.in.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == util.NO_MORE_DOCS {
			return util.NO_MORE_DOCS, nil
		}
		if f.filter == nil {
			return doc, nil
		}
		next, err := f.filterIt.Advance(doc)
		if err != nil {
			return 0, err
		}
		if next == doc {
			return doc, nil
		}
		// The document was filtered out; the wrapped iterator is already advanced,
		// so we loop and call NextDoc again.
	}
}

// Advance advances the iterator to the first document at or after target that is both
// present in the wrapped SortedDocValues and present in the filter DocIdSet.
func (f *FilterSortedDocValues) Advance(target int) (int, error) {
	for {
		doc, err := f.in.Advance(target)
		if err != nil {
			return 0, err
		}
		if doc == util.NO_MORE_DOCS {
			return util.NO_MORE_DOCS, nil
		}
		if f.filter == nil {
			return doc, nil
		}
		next, err := f.filterIt.Advance(doc)
		if err != nil {
			return 0, err
		}
		if next == doc {
			return doc, nil
		}
		// The document was filtered out; update target to the next possible
		// candidate in the filter and try again.
		target = next
	}
}

// DocIDRunEnd returns the exclusive end of the current run of consecutive doc
// IDs. Mirrors org.apache.lucene.search.DocIdSetIterator#docIDRunEnd: the
// delegate's override is used when it declares one, otherwise the default
// implementation "runs of a single doc ID" applies and docID() + 1 is returned.
func (f *FilterSortedDocValues) DocIDRunEnd() int {
	if runner, ok := f.in.(interface{ DocIDRunEnd() int }); ok {
		return runner.DocIDRunEnd()
	}
	return f.in.DocID() + 1
}

// Cost returns the estimated cost of the iterator.
func (f *FilterSortedDocValues) Cost() int64 {
	return f.in.Cost()
}

// AdvanceExact advances the iterator to exactly target and returns whether target has a value.
func (f *FilterSortedDocValues) AdvanceExact(target int) (bool, error) {
	hasValue, err := f.in.AdvanceExact(target)
	if err != nil {
		return false, err
	}
	if !hasValue {
		return false, nil
	}
	if f.filter == nil {
		return true, nil
	}
	next, err := f.filterIt.Advance(target)
	if err != nil {
		return false, err
	}
	return next == target, nil
}

// OrdValue returns the ordinal for the current docID.
func (f *FilterSortedDocValues) OrdValue() (int, error) {
	return f.in.OrdValue()
}

// LookupOrd retrieves the value for the specified ordinal.
func (f *FilterSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	return f.in.LookupOrd(ord)
}

// GetValueCount returns the number of unique values.
func (f *FilterSortedDocValues) GetValueCount() int {
	return f.in.GetValueCount()
}
