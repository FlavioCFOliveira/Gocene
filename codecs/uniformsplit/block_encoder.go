package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BlockEncoder encodes the raw bytes of a block when the index is written.
//
// For example, implementations may compress or encrypt.
//
// Note: This is an experimental feature in Lucene.
type BlockEncoder interface {
	// Encode encodes all the bytes of one block in a single operation.
	// The encoding is per block.
	Encode(blockBytes store.DataInput, length int64) (WritableBytes, error)
}

// WritableBytes is a writable byte buffer.
type WritableBytes interface {
	// Size gets the number of bytes.
	Size() int64

	// WriteTo writes the bytes to the provided DataOutput.
	WriteTo(dataOutput store.DataOutput) error
}
