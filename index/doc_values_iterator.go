//go:build ignore

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValuesIterator defines an iterator over doc values that allows advancing to an exact doc ID.
type DocValuesIterator interface {
	util.DocIdSetIterator
	// AdvanceExact advances the iterator to exactly target and returns whether target has a value.
	// target must be greater than or equal to the current doc ID and must be a valid doc ID.
	AdvanceExact(target int) (bool, error)
}
