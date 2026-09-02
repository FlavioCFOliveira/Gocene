// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// TermState encapsulates all state required by a TermsEnum to reconstruct
// the postings for a specific term, decoupled from any particular TermsEnum
// instance. Implementations are codec-specific. Mirrors
// org.apache.lucene.index.TermState from Apache Lucene 10.4.0.
//
// Concrete implementations must satisfy CopyFrom — Lucene's mechanism for
// "reset this state from the contents of other". Gocene uses an interface
// rather than an abstract class.
type TermState interface {
	// CopyFrom resets this state from other. Implementations should panic or
	// return an error if other's concrete type is incompatible.
	CopyFrom(other TermState) error
}

// OrdTermState is an ordinal-based TermState. Mirrors
// org.apache.lucene.index.OrdTermState (Lucene 10.4.0).
type OrdTermState struct {
	// Ord is the term ordinal — its position in the full list of sorted terms.
	Ord int64
}

// NewOrdTermState constructs an OrdTermState.
func NewOrdTermState() *OrdTermState { return &OrdTermState{} }

// CopyFrom resets this OrdTermState from other. Returns an error if other is
// not an *OrdTermState.
func (s *OrdTermState) CopyFrom(other TermState) error {
	o, ok := other.(*OrdTermState)
	if !ok {
		return fmt.Errorf("OrdTermState.CopyFrom: incompatible source type %T", other)
	}
	s.Ord = o.Ord
	return nil
}

// String returns a debug representation matching Lucene's.
func (s *OrdTermState) String() string {
	return fmt.Sprintf("OrdTermState ord=%d", s.Ord)
}
