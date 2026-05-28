// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// fuzzTokenizerMaxInput caps the size, in bytes, of an input the tokenizer
// fuzz target will tokenize. The UAX#29 scanner is O(n) in the input, so a
// multi-megabyte mutated input would make a single fuzz iteration slow
// without testing any new code path. Inputs above this bound are skipped.
const fuzzTokenizerMaxInput = 1 << 16 // 64 KiB

// fuzzTokenizerMaxTokens bounds the number of tokens consumed from a single
// input. The property under test is "terminates": IncrementToken must make
// forward progress and eventually return (false, nil) at end of input. This
// cap is a safety net — if the scanner ever fails to advance, the test fails
// loudly instead of hanging the fuzzing engine. The bound is generous: even
// every byte becoming its own single-rune token stays well under it for a
// 64 KiB input.
const fuzzTokenizerMaxTokens = fuzzTokenizerMaxInput + 1

// FuzzStandardTokenizer fuzzes the UAX#29 StandardTokenizer over arbitrary
// input.
//
// Properties:
//   - No panic. Tokenizer input is untrusted (raw document/field text); a
//     malformed byte sequence must never crash the host.
//   - Termination. The token stream must reach end-of-input in a bounded
//     number of steps; a scanner that stops advancing is a bug.
//
// The seed corpus mixes scripts and boundary conditions that exercise the
// distinct branches of the word-break state machine: Latin words, numbers,
// CJK ideographs, Hiragana/Katakana, Hangul, South-East Asian runs, emoji
// (including ZWJ and regional-indicator sequences), combining marks, control
// characters, and invalid UTF-8 (which Go's reader replaces with U+FFFD).
func FuzzStandardTokenizer(f *testing.F) {
	seeds := []string{
		"",
		" ",
		"hello",
		"hello world",
		"The quick brown fox.",
		"foo123 bar456",
		"e-mail test@example.com http://example.org/path",
		"café naïve résumé",
		"日本語のテスト",
		"中文分词测试",
		"한국어 텍스트",
		"ひらがな カタカナ",
		"ภาษาไทยทดสอบ",
		"emoji 👍🏽 👨‍👩‍👧‍👦 🇵🇹 here",
		"áê",                    // combining acute / circumflex
		"\x00\x01\x02control",     // control characters
		"\xff\xfe\xfd",            // invalid UTF-8 bytes -> U+FFFD
		"mix日本ed中script文",         // script boundaries with no spaces
		strings.Repeat("a", 1024), // long single token (exercises chunking)
		strings.Repeat("a b ", 256),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Bound the work per iteration; oversized mutated inputs add latency
		// without covering new branches.
		if len(input) > fuzzTokenizerMaxInput {
			return
		}

		tok := NewStandardTokenizer()
		if err := tok.SetReader(strings.NewReader(input)); err != nil {
			// A reader-setup failure is not the property under test.
			return
		}
		if err := tok.Reset(); err != nil {
			return
		}

		for i := 0; ; i++ {
			if i > fuzzTokenizerMaxTokens {
				t.Fatalf("tokenizer did not terminate after %d tokens on input %q", i, input)
			}
			more, err := tok.IncrementToken()
			if err != nil {
				// Tokenization errors are acceptable; the properties are
				// no-panic and termination, both still satisfied here.
				return
			}
			if !more {
				break
			}
		}

		// End and Close must also be panic-free on any reachable state.
		_ = tok.End()
		_ = tok.Close()
	})
}
