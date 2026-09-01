// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync/atomic"
)

const (
	// BytesScratchSize is the size in bytes of the shared byte scratch buffer.
	BytesScratchSize = 4 * 1024
	// IntsScratchSize is the number of ints in the shared int scratch buffer.
	IntsScratchSize = 1024
)

// SharedIndexingScratch is a holder of lazily-allocated, shared scratch buffers owned by IndexingChain
// and provided to per-field writers that need transient buffers during indexing.
//
// Because IndexingChain indexes documents single-threadedly, these buffers are safe to reuse across
// all writers within the same chain. Callers must treat each buffer as transient scratch.
// Mirrors org.apache.lucene.index.SharedIndexingScratch from Apache Lucene 10.5.0.
type SharedIndexingScratch struct {
	bytesUsed           *atomic.Int64
	bytesScratchBuffer []byte
	intsScratchBuffer  []int32
}

// NewSharedIndexingScratch constructs a SharedIndexingScratch.
func NewSharedIndexingScratch(bytesUsed *atomic.Int64) *SharedIndexingScratch {
	return &SharedIndexingScratch{
		bytesUsed: bytesUsed,
	}
}

// BytesScratch returns the shared byte scratch buffer, allocating it on the first call
// and tracking its RAM via the counter supplied at construction.
func (s *SharedIndexingScratch) BytesScratch() []byte {
	if s.bytesScratchBuffer == nil {
		s.bytesScratchBuffer = make([]byte, BytesScratchSize)
		s.bytesUsed.Add(BytesScratchSize)
	}
	return s.bytesScratchBuffer
}

// IntsScratch returns the shared int scratch buffer, allocating it on the first call
// and tracking its RAM via the counter supplied at construction.
func (s *SharedIndexingScratch) IntsScratch() []int32 {
	if s.intsScratchBuffer == nil {
		s.intsScratchBuffer = make([]int32, IntsScratchSize)
		s.bytesUsed.Add(int64(IntsScratchSize * 4)) // 4 bytes per int32
	}
	return s.intsScratchBuffer
}
