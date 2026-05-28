// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classic

import (
	"errors"
	"io"
	"strings"
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

// TestClassicTokenizer_InputTooLarge asserts that an oversized input is
// rejected through SetReader, even though the underlying impl constructor
// cannot itself return an error.
func TestClassicTokenizer_InputTooLarge(t *testing.T) {
	t.Parallel()
	tok := NewClassicTokenizer()
	if err := tok.SetReader(oversizedReader()); !errors.Is(err, analysis.ErrInputTooLarge) {
		t.Fatalf("SetReader err=%v want analysis.ErrInputTooLarge", err)
	}
}

// TestClassicTokenizer_SmallInputStillWorks guards the normal path: a small
// input must tokenise without error after the size guard was added. It reuses
// drainClassicTokenizer from classic_factories_test.go.
func TestClassicTokenizer_SmallInputStillWorks(t *testing.T) {
	t.Parallel()
	tok := NewClassicTokenizer()
	if err := tok.SetReader(strings.NewReader("Hello world")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	got := drainClassicTokenizer(t, tok)
	if len(got) != 2 || got[0] != "Hello" || got[1] != "world" {
		t.Fatalf("tokens=%v want [Hello world]", got)
	}
}
