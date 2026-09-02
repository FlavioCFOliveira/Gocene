//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	// bytesScratchSize is the size in bytes of the shared byte scratch buffer.
	bytesScratchSize = 4 * 1024
	// intsScratchSize is the number of ints in the shared int scratch buffer (4 KB).
	intsScratchSize = 1024
)

// sharedIndexingScratch is a holder of lazily-allocated, shared scratch buffers owned by IndexingChain
// and provided to per-field writers that need transient buffers during indexing.
//
// Because IndexingChain (and the DocumentsWriterPerThread it belongs to) indexes
// documents single-threadedly, these buffers are safe to reuse across all writers within the same
// chain. Callers must treat each buffer as transient scratch: fill it, drain it within the same
// call, and not retain a reference across calls.
type sharedIndexingScratch struct {
	bytesUsed          util.CounterAPI
	bytesScratchBuffer []byte
	intsScratchBuffer  []int32
}

// newSharedIndexingScratch constructs a new sharedIndexingScratch.
func newSharedIndexingScratch(bytesUsed util.CounterAPI) *sharedIndexingScratch {
	return &sharedIndexingScratch{
		bytesUsed: bytesUsed,
	}
}

// bytesScratch returns the shared byte scratch buffer, allocating it on the first call and tracking its RAM
// via the Counter supplied at construction.
//
// Callers must treat the returned array as transient scratch.
func (s *sharedIndexingScratch) bytesScratch() []byte {
	if s.bytesScratchBuffer == nil {
		s.bytesScratchBuffer = make([]byte, bytesScratchSize)
		s.bytesUsed.AddAndGet(int64(bytesScratchSize))
	}
	return s.bytesScratchBuffer
}

// intsScratch returns the shared int scratch buffer, allocating it on the first call and tracking its RAM via
// the Counter supplied at construction.
//
// Callers must treat the returned array as transient scratch.
func (s *sharedIndexingScratch) intsScratch() []int32 {
	if s.intsScratchBuffer == nil {
		s.intsScratchBuffer = make([]int32, intsScratchSize)
		s.bytesUsed.AddAndGet(int64(intsScratchSize) * 4)
	}
	return s.intsScratchBuffer
}
