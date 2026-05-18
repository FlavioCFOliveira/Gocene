package suggest

// UnsortedInputIterator is an alias for BufferedInputIterator that
// emphasises the "no sort" guarantee. Mirrors
// org.apache.lucene.search.suggest.UnsortedInputIterator.
type UnsortedInputIterator struct{ *BufferedInputIterator }

// NewUnsortedInputIterator buffers inner without changing order.
func NewUnsortedInputIterator(inner InputIterator) (*UnsortedInputIterator, error) {
	buf, err := NewBufferedInputIterator(inner)
	if err != nil {
		return nil, err
	}
	return &UnsortedInputIterator{BufferedInputIterator: buf}, nil
}

var _ InputIterator = (*UnsortedInputIterator)(nil)
