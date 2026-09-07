// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// OrdTermState is an ordinal based TermState.
//
// This is the Go port of Lucene's org.apache.lucene.index.OrdTermState.
type OrdTermState struct {
	TermState
	// Ord is the term ordinal, i.e. its position in the full list of sorted terms.
	Ord int64
}

// NewOrdTermState constructs a new OrdTermState.
func NewOrdTermState() *OrdTermState {
	return &OrdTermState{}
}

// CopyFrom copies the ordinal from another TermState.
func (ots *OrdTermState) CopyFrom(other TermState) error {
	if otherOTS, ok := other.(*OrdTermState); ok {
		ots.Ord = otherOTS.Ord
		return nil
	}
	return fmt.Errorf("OrdTermState: can not copy from %T", other)
}

// String returns a string representation of the OrdTermState.
func (ots *OrdTermState) String() string {
	return fmt.Sprintf("OrdTermState ord=%d", ots.Ord)
}
