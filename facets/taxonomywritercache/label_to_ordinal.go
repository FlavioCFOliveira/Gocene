package taxonomywritercache

// LabelToOrdinal is the abstract base shared by every cache that maps facet
// labels to integer ordinals. Mirrors
// org.apache.lucene.facet.taxonomy.writercache.LabelToOrdinal.
//
// InvalidOrdinal is the sentinel returned when a label is not known.
const InvalidOrdinal = -1

// LabelToOrdinal is the contract for label-to-ordinal storage. The Java port
// uses an abstract class with a counter for the highest ordinal observed; in
// Go we expose the same operations as an interface plus a small base struct
// callers can embed.
type LabelToOrdinal interface {
	GetMaxOrdinal() int
	GetNextOrdinal() int
	AddLabel(label string, ord int)
	GetOrdinal(label string) int
}

// LabelToOrdinalBase carries the highest-ordinal counter shared by every
// concrete LabelToOrdinal implementation; embed it in a struct and override
// AddLabel/GetOrdinal to provide storage.
type LabelToOrdinalBase struct {
	counter int
}

// GetMaxOrdinal returns the highest ordinal observed so far.
func (b *LabelToOrdinalBase) GetMaxOrdinal() int { return b.counter }

// GetNextOrdinal returns the next ordinal to hand out and advances the counter.
func (b *LabelToOrdinalBase) GetNextOrdinal() int {
	o := b.counter
	b.counter++
	return o
}

// SetCounter sets the highest-ordinal counter directly.
func (b *LabelToOrdinalBase) SetCounter(v int) { b.counter = v }
