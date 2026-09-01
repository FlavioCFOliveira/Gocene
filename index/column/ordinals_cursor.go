package column

// OrdinalsCursor is a dense values cursor over a DictionaryColumn.
// It produces exactly Size() ordinals for consecutive batch-local doc-ids
// starting at 0, one per call to NextOrd().
//
// Each ordinal must be in [0, column.Dictionary().length).
//
// Implementations must throw an exception (or panic in Go) if NextOrd()
// is called more than Size() times.
type OrdinalsCursor interface {
	// Size returns the total number of ordinals this cursor will produce.
	Size() int

	// NextOrd returns the next ordinal. Must not be called more than Size() times.
	// The returned value must be in [0, dictionary.length) where dictionary is the
	// enclosing column's dictionary.
	NextOrd() int
}
