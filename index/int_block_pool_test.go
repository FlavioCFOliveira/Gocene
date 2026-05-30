// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.index.TestIntBlockPool. The pool itself lives in
// the util package (util.IntBlockPool); this test exercises it from index to
// mirror the Lucene test's location.
package index

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestIntBlockPool_WriteReadReset(t *testing.T) {
	p := util.NewIntBlockPool()
	p.NextBuffer()

	// Write count consecutive ints, allocating new buffers as needed.
	count := rand.Intn(2 * util.IntBlockSize)
	for i := 0; i < count; i++ {
		if p.IntUpto == util.IntBlockSize {
			p.NextBuffer()
		}
		p.Buffer[p.IntUpto] = int32(i)
		p.IntUpto++
	}

	// Check that all ints are present in the pool.
	for i := 0; i < count; i++ {
		if got := p.Buffers[i/util.IntBlockSize][i%util.IntBlockSize]; got != int32(i) {
			t.Fatalf("buffers[%d][%d]=%d want %d", i/util.IntBlockSize, i%util.IntBlockSize, got, i)
		}
	}

	// Reset without zero-fill: the first buffer must still hold the ints.
	if count > util.IntBlockSize {
		count = util.IntBlockSize
	}
	p.Reset(false, true)
	for i := 0; i < count; i++ {
		if got := p.Buffers[0][i]; got != int32(i) {
			t.Fatalf("after reset(false,true) buffers[0][%d]=%d want %d", i, got, i)
		}
	}

	// Reset with zero-fill: no data must remain.
	p.IntUpto = count
	p.Reset(true, true)
	for i := 0; i < count; i++ {
		if got := p.Buffers[0][i]; got != 0 {
			t.Fatalf("after reset(true,true) buffers[0][%d]=%d want 0", i, got)
		}
	}
}

// TestIntBlockPool_TooManyAllocs ports testTooManyAllocs. The Lucene test
// asserts intOffset wraps past Integer.MAX_VALUE; that behaviour is specific to
// Java's 32-bit int. Gocene's IntOffset is a 64-bit int and NextBuffer panics
// before committing an overflowing offset, so the original wrap-around check
// cannot be reproduced. Skipped as a documented JVM/Go divergence.
func TestIntBlockPool_TooManyAllocs(t *testing.T) {
	t.Fatal("intOffset overflow is JVM-specific (32-bit int); not portable to Go's 64-bit int")
}
