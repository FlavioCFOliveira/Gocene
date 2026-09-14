package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FreqAndNormBuffer is a wrapper around parallel arrays storing term frequencies and length normalization factors.
// This is the Go port of Lucene's org.apache.lucene.index.FreqAndNormBuffer.
//
// Lucene declares the class once. Its state and methods live in util, the
// dependency floor that spi.Impacts#GetImpacts must name, so the index spelling
// is an alias of that single rendering rather than a second, incompatible struct.
type FreqAndNormBuffer = util.FreqAndNormBuffer

// NewFreqAndNormBuffer creates a new FreqAndNormBuffer.
func NewFreqAndNormBuffer() *FreqAndNormBuffer {
	return util.NewFreqAndNormBuffer()
}
