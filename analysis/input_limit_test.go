// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
)

// fillReader yields n bytes of the single byte b without allocating them all,
// so tests can exercise the > MaxTokenizerInputSize boundary cheaply.
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

// oversizedReader returns a reader that produces one byte more than the limit.
func oversizedReader() io.Reader {
	return &fillReader{b: 'a', remaining: int64(MaxTokenizerInputSize) + 1}
}

func TestReadAllLimited(t *testing.T) {
	t.Parallel()

	t.Run("nil reader", func(t *testing.T) {
		data, err := readAllLimited(nil)
		if err != nil || data != nil {
			t.Fatalf("nil reader: data=%v err=%v", data, err)
		}
	})

	t.Run("small input passes through", func(t *testing.T) {
		const want = "hello world"
		data, err := readAllLimited(strings.NewReader(want))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != want {
			t.Fatalf("data=%q want %q", data, want)
		}
	})

	t.Run("exactly at limit passes", func(t *testing.T) {
		r := &fillReader{b: 'a', remaining: int64(MaxTokenizerInputSize)}
		data, err := readAllLimited(r)
		if err != nil {
			t.Fatalf("input at limit must be accepted, got %v", err)
		}
		if len(data) != MaxTokenizerInputSize {
			t.Fatalf("len(data)=%d want %d", len(data), MaxTokenizerInputSize)
		}
	})

	t.Run("over limit rejected", func(t *testing.T) {
		data, err := readAllLimited(oversizedReader())
		if !errors.Is(err, ErrInputTooLarge) {
			t.Fatalf("over-limit input: err=%v want ErrInputTooLarge", err)
		}
		if data != nil {
			t.Fatalf("over-limit input should return nil data, got %d bytes", len(data))
		}
	})

	t.Run("propagates underlying read error", func(t *testing.T) {
		sentinel := errors.New("boom")
		_, err := readAllLimited(&errReader{err: sentinel})
		if !errors.Is(err, sentinel) {
			t.Fatalf("err=%v want %v", err, sentinel)
		}
	})
}

// errReader always fails with err.
type errReader struct{ err error }

func (e *errReader) Read([]byte) (int, error) { return 0, e.err }

func TestStandardTokenizer_InputTooLarge(t *testing.T) {
	t.Parallel()
	tok := NewStandardTokenizer()
	if err := tok.SetReader(oversizedReader()); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("SetReader err=%v want ErrInputTooLarge", err)
	}
}

func TestCJKTokenizer_InputTooLarge(t *testing.T) {
	t.Parallel()
	tok := NewCJKTokenizer()
	if err := tok.SetReader(oversizedReader()); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("SetReader err=%v want ErrInputTooLarge", err)
	}
}

func TestPatternTokenizer_InputTooLarge(t *testing.T) {
	t.Parallel()
	tok := NewPatternTokenizer(regexp.MustCompile(`\s+`))
	if err := tok.SetReader(oversizedReader()); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("SetReader err=%v want ErrInputTooLarge", err)
	}
}

func TestSimplePatternSplitTokenizer_InputTooLarge(t *testing.T) {
	t.Parallel()
	tok, err := NewSimplePatternSplitTokenizer(regexp.MustCompile(`\s+`))
	if err != nil {
		t.Fatalf("construct tokenizer: %v", err)
	}
	if err := tok.SetReader(oversizedReader()); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("SetReader err=%v want ErrInputTooLarge", err)
	}
}

func TestSimplePatternTokenizer_InputTooLarge(t *testing.T) {
	t.Parallel()
	tok := NewSimplePatternTokenizerWithRegexp(regexp.MustCompile(`\w+`))
	if err := tok.SetReader(oversizedReader()); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("SetReader err=%v want ErrInputTooLarge", err)
	}
}

// TestHTMLStripCharFilter_InputTooLarge asserts the char filter, whose
// constructor cannot return an error, surfaces ErrInputTooLarge through the
// io.Reader contract on Read rather than silently truncating.
func TestHTMLStripCharFilter_InputTooLarge(t *testing.T) {
	t.Parallel()
	f := NewHTMLStripCharFilter(oversizedReader())
	buf := make([]byte, 64)
	if _, err := f.Read(buf); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("Read err=%v want ErrInputTooLarge", err)
	}
}

// TestHTMLStripCharFilter_SmallInputStillWorks guards against the size check
// breaking the normal path: a small HTML input must still strip and read back.
func TestHTMLStripCharFilter_SmallInputStillWorks(t *testing.T) {
	t.Parallel()
	f := NewHTMLStripCharFilter(strings.NewReader("<p>hi &amp; bye</p>"))
	out, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(out), "hi") || !strings.Contains(string(out), "bye") {
		t.Fatalf("unexpected stripped output: %q", out)
	}
	if strings.Contains(string(out), "<p>") {
		t.Fatalf("tags not stripped: %q", out)
	}
}
