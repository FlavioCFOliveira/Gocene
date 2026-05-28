// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"errors"
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// fillReader yields n bytes of b without allocating them all.
type fillReader struct {
	b         byte
	remaining int64
}

func (f *fillReader) Read(p []byte) (int, error) {
	if f.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > f.remaining {
		n = int(f.remaining)
	}
	for i := 0; i < n; i++ {
		p[i] = f.b
	}
	f.remaining -= int64(n)
	return n, nil
}

func oversizedReader() io.Reader {
	return &fillReader{b: 'a', remaining: int64(analysis.MaxTokenizerInputSize) + 1}
}

// TestKoreanTokenizer_InputTooLarge asserts that SetReader rejects an input
// exceeding analysis.MaxTokenizerInputSize with analysis.ErrInputTooLarge,
// before any Viterbi decoding is attempted.
func TestKoreanTokenizer_InputTooLarge(t *testing.T) {
	t.Parallel()
	tok := NewKoreanTokenizer()
	if err := tok.SetReader(oversizedReader()); !errors.Is(err, analysis.ErrInputTooLarge) {
		t.Fatalf("SetReader err=%v want analysis.ErrInputTooLarge", err)
	}
}
