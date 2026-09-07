// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// SortedSetDocValues is a multi-valued version of SortedDocValues.
//
// Per-Document values in a SortedSetDocValues are deduplicated, dereferenced, and sorted into a
// dictionary of unique values. A pointer to the dictionary value (ordinal) can be retrieved for
// each document. Ordinals are dense and in increasing sorted order.
type SortedSetDocValues interface {
	DocValuesIterator

	// NextOrd returns the next ordinal for the current document.
	// It is illegal to call this method after AdvanceExact(int) returned false.
	// It is illegal to call this more than DocValueCount() times for the currently-positioned doc.
	NextOrd() (int64, error)

	// DocValueCount retrieves the number of unique ords for the current document.
	// This must always be greater than zero.
	// It is illegal to call this method after AdvanceExact(int) returned false.
	DocValueCount() int

	// LookupOrd retrieves the value for the specified ordinal.
	// The returned BytesRef may be re-used across calls to LookupOrd so make sure
	// to clone it if you want to keep it around.
	LookupOrd(ord int64) (*util.BytesRef, error)

	// GetValueCount returns the number of unique values.
	// This is also equivalent to one plus the maximum ordinal.
	GetValueCount() int64

	// TermsEnum returns a TermsEnum over the values.
	// The enum supports TermsEnum.Ord() and TermsEnum.SeekExact(int64).
	TermsEnum() (TermsEnum, error)

	// Intersect returns a TermsEnum over the values, filtered by a CompiledAutomaton.
	// The enum supports TermsEnum.Ord().
	Intersect(automaton *automaton.CompiledAutomaton) (TermsEnum, error)
}

// LookupTerm checks if key exists, and if so, returns its ordinal, else returns -insertionPoint-1,
// like binary search.
func LookupTerm(ssdv SortedSetDocValues, key *util.BytesRef) (int64, error) {
	low := int64(0)
	high := ssdv.GetValueCount() - 1

	for low <= high {
		mid := (low + high) >> 1
		term, err := ssdv.LookupOrd(mid)
		if err != nil {
			return 0, err
		}
		cmp := util.BytesRefCompare(term, key)

		if cmp < 0 {
			low = mid + 1
		} else if cmp > 0 {
			high = mid - 1
		} else {
			return mid, nil // key found
		}
	}

	return -(low + 1), nil // key not found.
}
