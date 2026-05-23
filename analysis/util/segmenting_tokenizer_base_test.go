// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

// sentenceBreaker is a minimal BreakIterator that splits on "." "!" "?"
// followed by a space (or at the end of text). Used only in tests.
type sentenceBreaker struct {
	buf    []rune
	length int
	pos    int // current position
}

func (b *sentenceBreaker) SetText(buf []rune, length int) {
	b.buf = buf
	b.length = length
	b.pos = 0
}

func (b *sentenceBreaker) Current() int {
	return b.pos
}

func (b *sentenceBreaker) Next() int {
	if b.pos >= b.length {
		return BreakDone
	}
	for i := b.pos; i < b.length; i++ {
		ch := b.buf[i]
		if (ch == '.' || ch == '!' || ch == '?') && (i+1 >= b.length || b.buf[i+1] == ' ') {
			end := i + 1
			b.pos = end
			return end
		}
	}
	// no sentence end — return rest of buffer as one sentence
	end := b.length
	b.pos = end
	return end
}

// TestSegmentingTokenizerBase_BasicFlow verifies the buffer-management loop:
// sentences are found, SetNextSentenceFn is called, and IncrementWordFn
// drives token output.
func TestSegmentingTokenizerBase_BasicFlow(t *testing.T) {
	bi := &sentenceBreaker{}
	base := NewSegmentingTokenizerBase(bi)

	input := "Hello world. Go rocks!"
	base.SetReader(strings.NewReader(input))
	base.Reset()

	// Collect sentence spans.
	var sentences []string
	base.SetNextSentenceFn = func(start, end int) {
		s := string(base.Buffer[start:end])
		sentences = append(sentences, s)
	}

	// IncrementWordFn just returns true once per sentence (whole-sentence mode).
	once := true
	base.IncrementWordFn = func() bool {
		if once {
			once = false
			return true
		}
		return false
	}

	// Drive the loop.
	base.SetNextSentenceFn = func(start, end int) {
		sentences = append(sentences, string(base.Buffer[start:end]))
		once = true
	}

	for {
		ok, err := base.IncrementToken()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
	}

	if len(sentences) < 1 {
		t.Fatal("expected at least one sentence, got none")
	}
}

// TestSegmentingTokenizerBase_Reset verifies that Reset clears state so the
// base can be reused across calls.
func TestSegmentingTokenizerBase_Reset(t *testing.T) {
	bi := &sentenceBreaker{}
	base := NewSegmentingTokenizerBase(bi)

	for _, input := range []string{"First sentence.", "Second sentence."} {
		base.SetReader(strings.NewReader(input))
		base.Reset()

		called := false
		base.SetNextSentenceFn = func(start, end int) { called = true }
		fired := false
		base.IncrementWordFn = func() bool {
			if !fired {
				fired = true
				return true
			}
			return false
		}

		ok, err := base.IncrementToken()
		if err != nil {
			t.Fatalf("input %q: %v", input, err)
		}
		if !ok {
			t.Fatalf("input %q: expected a token", input)
		}
		_ = called
	}
}

// TestSegmentingTokenizerBase_SafeEnd verifies isSafeEnd recognises the
// paragraph-separator characters from the Java original.
func TestSegmentingTokenizerBase_SafeEnd(t *testing.T) {
	bi := &sentenceBreaker{}
	base := NewSegmentingTokenizerBase(bi)

	safe := []rune{0x000D, 0x000A, 0x0085, 0x2028, 0x2029}
	for _, ch := range safe {
		if !base.isSafeEnd(ch) {
			t.Errorf("isSafeEnd(U+%04X) = false, want true", ch)
		}
	}
	unsafe := []rune{' ', 'a', '.', '!', 0x0000}
	for _, ch := range unsafe {
		if base.isSafeEnd(ch) {
			t.Errorf("isSafeEnd(U+%04X) = true, want false", ch)
		}
	}
}
