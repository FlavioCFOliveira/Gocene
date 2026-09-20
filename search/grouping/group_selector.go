package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// State defines what to do with the current value.
type State int

const (
	StateSkip State = iota
	StateAccept
)

// GroupSelector defines a group, for use by grouping collectors.
// A GroupSelector acts as an iterator over documents. For each segment, clients should call
// SetNextReader, and then AdvanceTo for each matching document.
type GroupSelector[T any] interface {
	// SetNextReader sets the LeafReaderContext.
	SetNextReader(readerContext *index.LeafReaderContext) error

	// SetScorer sets the current Scorer.
	SetScorer(scorer search.Scorable) error

	// AdvanceTo advances the GroupSelector's iterator to the given document.
	AdvanceTo(doc int) (State, error)

	// CurrentValue gets the group value of the current document.
	// N.B. this object may be reused, for a persistent version use CopyValue.
	CurrentValue() (T, error)

	// CopyValue returns a copy of the group value of the current document.
	CopyValue() (T, error)

	// SetGroups sets a restriction on the group values returned by this selector.
	// If the selector is positioned on a document whose group value is not contained within this
	// set, then AdvanceTo will return StateSkip.
	SetGroups(groups []SearchGroup[T])
}
