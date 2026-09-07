package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortedDocValues is a per-document byte[] with presorted values.
// This is fundamentally an iterator over the int ord values per document,
// with random access APIs to resolve an int ord to BytesRef.
// This is the Go port of Lucene's org.apache.lucene.index.SortedDocValues.
type SortedDocValues interface {
	DocValuesIterator
	// OrdValue returns the ordinal for the current docID.
	// It is illegal to call this method after AdvanceExact(int) returned false.
	// Ordinals are dense, start at 0, then increment by 1 for the next value in sorted order.
	OrdValue() (int, error)

	// LookupOrd retrieves the value for the specified ordinal.
	// The returned BytesRef may be re-used across calls to LookupOrd so make sure to copy it if you want to keep it around.
	LookupOrd(ord int) (*util.BytesRef, error)

	// GetValueCount returns the number of unique values.
	// This is also equivalent to one plus the maximum ordinal.
	GetValueCount() int
}

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

// TermsEnum returns a TermsEnum over the values.
// This is the Go port of SortedDocValues.termsEnum.
func TermsEnum(field string, sdv SortedDocValues) (TermsEnum, error) {
	return NewSortedDocValuesTermsEnum(field, sdv)
}

// Intersect returns a TermsEnum over the values, filtered by a CompiledAutomaton.
// This is the Go port of SortedDocValues.intersect.
func Intersect(field string, sdv SortedDocValues, automaton util.CompiledAutomaton) (TermsEnum, error) {
	in, err := TermsEnum(field, sdv)
	if err != nil {
		return nil, err
	}
	switch automaton.Type {
	case util.AutomatonTypeNone:
		// Return empty terms enum
		return NewEmptyTermsEnum(), nil
	case util.AutomatonTypeAll:
		return in, nil
	case util.AutomatonTypeSingle:
		return NewSingleTermsEnum(in, automaton.Term), nil
	case util.AutomatonTypeNormal:
		return NewAutomatonTermsEnum(in, automaton), nil
	default:
		return nil, fmt.Errorf("unhandled automaton type: %v", automaton.Type)
	}
}
