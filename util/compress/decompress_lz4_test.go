// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compress

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDecompressOffset0 mirrors Lucene's
// org.apache.lucene.util.compress.TestDecompressLZ4#testDecompressOffset0.
// A token byte advertising a 0-length literal and a match-length of 14,
// followed by a 16-bit little-endian offset of 0, must be rejected with
// ErrInvalidOffset because the LZ4 format never produces such a match.
func TestDecompressOffset0(t *testing.T) {
	t.Parallel()

	input := []byte{
		// token: 0 literals, match length = 14 + MinMatch
		0xE,
		// offset 0 (invalid)
		0,
		0,
		// last literal block: token announcing 7 literals
		7 << 4,
		// seven literal bytes (their value is irrelevant; never reached)
		0, 0, 0, 0, 0, 0, 0,
	}

	dest := make([]byte, 18)

	_, err := LZ4Decompress(store.NewByteArrayDataInput(input), len(dest), dest, 0)
	if !errors.Is(err, ErrInvalidOffset) {
		t.Fatalf("LZ4Decompress: got err=%v, want %v", err, ErrInvalidOffset)
	}
}
