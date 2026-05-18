// Package uhighlight implements org.apache.lucene.search.uhighlight: the
// "unified" highlighter that consumes per-document offsets from postings,
// term vectors, or re-analysis on demand.
package uhighlight

// OffsetsEnum is the iteration primitive every uhighlight FieldOffsetStrategy
// produces: a stream of (term, startOffset, endOffset, weight) tuples in
// document order. Mirrors org.apache.lucene.search.uhighlight.OffsetsEnum.
type OffsetsEnum interface {
	// Next advances to the next offset; false at end-of-stream.
	Next() bool

	// Term returns the term text for the current offset.
	Term() string

	// StartOffset returns the start character offset.
	StartOffset() int

	// EndOffset returns the end (exclusive) character offset.
	EndOffset() int

	// Weight returns the highlighter weight for the current offset.
	Weight() float32

	// FreqIndex returns the per-term occurrence counter for the current
	// position (matches the Java FreqIndex contract).
	FreqIndex() int

	// Close releases resources held by the enum.
	Close() error
}

// SliceOffsetsEnum is a trivial OffsetsEnum backed by an in-memory slice.
type SliceOffsetsEnum struct {
	entries []OffsetEntry
	idx     int
}

// OffsetEntry is a single (term, start, end, weight) tuple.
type OffsetEntry struct {
	Term         string
	StartOffset  int
	EndOffset    int
	Weight       float32
}

// NewSliceOffsetsEnum builds an enum backed by entries.
func NewSliceOffsetsEnum(entries []OffsetEntry) *SliceOffsetsEnum {
	clone := make([]OffsetEntry, len(entries))
	copy(clone, entries)
	return &SliceOffsetsEnum{entries: clone, idx: -1}
}

// Next advances to the next entry.
func (e *SliceOffsetsEnum) Next() bool { e.idx++; return e.idx < len(e.entries) }

// Term returns the current term.
func (e *SliceOffsetsEnum) Term() string {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return ""
	}
	return e.entries[e.idx].Term
}

// StartOffset returns the current start offset.
func (e *SliceOffsetsEnum) StartOffset() int {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return -1
	}
	return e.entries[e.idx].StartOffset
}

// EndOffset returns the current end offset.
func (e *SliceOffsetsEnum) EndOffset() int {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return -1
	}
	return e.entries[e.idx].EndOffset
}

// Weight returns the current weight.
func (e *SliceOffsetsEnum) Weight() float32 {
	if e.idx < 0 || e.idx >= len(e.entries) {
		return 0
	}
	return e.entries[e.idx].Weight
}

// FreqIndex returns the current index within the entry list.
func (e *SliceOffsetsEnum) FreqIndex() int { return e.idx }

// Close is a no-op.
func (e *SliceOffsetsEnum) Close() error { return nil }
