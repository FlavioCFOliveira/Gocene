package index

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// LookupTerm looks up the ordinal of a key. If key exists, returns its ordinal,
// else returns -insertionPoint-1, like Arrays.binarySearch.
// This is the Go port of SortedDocValues.lookupTerm.
func LookupTerm(sdv SortedDocValues, key *util.BytesRef) (int, error) {
	low := 0
	high := sdv.GetValueCount() - 1

	for low <= high {
		mid := (low + high) >> 1
		term, err := sdv.LookupOrd(mid)
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

// OpenTermsEnum returns a TermsEnum over the values.
// This is the Go port of SortedDocValues.termsEnum.
func OpenTermsEnum(field string, sdv SortedDocValues) (TermsEnum, error) {
	return &sortedDVTermsEnum{SortedDocValuesTermsEnum: NewSortedDocValuesTermsEnum(field, sdv)}, nil
}

// sortedDVTermsEnum presents SortedDocValuesTermsEnum on the full TermsEnum
// surface. SortedDocValuesTermsEnum reports the ordinal as the int used by
// org.apache.lucene.index.SortedDocValues, whereas TermsEnum.ord() is a long,
// and it does not yet carry the impacts() override that
// org.apache.lucene.index.SortedDocValuesTermsEnum declares. Both are supplied
// here so the doc-values dictionary can stand in for any TermsEnum.
type sortedDVTermsEnum struct {
	*SortedDocValuesTermsEnum
}

// Ord widens the int ordinal of SortedDocValues to the int64 ordinal of
// TermsEnum.ord().
func (e *sortedDVTermsEnum) Ord() int64 { return int64(e.SortedDocValuesTermsEnum.Ord()) }

// Impacts is unsupported, mirroring
// SortedDocValuesTermsEnum.impacts(int) which throws
// UnsupportedOperationException in Apache Lucene 10.5.0.
func (e *sortedDVTermsEnum) Impacts(_ int) (spi.ImpactsEnum, error) {
	return nil, ErrUnsupportedSortedDVOp
}

// Intersect returns a TermsEnum over the values, filtered by a CompiledAutomaton.
// This is the Go port of SortedDocValues.intersect.
func Intersect(field string, sdv SortedDocValues, compiled *automaton.CompiledAutomaton) (TermsEnum, error) {
	in, err := OpenTermsEnum(field, sdv)
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

// singleTermAcceptor accepts exactly one term and ends the enumeration at the
// first term that differs. It is the predicate half of the Go port of
// org.apache.lucene.index.SingleTermsEnum.
type singleTermAcceptor struct {
	single *Term
}

// Accept mirrors SingleTermsEnum.accept: YES for the single term, END for
// anything else (the delegate is sorted, so the first mismatch after the
// initial seek is past the single term).
func (a *singleTermAcceptor) Accept(term *Term) (AcceptStatus, error) {
	if term != nil && term.Equals(a.single) {
		return AcceptYes, nil
	}
	return AcceptEnd, nil
}

// NextSeekTerm mirrors FilteredTermsEnum.nextSeekTerm's default, which returns
// the (already consumed) initial seek term and then nil.
func (a *singleTermAcceptor) NextSeekTerm(_ *Term) (*Term, error) { return nil, nil }

// newSingleTermFilteredEnum is the Go port of
// org.apache.lucene.index.SingleTermsEnum: a FilteredTermsEnum over in that
// enumerates termText and nothing else, seeded with setInitialSeekTerm.
func newSingleTermFilteredEnum(in TermsEnum, termText *Term) *FilteredTermsEnum {
	enum := NewFilteredTermsEnum(in, &singleTermAcceptor{single: termText})
	enum.SetInitialSeekTerm(termText)
	return enum
}
