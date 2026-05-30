// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"testing"
)

// TestRBBIBreakIterator_Reverse_Parity asserts that iterating backwards
// via Previous() produces the same boundaries (in reverse order) as the
// forward iteration via Next(). This is the AC1 parity test for rmp #4791.
//
// The test drives both Default.brk (word boundaries for Latin + CJK + Thai)
// and MyanmarSyllable.brk. For each corpus:
//  1. Collect all forward boundaries from Next().
//  2. Position the iterator at the end of the text.
//  3. Collect all boundaries going backwards via Previous().
//  4. Assert the reversed list equals the forward list.
func TestRBBIBreakIterator_Reverse_Parity(t *testing.T) {
	tests := []struct {
		brkName string
		text    string
		desc    string
	}{
		{
			brkName: "Default.brk",
			text:    "Hello, world! The quick brown fox.",
			desc:    "Latin words with punctuation",
		},
		{
			brkName: "Default.brk",
			text:    "日本語テスト",
			desc:    "CJK ideographs (per-character boundaries)",
		},
		{
			brkName: "MyanmarSyllable.brk",
			text:    "ကြည့်ရှုပါ",
			desc:    "Myanmar syllable boundaries",
		},
		{
			brkName: "Default.brk",
			text:    "foo bar baz",
			desc:    "simple whitespace-separated tokens",
		},
		{
			brkName: "Default.brk",
			text:    "abc",
			desc:    "single short word",
		},
		{
			brkName: "Default.brk",
			text:    "x",
			desc:    "single character",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			dict, err := LoadEmbeddedBRK(tc.brkName)
			if err != nil {
				t.Fatalf("LoadEmbeddedBRK(%s): %v", tc.brkName, err)
			}
			data, err := dict.RBBIData()
			if err != nil {
				t.Fatalf("RBBIData(%s): %v", tc.brkName, err)
			}

			r := []rune(tc.text)
			n := len(r)

			// 1. Forward pass: collect all boundaries.
			bi := newRBBIBreakIterator(data)
			bi.SetText(r, 0, n)
			var forward []int
			for {
				bp := bi.Next()
				if bp == Done {
					break
				}
				forward = append(forward, bp)
			}

			if len(forward) == 0 {
				t.Fatalf("%s: forward iteration returned no boundaries", tc.brkName)
			}

			// 2. Start at the last forward boundary.
			bi2 := newRBBIBreakIterator(data)
			bi2.SetText(r, 0, n)
			// Advance to end.
			bi2.position = forward[len(forward)-1]

			// 3. Backward pass: collect boundaries via Previous().
			var backward []int
			for {
				bp := bi2.Previous()
				if bp == Done {
					break
				}
				backward = append(backward, bp)
			}

			// 4. The expected backward sequence consists of all break boundaries
			//    before our start position (= forward's last element), in
			//    descending order.  The complete set of boundaries in the text
			//    is {0} ∪ forward.  The ones before `startPos = forward[n-1]`
			//    are {0} ∪ forward[0:n-1], so we expect them reversed:
			//    [forward[n-2], ..., forward[0], 0].
			startSet := make([]int, len(forward)) // [0, forward[0], ..., forward[n-2]]
			startSet[0] = 0
			for i, v := range forward[:len(forward)-1] {
				startSet[i+1] = v
			}
			// Reverse to get descending order.
			wantBack := make([]int, len(startSet))
			for i, v := range startSet {
				wantBack[len(startSet)-1-i] = v
			}

			if len(backward) != len(wantBack) {
				t.Errorf("%s: backward boundary count %d != expected %d\n  forward=%v\n  backward=%v",
					tc.brkName, len(backward), len(wantBack), forward, backward)
				return
			}
			for i, want := range wantBack {
				if backward[i] != want {
					t.Errorf("%s: backward[%d]=%d, want %d\n  forward=%v\n  backward=%v",
						tc.brkName, i, backward[i], want, forward, backward)
				}
			}
		})
	}
}

// TestRBBIBreakIterator_Previous_AtStart asserts that Previous() returns
// Done when the iterator is already at position 0.
func TestRBBIBreakIterator_Previous_AtStart(t *testing.T) {
	dict, err := LoadEmbeddedBRK("Default.brk")
	if err != nil {
		t.Fatalf("LoadEmbeddedBRK: %v", err)
	}
	data, err := dict.RBBIData()
	if err != nil {
		t.Fatalf("RBBIData: %v", err)
	}

	r := []rune("hello")
	bi := newRBBIBreakIterator(data)
	bi.SetText(r, 0, len(r))

	// At position 0, Previous() must return Done.
	if got := bi.Previous(); got != Done {
		t.Errorf("Previous() at position 0 = %d, want Done (%d)", got, Done)
	}
}
