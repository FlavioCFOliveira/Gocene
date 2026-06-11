// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene84

import "testing"

func TestBlockSize_Is128(t *testing.T) {
	if blockSize != 128 {
		t.Fatalf("blockSize: got %d, want 128", blockSize)
	}
}

func TestBlockSize_Positive(t *testing.T) {
	if blockSize <= 0 {
		t.Fatal("blockSize must be positive")
	}
}
