// Package iterators implements org.apache.lucene.sandbox.facet.iterators.
package iterators

// OrdinalIterator is the foundational ordinal iterator. Mirrors
// org.apache.lucene.sandbox.facet.iterators.OrdinalIterator.
type OrdinalIterator interface {
	Next() (int, bool)
}

// ComparableSupplier resolves the comparable that orders ordinals.
type ComparableSupplier[T any] interface {
	GetComparable(ord int) T
}

// LengthOrdinalIterator returns the first Length ordinals in sequence.
type LengthOrdinalIterator struct {
	Length int
	pos    int
}

// NewLengthOrdinalIterator builds the iterator.
func NewLengthOrdinalIterator(length int) *LengthOrdinalIterator {
	return &LengthOrdinalIterator{Length: length}
}

// Next advances.
func (it *LengthOrdinalIterator) Next() (int, bool) {
	if it.pos >= it.Length {
		return 0, false
	}
	v := it.pos
	it.pos++
	return v, true
}

// TopnOrdinalIterator surfaces the top-N ordinals from a wrapped iterator.
type TopnOrdinalIterator struct {
	Inner OrdinalIterator
	N     int
	count int
}

// NewTopnOrdinalIterator builds the iterator.
func NewTopnOrdinalIterator(inner OrdinalIterator, n int) *TopnOrdinalIterator {
	if n < 1 {
		n = 1
	}
	return &TopnOrdinalIterator{Inner: inner, N: n}
}

// Next advances at most N times.
func (it *TopnOrdinalIterator) Next() (int, bool) {
	if it.count >= it.N {
		return 0, false
	}
	v, ok := it.Inner.Next()
	if !ok {
		return 0, false
	}
	it.count++
	return v, true
}

// CandidateSetOrdinalIterator iterates only over the supplied candidate set.
type CandidateSetOrdinalIterator struct {
	Candidates []int
	pos        int
}

// NewCandidateSetOrdinalIterator builds the iterator.
func NewCandidateSetOrdinalIterator(candidates []int) *CandidateSetOrdinalIterator {
	return &CandidateSetOrdinalIterator{Candidates: append([]int(nil), candidates...)}
}

// Next returns the next candidate.
func (it *CandidateSetOrdinalIterator) Next() (int, bool) {
	if it.pos >= len(it.Candidates) {
		return 0, false
	}
	v := it.Candidates[it.pos]
	it.pos++
	return v, true
}

// TaxonomyChildrenOrdinalIterator walks the children of a taxonomy node.
type TaxonomyChildrenOrdinalIterator struct {
	ChildrenFn func(parent int) []int
	parent     int
	emitted    int
	cache      []int
}

// NewTaxonomyChildrenOrdinalIterator builds the iterator.
func NewTaxonomyChildrenOrdinalIterator(parent int, fn func(parent int) []int) *TaxonomyChildrenOrdinalIterator {
	return &TaxonomyChildrenOrdinalIterator{parent: parent, ChildrenFn: fn}
}

// Next returns the next child ordinal.
func (it *TaxonomyChildrenOrdinalIterator) Next() (int, bool) {
	if it.cache == nil && it.ChildrenFn != nil {
		it.cache = it.ChildrenFn(it.parent)
	}
	if it.emitted >= len(it.cache) {
		return 0, false
	}
	v := it.cache[it.emitted]
	it.emitted++
	return v, true
}
