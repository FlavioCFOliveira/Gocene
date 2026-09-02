package column

import "github.com/FlavioCFOliveira/Gocene/util"

// LongTupleCursor is a tuple cursor over a LongColumn.
// It yields (docID, longValue) pairs.
// Batch-local doc-ids are returned in non-decreasing order; the same doc-id may repeat
// for multi-valued fields.
type LongTupleCursor interface {
	// NextDoc advances to the next tuple and returns its doc-id,
	// or util.NO_MORE_DOCS if exhausted.
	// Returned doc-ids are batch-local (0 to numDocs - 1) and are emitted in
	// non-decreasing order.
	NextDoc() int

	// LongValue returns the value at the current cursor position.
	// Only valid after a successful NextDoc() call that returned a value
	// other than util.NO_MORE_DOCS.
	LongValue() int64
}
