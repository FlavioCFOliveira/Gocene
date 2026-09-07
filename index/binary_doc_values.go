package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BinaryDocValues defines an iterator over binary values.
// It extends DocValuesIterator, allowing advancing to an exact doc ID.
type BinaryDocValues interface {
	DocValuesIterator

	// BinaryValue returns the binary value for the current document ID.
	// It is illegal to call this method after AdvanceExact(int) returned false.
	BinaryValue() ([]byte, error)

	// GetVectorByteLength returns the vector byte length.
	GetVectorByteLength() int
}
