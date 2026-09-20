package index

// FilterBinaryDocValues delegates all methods to a wrapped BinaryDocValues.
// This is the Go port of Lucene's org.apache.lucene.index.FilterBinaryDocValues.
type FilterBinaryDocValues struct {
	in BinaryDocValues
}

// NewFilterBinaryDocValues creates a new FilterBinaryDocValues wrapping the given BinaryDocValues.
func NewFilterBinaryDocValues(in BinaryDocValues) *FilterBinaryDocValues {
	return &FilterBinaryDocValues{in: in}
}

// DocID returns the current doc ID.
func (f *FilterBinaryDocValues) DocID() int {
	return f.in.DocID()
}

// NextDoc advances to the next doc ID and returns it.
func (f *FilterBinaryDocValues) NextDoc() (int, error) {
	return f.in.NextDoc()
}

// Advance advances the iterator to the next doc ID on or after target.
func (f *FilterBinaryDocValues) Advance(target int) (int, error) {
	return f.in.Advance(target)
}

// AdvanceExact advances the iterator to exactly target and returns whether target has a value.
func (f *FilterBinaryDocValues) AdvanceExact(target int) (bool, error) {
	return f.in.AdvanceExact(target)
}

// Cost returns the cost of using this iterator.
func (f *FilterBinaryDocValues) Cost() int64 {
	return f.in.Cost()
}

// BinaryValue returns the binary value for the current document ID.
func (f *FilterBinaryDocValues) BinaryValue() ([]byte, error) {
	return f.in.BinaryValue()
}
