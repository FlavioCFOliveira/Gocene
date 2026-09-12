// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// LookupSetTerm checks if key exists, and if so, returns its ordinal, else returns -insertionPoint-1,
// like binary search.
// This is the Go port of SortedSetDocValues.lookupTerm.
func LookupSetTerm(ssdv SortedSetDocValues, key *util.BytesRef) (int, error) {
	low := 0
	high := ssdv.GetValueCount() - 1

	for low <= high {
		mid := (low + high) >> 1
		term, err := ssdv.LookupOrd(mid)
		if err != nil {
			return 0, err
		}
		cmp := bytes.Compare(term, key.ValidBytes())

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

// OpenSetTermsEnum returns a TermsEnum over the values.
// This is the Go port of SortedSetDocValues.termsEnum.
func OpenSetTermsEnum(field string, ssdv SortedSetDocValues) (TermsEnum, error) {
	return &sortedSetDVTermsEnum{SortedSetDocValuesTermsEnum: NewSortedSetDocValuesTermsEnum(field, ssdv)}, nil
}

// IntersectSet returns a TermsEnum over the values, filtered by a
// CompiledAutomaton. This is the Go port of SortedSetDocValues.intersect.
func IntersectSet(field string, ssdv SortedSetDocValues, compiled *automaton.CompiledAutomaton) (TermsEnum, error) {
	in, err := OpenSetTermsEnum(field, ssdv)
	if err != nil {
		return nil, err
	}
	if compiled == nil {
		return in, nil
	}
	switch compiled.Type {
	case automaton.AutomatonTypeNone:
		// Return empty terms enum (TermsEnum.EMPTY in Lucene).
		return &EmptyTermsEnum{}, nil
	case automaton.AutomatonTypeAll:
		return in, nil
	case automaton.AutomatonTypeSingle:
		return newSingleTermFilteredEnum(in, NewTerm(field, compiled.Term)), nil
	case automaton.AutomatonTypeNormal:
		return NewAutomatonTermsEnum(in, compiled), nil
	default:
		return nil, fmt.Errorf("unhandled automaton type: %v", compiled.Type)
	}
}

// sortedSetDVTermsEnum presents SortedSetDocValuesTermsEnum on the full
// TermsEnum surface: the concrete type does not yet carry the impacts()
// override that org.apache.lucene.index.SortedSetDocValuesTermsEnum declares.
type sortedSetDVTermsEnum struct {
	*SortedSetDocValuesTermsEnum
}

// Impacts is unsupported, mirroring
// SortedSetDocValuesTermsEnum.impacts(int) which throws
// UnsupportedOperationException in Apache Lucene 10.5.0.
func (e *sortedSetDVTermsEnum) Impacts(_ int) (spi.ImpactsEnum, error) {
	return nil, ErrUnsupportedSortedSetDVOp
}
