package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// PostingsWriter is the interface for writing postings lists.
type PostingsWriter interface {
	// Init initializes the postings writer with the given output and segment state.
	Init(out store.IndexOutput, state *spi.SegmentWriteState) error
}
