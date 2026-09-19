// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// GroupSelectorState renders the nested enum
// org.apache.lucene.search.grouping.GroupSelector.State: what to do with the
// current value.
type GroupSelectorState int

const (
	// GroupSelectorStateSkip mirrors GroupSelector.State.SKIP.
	GroupSelectorStateSkip GroupSelectorState = iota
	// GroupSelectorStateAccept mirrors GroupSelector.State.ACCEPT.
	GroupSelectorStateAccept
)

// String returns the Java enum constant name.
func (s GroupSelectorState) String() string {
	switch s {
	case GroupSelectorStateSkip:
		return "SKIP"
	case GroupSelectorStateAccept:
		return "ACCEPT"
	default:
		return "UNKNOWN"
	}
}

// GroupSelector defines a group, for use by grouping collectors.
//
// A GroupSelector acts as an iterator over documents. For each segment,
// clients should call SetNextReader, and then AdvanceTo for each matching
// document.
//
// Mirrors the abstract class
// org.apache.lucene.search.grouping.GroupSelector<T>; every Java member is
// abstract, so the Go rendering is an interface.
type GroupSelector[T any] interface {
	// SetNextReader sets the LeafReaderContext.
	SetNextReader(readerContext *index.LeafReaderContext) error

	// SetScorer sets the current Scorer.
	SetScorer(scorer search.Scorable) error

	// AdvanceTo advances the GroupSelector's iterator to the given document.
	AdvanceTo(doc int) (GroupSelectorState, error)

	// CurrentValue returns the group value of the current document.
	//
	// N.B. this object may be reused; for a persistent version use CopyValue.
	CurrentValue() (T, error)

	// CopyValue returns a copy of the group value of the current document.
	CopyValue() (T, error)

	// SetGroups sets a restriction on the group values returned by this
	// selector. If the selector is positioned on a document whose group value
	// is not contained within this set, then AdvanceTo returns
	// GroupSelectorStateSkip.
	SetGroups(groups []*SearchGroup[T])
}
