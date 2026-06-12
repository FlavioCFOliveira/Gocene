// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomywritercache

import (
	"math"
	"os"
	"testing"
)

// Test2GBCharBlockArray_Monster2GBChars ports the Java @Monster test that
// fills >2 GB of chars to verify the capacity overflow guard.
//
// Run with GOCENE_RUN_MONSTERS=1 to include this test.
func Test2GBCharBlockArray_Monster2GBChars(t *testing.T) {
	if os.Getenv("GOCENE_RUN_MONSTERS") != "1" {
		t.Skip("set GOCENE_RUN_MONSTERS=1 to run this monster test (>2GB CharBlockArray)")
	}
	const blockSize = 32768
	array := NewCharBlockArrayWithBlockSize(blockSize)

	// Java: int size = TestUtil.nextInt(random(), 20000, 40000);
	// Use a fixed deterministic size for reproducibility.
	const size = 30000

	chars := make([]rune, size)
	count := 0
	var panicked bool
	for {
		count++
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
				}
			}()
			array.AppendRunes(chars, 0, size)
		}()
		if panicked {
			// Java: assertTrue(count * (long) size + blockSize > Integer.MAX_VALUE);
			if int64(count)*int64(size)+int64(blockSize) <= int64(math.MaxInt32) {
				t.Fatalf("panic occurred too early: count=%d, size=%d, blockSize=%d, total=%d", count, size, blockSize, int64(count)*int64(size)+int64(blockSize))
			}
			break
		}
		// Java: assertFalse(count * (long) size > Integer.MAX_VALUE);
		if int64(count)*int64(size) > int64(math.MaxInt32) {
			t.Fatalf("appended %d characters beyond Integer.MAX_VALUE without panic", int64(count)*int64(size)-int64(math.MaxInt32))
		}
	}
}
