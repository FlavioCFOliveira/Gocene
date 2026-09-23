// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// Test-framework helpers shared by the ports of the Lucene 10.5.0 test classes
// of org.apache.lucene.util that live in package util. They render the
// members of org.apache.lucene.tests.util.LuceneTestCase and
// org.apache.lucene.tests.util.TestUtil that those classes call, at the
// Lucene default scale (RANDOM_MULTIPLIER = 1, TEST_NIGHTLY = false).

import (
	"math/rand"
	"testing"
	"time"
)

// newTestRandom renders LuceneTestCase.random(): a per-test random source
// whose seed is logged so a failure can be reproduced.
func newTestRandom(t testing.TB) *rand.Rand {
	t.Helper()
	seed := time.Now().UnixNano()
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewSource(seed))
}

// atLeast renders LuceneTestCase.atLeast(Random, int) with RANDOM_MULTIPLIER = 1.
func atLeast(r *rand.Rand, i int) int {
	minimum := i
	maximum := minimum + (minimum / 2)
	return nextInt(r, minimum, maximum)
}

// nextInt renders TestUtil.nextInt(Random, int, int): start and end are both
// inclusive.
func nextInt(r *rand.Rand, start, end int) int {
	return start + r.Intn(end-start+1)
}

// randomSimpleString renders TestUtil.randomSimpleString(Random): 0 to 10
// characters in 'a'..'z'.
func randomSimpleString(r *rand.Rand) string {
	end := nextInt(r, 0, 10)
	if end == 0 {
		return ""
	}
	buffer := make([]byte, end)
	for i := range buffer {
		buffer[i] = byte(nextInt(r, 'a', 'z'))
	}
	return string(buffer)
}

// randomBinaryTerm renders TestUtil.randomBinaryTerm(Random, int).
func randomBinaryTerm(r *rand.Rand, length int) *BytesRef {
	b := NewBytesRefWithCapacity(length)
	b.Bytes = b.Bytes[:cap(b.Bytes)]
	r.Read(b.Bytes)
	b.Length = length
	return b
}

// expectPanic renders LuceneTestCase.expectThrows for the unchecked Java
// exceptions that Gocene raises as panics.
func expectPanic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("expected a panic")
		}
	}()
	fn()
}
