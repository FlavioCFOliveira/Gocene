// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

// runeReader wraps strings.NewReader to implement io.RuneReader.
type runeReader struct{ *strings.Reader }

func newRuneReader(s string) *runeReader { return &runeReader{strings.NewReader(s)} }

// TestRollingCharBuffer_Sequential reads every character in order and
// verifies identity with the source string.
func TestRollingCharBuffer_Sequential(t *testing.T) {
	for _, s := range []string{"", "hello", "héllo wörld", "日本語テスト"} {
		runes := []rune(s)
		buf := NewRollingCharBuffer()
		buf.Reset(newRuneReader(s))
		for i, want := range runes {
			got, err := buf.Get(i)
			if err != nil {
				t.Fatalf("Get(%d): %v", i, err)
			}
			if got != want {
				t.Errorf("s=%q Get(%d) = %c, want %c", s, i, got, want)
			}
		}
		// Next read must be EOF.
		eof, err := buf.Get(len(runes))
		if err != nil {
			t.Fatalf("EOF Get: %v", err)
		}
		if eof != -1 {
			t.Errorf("expected EOF (-1), got %c", eof)
		}
	}
}

// TestRollingCharBuffer_RandomAccess mirrors the Java test: random forward
// reads, backward re-reads, and slice reads with intermittent freeBefore.
func TestRollingCharBuffer_RandomAccess(t *testing.T) {
	rng := rand.New(rand.NewSource(0x5EED))

	const iters = 50
	for iter := 0; iter < iters; iter++ {
		// Build a random ASCII+Unicode string.
		var sb strings.Builder
		n := rng.Intn(500) + 1
		for i := 0; i < n; i++ {
			cp := rune(rng.Intn(0x1000))
			if !utf8.ValidRune(cp) {
				cp = 'x'
			}
			sb.WriteRune(cp)
		}
		s := sb.String()
		runes := []rune(s)

		buf := NewRollingCharBuffer()
		buf.Reset(newRuneReader(s))

		nextRead := 0
		availCount := 0

		for nextRead < len(runes) {
			if availCount == 0 || rng.Intn(2) == 0 {
				// Advance to next character.
				got, err := buf.Get(nextRead)
				if err != nil {
					t.Fatalf("iter=%d Get(%d): %v", iter, nextRead, err)
				}
				if got != runes[nextRead] {
					t.Fatalf("iter=%d Get(%d) = %c, want %c", iter, nextRead, got, runes[nextRead])
				}
				nextRead++
				availCount++
			} else if rng.Intn(2) == 0 && availCount > 0 {
				// Re-read a previous character.
				pos := nextRead - availCount + rng.Intn(availCount)
				got, err := buf.Get(pos)
				if err != nil {
					t.Fatalf("iter=%d back-Get(%d): %v", iter, pos, err)
				}
				if got != runes[pos] {
					t.Fatalf("iter=%d back-Get(%d) = %c, want %c", iter, pos, got, runes[pos])
				}
			} else if availCount > 0 {
				// Read a slice.
				length := 1
				if availCount > 1 {
					length = 1 + rng.Intn(availCount)
				}
				start := nextRead - availCount
				if length < availCount {
					start += rng.Intn(availCount - length)
				}
				slice := buf.GetSlice(start, length)
				want := runes[start : start+length]
				for i := range want {
					if slice[i] != want[i] {
						t.Fatalf("iter=%d slice[%d] = %c, want %c", iter, i, slice[i], want[i])
					}
				}
			}

			if availCount > 0 && rng.Intn(20) == 17 {
				toFree := rng.Intn(availCount)
				buf.FreeBefore(nextRead - (availCount - toFree))
				availCount -= toFree
			}
		}
	}
}

// TestRollingCharBuffer_Grow exercises buffer growth by feeding a string longer
// than the initial 512-rune capacity.
func TestRollingCharBuffer_Grow(t *testing.T) {
	runes := make([]rune, 2000)
	for i := range runes {
		runes[i] = rune('a' + i%26)
	}
	s := string(runes)

	buf := NewRollingCharBuffer()
	buf.Reset(newRuneReader(s))

	for i, want := range runes {
		got, err := buf.Get(i)
		if err != nil {
			t.Fatalf("Get(%d): %v", i, err)
		}
		if got != want {
			t.Errorf("Get(%d) = %c, want %c", i, got, want)
		}
	}
}
