package index

import "github.com/FlavioCFOliveira/Gocene/util"

// FilterNumericDocValues delegates all methods to a wrapped NumericDocValues.
// This is the Go port of Lucene's org.apache.lucene.index.FilterNumericDocValues.
type FilterNumericDocValues struct {
	in NumericDocValues
}

// NewFilterNumericDocValues creates a new FilterNumericDocValues wrapper.
func NewFilterNumericDocValues(in NumericDocValues) *FilterNumericDocValues {
	if in == nil {
		panic("FilterNumericDocValues: wrapped NumericDocValues cannot be nil")
	}
	return &FilterNumericDocValues{in: in}
}

// DocID returns the current document ID.
func (f *FilterNumericDocValues) DocID() int {
	return f.in.DocID()
}

// NextDoc advances to the next document that has a value and returns its ID.
func (f *FilterNumericDocValues) NextDoc() (int, error) {
	return f.in.NextDoc()
}

// Advance positions the iterator on the first document with a value whose ID is >= target.
func (f *FilterNumericDocValues) Advance(target int) (int, error) {
	return f.in.Advance(target)
}

// AdvanceExact positions the iterator on the given target document and returns true if it has a value.
func (f *FilterNumericDocValues) AdvanceExact(target int) (bool, error) {
	return f.in.AdvanceExact(target)
}

// LongValue returns the numeric value for the current document.
func (f *FilterNumericDocValues) LongValue() (int64, error) {
	return f.in.LongValue()
}

// Cost returns an estimate of the cost of iterating over the entire value-bearing document set.
func (f *FilterNumericDocValues) Cost() int64 {
	return f.in.Cost()
}

// IntoBitSet loads doc IDs into a FixedBitSet, shifted down by offset.
//
// Java's FilterNumericDocValues does not override intoBitSet: it inherits the
// DocIdSetIterator default, which this reproduces.
func (f *FilterNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(f, upTo, bitSet, offset)
}

// DocIDRunEnd returns one plus the last doc ID of the run of consecutive
// matching doc IDs containing DocID().
//
// Java's FilterNumericDocValues does not override docIDRunEnd: it inherits the
// DocIdSetIterator default, which this reproduces.
func (f *FilterNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(f)
}
