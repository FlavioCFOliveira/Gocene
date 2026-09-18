// Package uhighlight implements org.apache.lucene.search.uhighlight: the
// "unified" highlighter that consumes per-document offsets from postings,
// term vectors, or re-analysis on demand.
package uhighlight

// OffsetsEnum is a view over the positionally-sorted matching spans of a
// document.
//
// This is the Go port of the abstract class
// org.apache.lucene.search.uhighlight.OffsetsEnum from Apache Lucene 10.5.0;
// its member set is that class's abstract methods plus close(). Every Java
// member declares `throws IOException`, which Gocene renders as a trailing
// error result.
type OffsetsEnum interface {
	// NextPosition advances to the next position, returning false when the
	// enum is exhausted. Mirrors OffsetsEnum.nextPosition().
	NextPosition() (bool, error)

	// Freq returns the term frequency of the current term in the document.
	// Mirrors OffsetsEnum.freq().
	Freq() (int, error)

	// GetTerm returns the term at the current position. Mirrors
	// OffsetsEnum.getTerm().
	GetTerm() ([]byte, error)

	// StartOffset returns the start character offset of the current position.
	// Mirrors OffsetsEnum.startOffset().
	StartOffset() (int, error)

	// EndOffset returns the end (exclusive) character offset of the current
	// position. Mirrors OffsetsEnum.endOffset().
	EndOffset() (int, error)

	// Close releases resources held by the enum. Mirrors
	// OffsetsEnum.close(), whose body in Java is empty.
	Close() error
}

// SliceOffsetsEnum is a trivial OffsetsEnum backed by an in-memory slice.
type SliceOffsetsEnum struct {
	entries []OffsetEntry
	idx     int
}

// OffsetEntry is a single (term, start, end, freq) tuple.
type OffsetEntry struct {
	Term        string
	StartOffset int
	EndOffset   int
	Freq        int
}

// NewSliceOffsetsEnum builds an enum backed by entries.
func NewSliceOffsetsEnum(entries []OffsetEntry) *SliceOffsetsEnum {
	clone := make([]OffsetEntry, len(entries))
	copy(clone, entries)
	return &SliceOffsetsEnum{entries: clone, idx: -1}
}

// NextPosition advances to the next entry.
func (e *SliceOffsetsEnum) NextPosition() (bool, error) {
	e.idx++
	return e.idx < len(e.entries), nil
}

// GetTerm returns the current term.
func (e *SliceOffsetsEnum) GetTerm() ([]byte, error) {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return nil, nil
	}
	return []byte(e.entries[e.idx].Term), nil
}

// StartOffset returns the current start offset.
func (e *SliceOffsetsEnum) StartOffset() (int, error) {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return -1, nil
	}
	return e.entries[e.idx].StartOffset, nil
}

// EndOffset returns the current end offset.
func (e *SliceOffsetsEnum) EndOffset() (int, error) {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return -1, nil
	}
	return e.entries[e.idx].EndOffset, nil
}

// Freq returns the term frequency recorded for the current entry.
func (e *SliceOffsetsEnum) Freq() (int, error) {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return 0, nil
	}
	return e.entries[e.idx].Freq, nil
}

// Close is a no-op.
func (e *SliceOffsetsEnum) Close() error { return nil }

var _ OffsetsEnum = (*SliceOffsetsEnum)(nil)
