package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockDecoder decodes the raw bytes of a block when the index is read,
// according to the BlockEncoder used during the writing of the index.
//
// For example, implementations may decompress or decrypt.
type BlockDecoder interface {
	// Decode decodes all the bytes of one block in a single operation.
	// The decoding is per block.
	Decode(blockBytes store.DataInput, length int64) (*util.BytesRef, error)
}
