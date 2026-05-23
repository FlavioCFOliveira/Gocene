package suggest

import "math/rand"

// UnsortedInputIterator buffers the incoming elements and re-emits them in a
// random (shuffled) order. Mirrors
// org.apache.lucene.search.suggest.UnsortedInputIterator.
//
// The Java original uses a Fisher-Yates shuffle over an int[] ordinal array
// so that the underlying BytesRefArray storage is never reallocated; this
// port keeps the same strategy by shuffling an index slice over the
// BufferedInputIterator storage.
type UnsortedInputIterator struct {
	buf  *BufferedInputIterator
	ords []int
	pos  int
}

// NewUnsortedInputIterator buffers all tuples from inner and shuffles their
// order using a random permutation, matching the Java contract.
func NewUnsortedInputIterator(inner InputIterator) (*UnsortedInputIterator, error) {
	buf, err := NewBufferedInputIterator(inner)
	if err != nil {
		return nil, err
	}
	n := buf.Count()
	ords := make([]int, n)
	for i := range ords {
		ords[i] = i
	}
	// Fisher-Yates shuffle — mirrors UnsortedInputIterator constructor in Java.
	rng := rand.New(rand.NewSource(rand.Int63())) //nolint:gosec // non-cryptographic shuffle
	for i := 0; i < n; i++ {
		j := rng.Intn(n)
		ords[i], ords[j] = ords[j], ords[i]
	}
	return &UnsortedInputIterator{buf: buf, ords: ords, pos: -1}, nil
}

// Next advances the iterator and returns the entry at the next shuffled
// ordinal position.
func (u *UnsortedInputIterator) Next() (term []byte, weight int64, payload []byte, contexts [][]byte, ok bool, err error) {
	u.pos++
	if u.pos >= len(u.ords) {
		return nil, 0, nil, nil, false, nil
	}
	ord := u.ords[u.pos]
	t, w, p, c := u.buf.At(ord)
	return t, w, p, c, true, nil
}

// HasPayloads mirrors the wrapped iterator.
func (u *UnsortedInputIterator) HasPayloads() bool { return u.buf.HasPayloads() }

// HasContexts mirrors the wrapped iterator.
func (u *UnsortedInputIterator) HasContexts() bool { return u.buf.HasContexts() }

var _ InputIterator = (*UnsortedInputIterator)(nil)
