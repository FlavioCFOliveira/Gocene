package suggest

// BufferedInputIterator captures every tuple emitted by a wrapped iterator
// so it can be replayed many times. Mirrors
// org.apache.lucene.search.suggest.BufferedInputIterator.
type BufferedInputIterator struct {
	terms       [][]byte
	weights     []int64
	payloads    [][]byte
	contexts    [][][]byte
	idx         int
	hasPayload  bool
	hasContext  bool
}

// NewBufferedInputIterator buffers every tuple from inner.
func NewBufferedInputIterator(inner InputIterator) (*BufferedInputIterator, error) {
	out := &BufferedInputIterator{
		hasPayload: inner.HasPayloads(),
		hasContext: inner.HasContexts(),
	}
	for {
		term, w, p, c, ok, err := inner.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		out.terms = append(out.terms, append([]byte(nil), term...))
		out.weights = append(out.weights, w)
		out.payloads = append(out.payloads, append([]byte(nil), p...))
		ctxClone := make([][]byte, len(c))
		for i, b := range c {
			ctxClone[i] = append([]byte(nil), b...)
		}
		out.contexts = append(out.contexts, ctxClone)
	}
	out.idx = -1
	return out, nil
}

// Next advances the cursor.
func (b *BufferedInputIterator) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	b.idx++
	if b.idx >= len(b.terms) {
		return nil, 0, nil, nil, false, nil
	}
	return b.terms[b.idx], b.weights[b.idx], b.payloads[b.idx], b.contexts[b.idx], true, nil
}

// HasPayloads mirrors the wrapped iterator.
func (b *BufferedInputIterator) HasPayloads() bool { return b.hasPayload }

// HasContexts mirrors the wrapped iterator.
func (b *BufferedInputIterator) HasContexts() bool { return b.hasContext }

// Reset rewinds the cursor so the buffered tuples can be replayed.
func (b *BufferedInputIterator) Reset() { b.idx = -1 }

// Count returns the number of buffered entries.
func (b *BufferedInputIterator) Count() int { return len(b.terms) }

// At returns the term, weight, payload, and contexts at ordinal position i.
// The caller must ensure i is within [0, Count()).
func (b *BufferedInputIterator) At(i int) (term []byte, weight int64, payload []byte, contexts [][]byte) {
	return b.terms[i], b.weights[i], b.payloads[i], b.contexts[i]
}

var _ InputIterator = (*BufferedInputIterator)(nil)
