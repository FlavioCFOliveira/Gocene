// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BitSetProducer is a producer of BitSets per segment.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.BitSetProducer.
type BitSetProducer interface {
	// GetBitSet returns a BitSet matching the expected documents on the given segment.
	// This may return nil if no documents match.
	GetBitSet(context *index.LeafReaderContext) (util.BitSet, error)
}
