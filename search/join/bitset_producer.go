package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BitSetProducer is a producer of BitSets per segment.
type BitSetProducer interface {
	// GetBitSet produces a BitSet matching the expected documents on the given segment.
	// This may return nil if no documents match.
	GetBitSet(context *index.LeafReaderContext) (util.BitSet, error)
}
