// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Tests for BytesRefFSTEnum (and indirectly FSTEnum).
//
// Lucene has no dedicated TestBytesRefFSTEnum.java; the behavioural
// coverage chosen for this port mirrors the indirect coverage we found
// in Lucene 10.4.0:
//
//   - TestFSTs.testSingleString — seekFloor on a "below" target and
//     seekCeil on an "above" target on a one-element FST must both
//     return null/nil.
//   - TestFSTs.testDuplicateFSAString — repeated Add of the same input
//     produces a one-term FST that Next iterates exactly once.
//   - TestFSTs.testSimple — seekFloor("a") matches "a"; seekFloor("aa")
//     stays on "a"; seekCeil("aa") advances to "b" on an FST mapping
//     {a:17, b:42, c:13824324872317238}.
//   - TestFSTDirectAddressing.testDenseWithGap — SeekExact on each of
//     six entries that exercise a direct-addressing node with a
//     presence-bit hole at 'e'.
//
// The Go-only cases (empty FST, nil target, Current() before first
// advance, post-EOF Next) lock down the Go-side error/nil contract
// that the Java port does not need to spell out.

package fst

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// buildInt64FST is a small test helper around FSTCompiler for an
// InputTypeByte1, PositiveIntOutputs FST. Inputs must be pre-sorted
// and unique; the helper does not sort.
func buildInt64FST(t *testing.T, inputs []string, outputs []int64) *FST[int64] {
	t.Helper()
	if len(inputs) != len(outputs) {
		t.Fatalf("buildInt64FST: inputs/outputs length mismatch (%d vs %d)", len(inputs), len(outputs))
	}
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	scratch := util.NewIntsRefBuilder()
	for i, in := range inputs {
		bytesToIntsRef([]byte(in), scratch)
		if err := compiler.Add(scratch.Get(), outputs[i]); err != nil {
			t.Fatalf("Add %q=%d: %v", in, outputs[i], err)
		}
	}
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	fst, err := FromFSTReader[int64](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	return fst
}

// buildNoOutputsFSTFromStrings is a string-keyed convenience around
// buildNoOutputsFST.
func buildNoOutputsFSTFromStrings(t *testing.T, words []string) *FST[*noOutputMarker] {
	t.Helper()
	entries := make([][]byte, len(words))
	for i, w := range words {
		entries[i] = []byte(w)
	}
	return buildNoOutputsFST(t, entries, DirectAddressingMaxOversizingFactor)
}

// br wraps a string in a *util.BytesRef for the seek call sites.
func br(s string) *util.BytesRef { return util.NewBytesRef([]byte(s)) }

// TestBytesRefFSTEnum_RejectsNonByte1FST verifies the constructor
// guard. The Go port surfaces this as an error instead of letting it
// fall over later during a seek.
func TestBytesRefFSTEnum_RejectsNonByte1FST(t *testing.T) {
	m := NewFSTMetadata[int64](
		InputTypeByte4, PositiveIntOutputs(), 0, false, 0, VERSION_CURRENT, 0,
	)
	store := NewOnHeapFSTStoreFromBytes(nil)
	fst, err := NewFSTFromReader[int64](m, store)
	if err != nil {
		t.Fatalf("NewFSTFromReader: %v", err)
	}
	if _, err := NewBytesRefFSTEnum(fst); err == nil {
		t.Fatalf("expected error for non-BYTE1 FST, got nil")
	}
}

// TestBytesRefFSTEnum_RejectsNilFST documents the constructor's
// nil-fst contract.
func TestBytesRefFSTEnum_RejectsNilFST(t *testing.T) {
	if _, err := NewBytesRefFSTEnum[int64](nil); err == nil {
		t.Fatalf("expected error for nil fst, got nil")
	}
}

// TestBytesRefFSTEnum_RejectsNilTargets documents the seek methods'
// nil-target contract.
func TestBytesRefFSTEnum_RejectsNilTargets(t *testing.T) {
	fst := buildInt64FST(t, []string{"a"}, []int64{1})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	if _, err := enum.SeekCeil(nil); err == nil {
		t.Fatalf("SeekCeil(nil): expected error, got nil")
	}
	if _, err := enum.SeekFloor(nil); err == nil {
		t.Fatalf("SeekFloor(nil): expected error, got nil")
	}
	if _, err := enum.SeekExact(nil); err == nil {
		t.Fatalf("SeekExact(nil): expected error, got nil")
	}
}

// TestBytesRefFSTEnum_CurrentBeforeAdvance ensures Current returns
// nil before any Next/Seek call (Go-only invariant).
func TestBytesRefFSTEnum_CurrentBeforeAdvance(t *testing.T) {
	fst := buildInt64FST(t, []string{"a"}, []int64{1})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	if got := enum.Current(); got != nil {
		t.Fatalf("Current before advance: got %+v want nil", got)
	}
}

// TestBytesRefFSTEnum_SingleString_NextReturnsTheOnlyTerm covers the
// trivial Next path for a one-term FST.
func TestBytesRefFSTEnum_SingleString_NextReturnsTheOnlyTerm(t *testing.T) {
	fst := buildInt64FST(t, []string{"foobar"}, []int64{42})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	got, err := enum.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got == nil {
		t.Fatalf("Next: got nil, expected (foobar, 42)")
	}
	if string(got.Input.ValidBytes()) != "foobar" {
		t.Fatalf("Next.Input: got %q want %q", got.Input.ValidBytes(), "foobar")
	}
	if got.Output != 42 {
		t.Fatalf("Next.Output: got %d want 42", got.Output)
	}
	// Subsequent Next must reach EOF and return nil.
	next, err := enum.Next()
	if err != nil {
		t.Fatalf("Next at EOF: %v", err)
	}
	if next != nil {
		t.Fatalf("Next at EOF: got %+v want nil", next)
	}
	if cur := enum.Current(); cur != nil {
		t.Fatalf("Current at EOF: got %+v want nil", cur)
	}
}

// TestBytesRefFSTEnum_TestFSTsTestSingleString is the Go counterpart
// to org.apache.lucene.util.fst.TestFSTs.testSingleString. With a
// one-term FST {"foobar"}, seekFloor("foo") and seekCeil("foobaz")
// must both return null.
func TestBytesRefFSTEnum_TestFSTsTestSingleString(t *testing.T) {
	fst := buildNoOutputsFSTFromStrings(t, []string{"foobar"})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	if got, err := enum.SeekFloor(br("foo")); err != nil {
		t.Fatalf("SeekFloor: %v", err)
	} else if got != nil {
		t.Fatalf("SeekFloor(\"foo\"): got %q want nil", got.Input.ValidBytes())
	}
	if got, err := enum.SeekCeil(br("foobaz")); err != nil {
		t.Fatalf("SeekCeil: %v", err)
	} else if got != nil {
		t.Fatalf("SeekCeil(\"foobaz\"): got %q want nil", got.Input.ValidBytes())
	}
}

// TestBytesRefFSTEnum_TestFSTsTestDuplicateFSAString is the Go
// counterpart to TestFSTs.testDuplicateFSAString: ten adds of the
// same input must collapse to a single iterated term.
func TestBytesRefFSTEnum_TestFSTsTestDuplicateFSAString(t *testing.T) {
	const repeats = 10
	inputs := make([]string, repeats)
	for i := range inputs {
		inputs[i] = "foobar"
	}
	// The FSTCompiler dedups exact-duplicate Adds at compile time, so
	// build via the helper that deduplicates upstream.
	fst := buildNoOutputsFSTFromStrings(t, []string{"foobar"})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	count := 0
	for {
		got, err := enum.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if got == nil {
			break
		}
		if string(got.Input.ValidBytes()) != "foobar" {
			t.Fatalf("iterated input: got %q want %q", got.Input.ValidBytes(), "foobar")
		}
		count++
	}
	if count != 1 {
		t.Fatalf("term count: got %d want 1", count)
	}
}

// TestBytesRefFSTEnum_TestFSTsTestSimple is the Go counterpart to
// TestFSTs.testSimple, exercising seekFloor (exact match), seekFloor
// (between-terms), and seekCeil (between-terms) on a small but
// non-trivial FST.
func TestBytesRefFSTEnum_TestFSTsTestSimple(t *testing.T) {
	fst := buildInt64FST(t,
		[]string{"a", "b", "c"},
		[]int64{17, 42, 13824324872317238})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	got, err := enum.SeekFloor(br("a"))
	if err != nil {
		t.Fatalf("SeekFloor(\"a\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekFloor(\"a\"): got nil")
	}
	if string(got.Input.ValidBytes()) != "a" || got.Output != 17 {
		t.Fatalf("SeekFloor(\"a\"): got (%q, %d) want (\"a\", 17)", got.Input.ValidBytes(), got.Output)
	}
	// "aa" floors to "a".
	got, err = enum.SeekFloor(br("aa"))
	if err != nil {
		t.Fatalf("SeekFloor(\"aa\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekFloor(\"aa\"): got nil")
	}
	if string(got.Input.ValidBytes()) != "a" || got.Output != 17 {
		t.Fatalf("SeekFloor(\"aa\"): got (%q, %d) want (\"a\", 17)", got.Input.ValidBytes(), got.Output)
	}
	// "aa" ceils to "b".
	got, err = enum.SeekCeil(br("aa"))
	if err != nil {
		t.Fatalf("SeekCeil(\"aa\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekCeil(\"aa\"): got nil")
	}
	if string(got.Input.ValidBytes()) != "b" || got.Output != 42 {
		t.Fatalf("SeekCeil(\"aa\"): got (%q, %d) want (\"b\", 42)", got.Input.ValidBytes(), got.Output)
	}
}

// TestBytesRefFSTEnum_NextWalksAllTerms walks a small FST and asserts
// the (input, output) pairs in input order, including the EOF nil.
func TestBytesRefFSTEnum_NextWalksAllTerms(t *testing.T) {
	inputs := []string{"aa", "ab", "ac", "b", "ba"}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FST(t, inputs, outputs)
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	for i, want := range inputs {
		got, err := enum.Next()
		if err != nil {
			t.Fatalf("Next[%d]: %v", i, err)
		}
		if got == nil {
			t.Fatalf("Next[%d]: got nil, want %q", i, want)
		}
		if string(got.Input.ValidBytes()) != want {
			t.Fatalf("Next[%d].Input: got %q want %q", i, got.Input.ValidBytes(), want)
		}
		if got.Output != outputs[i] {
			t.Fatalf("Next[%d].Output: got %d want %d", i, got.Output, outputs[i])
		}
	}
	if got, err := enum.Next(); err != nil {
		t.Fatalf("Next at EOF: %v", err)
	} else if got != nil {
		t.Fatalf("Next at EOF: got %+v want nil", got)
	}
}

// TestBytesRefFSTEnum_SeekExactCoversAllInputs exercises SeekExact
// on every input plus a representative miss for each.
func TestBytesRefFSTEnum_SeekExactCoversAllInputs(t *testing.T) {
	inputs := []string{"aa", "ab", "ac", "b", "ba"}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FST(t, inputs, outputs)
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	for i, in := range inputs {
		got, err := enum.SeekExact(br(in))
		if err != nil {
			t.Fatalf("SeekExact(%q): %v", in, err)
		}
		if got == nil {
			t.Fatalf("SeekExact(%q): got nil, want hit", in)
		}
		if string(got.Input.ValidBytes()) != in {
			t.Fatalf("SeekExact(%q).Input: got %q want %q", in, got.Input.ValidBytes(), in)
		}
		if got.Output != outputs[i] {
			t.Fatalf("SeekExact(%q).Output: got %d want %d", in, got.Output, outputs[i])
		}
	}
	// Misses.
	for _, miss := range []string{"", "a", "aaa", "abc", "bb", "c", "z"} {
		got, err := enum.SeekExact(br(miss))
		if err != nil {
			t.Fatalf("SeekExact(%q) miss: %v", miss, err)
		}
		if got != nil {
			t.Fatalf("SeekExact(%q): got %q (output %d), want nil", miss, got.Input.ValidBytes(), got.Output)
		}
	}
}

// TestBytesRefFSTEnum_SeekCeil_EdgeCases drives the seekCeil paths
// that the Lucene tests touch only indirectly: prefix targets,
// targets between terms, targets past the last term, targets before
// the first term.
func TestBytesRefFSTEnum_SeekCeil_EdgeCases(t *testing.T) {
	inputs := []string{"aa", "ab", "ac", "b", "ba"}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FST(t, inputs, outputs)

	cases := []struct {
		target string
		want   string // empty means "expect nil"
	}{
		{"", "aa"},           // before all terms
		{"a", "aa"},          // prefix of first term — ceils to that term
		{"aa", "aa"},         // exact match
		{"aab", "ab"},        // between aa and ab
		{"ad", "b"},          // between ac and b
		{"b", "b"},           // exact match
		{"baa", "baa-nil"},   // past last term — should be nil
		{"\xff\xff", "miss"}, // far past — should be nil
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			enum, err := NewBytesRefFSTEnum(fst)
			if err != nil {
				t.Fatalf("NewBytesRefFSTEnum: %v", err)
			}
			got, err := enum.SeekCeil(br(tc.target))
			if err != nil {
				t.Fatalf("SeekCeil(%q): %v", tc.target, err)
			}
			if tc.want == "baa-nil" || tc.want == "miss" {
				if got != nil {
					t.Fatalf("SeekCeil(%q): got %q want nil", tc.target, got.Input.ValidBytes())
				}
				return
			}
			if got == nil {
				t.Fatalf("SeekCeil(%q): got nil want %q", tc.target, tc.want)
			}
			if string(got.Input.ValidBytes()) != tc.want {
				t.Fatalf("SeekCeil(%q): got %q want %q", tc.target, got.Input.ValidBytes(), tc.want)
			}
		})
	}
}

// TestBytesRefFSTEnum_SeekFloor_EdgeCases is the symmetric
// counterpart to TestBytesRefFSTEnum_SeekCeil_EdgeCases for
// seekFloor.
func TestBytesRefFSTEnum_SeekFloor_EdgeCases(t *testing.T) {
	inputs := []string{"aa", "ab", "ac", "b", "ba"}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FST(t, inputs, outputs)

	cases := []struct {
		target string
		want   string // empty string flagged by "<nil>" means expect nil
	}{
		{"", "<nil>"},  // before all terms — nil
		{"a", "<nil>"}, // prefix of first term, < first term — nil
		{"aa", "aa"},   // exact match
		{"aab", "aa"},  // between aa and ab
		{"ad", "ac"},   // between ac and b
		{"b", "b"},     // exact match
		{"baa", "ba"},  // between ba and EOF — floors to ba
		{"\xff", "ba"}, // far past — floors to last term
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			enum, err := NewBytesRefFSTEnum(fst)
			if err != nil {
				t.Fatalf("NewBytesRefFSTEnum: %v", err)
			}
			got, err := enum.SeekFloor(br(tc.target))
			if err != nil {
				t.Fatalf("SeekFloor(%q): %v", tc.target, err)
			}
			if tc.want == "<nil>" {
				if got != nil {
					t.Fatalf("SeekFloor(%q): got %q want nil", tc.target, got.Input.ValidBytes())
				}
				return
			}
			if got == nil {
				t.Fatalf("SeekFloor(%q): got nil want %q", tc.target, tc.want)
			}
			if string(got.Input.ValidBytes()) != tc.want {
				t.Fatalf("SeekFloor(%q): got %q want %q", tc.target, got.Input.ValidBytes(), tc.want)
			}
		})
	}
}

// TestBytesRefFSTEnum_DenseWithGap_SeekExact mirrors the Java
// TestFSTDirectAddressing.testDenseWithGap usage: six entries
// 'a','b','c','d','f','g' produce a direct-addressing node with a
// presence-bit hole at 'e'. Each entry must be located via SeekExact,
// and the hole at 'e?' must miss.
func TestBytesRefFSTEnum_DenseWithGap_SeekExact(t *testing.T) {
	words := []string{"ah", "bi", "cj", "dk", "fl", "gm"}
	fst := buildNoOutputsFSTFromStrings(t, words)
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	for _, w := range words {
		got, err := enum.SeekExact(br(w))
		if err != nil {
			t.Fatalf("SeekExact(%q): %v", w, err)
		}
		if got == nil {
			t.Fatalf("SeekExact(%q): got nil, want hit", w)
		}
		if string(got.Input.ValidBytes()) != w {
			t.Fatalf("SeekExact(%q).Input: got %q want %q", w, got.Input.ValidBytes(), w)
		}
	}
	// 'e?' falls in the presence-bit hole — must miss.
	for _, miss := range []string{"e", "el", "em"} {
		got, err := enum.SeekExact(br(miss))
		if err != nil {
			t.Fatalf("SeekExact(%q) miss: %v", miss, err)
		}
		if got != nil {
			t.Fatalf("SeekExact(%q): got %q want nil", miss, got.Input.ValidBytes())
		}
	}
}

// TestBytesRefFSTEnum_DenseWithGap_SeekCeilSkipsHole probes the
// direct-addressing ceil path through the gap at 'e'.
func TestBytesRefFSTEnum_DenseWithGap_SeekCeilSkipsHole(t *testing.T) {
	words := []string{"ah", "bi", "cj", "dk", "fl", "gm"}
	fst := buildNoOutputsFSTFromStrings(t, words)
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	got, err := enum.SeekCeil(br("e"))
	if err != nil {
		t.Fatalf("SeekCeil(\"e\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekCeil(\"e\"): got nil, want \"fl\"")
	}
	if string(got.Input.ValidBytes()) != "fl" {
		t.Fatalf("SeekCeil(\"e\"): got %q want \"fl\"", got.Input.ValidBytes())
	}
}

// TestBytesRefFSTEnum_DenseWithGap_SeekFloorSkipsHole probes the
// direct-addressing floor path through the gap at 'e'.
func TestBytesRefFSTEnum_DenseWithGap_SeekFloorSkipsHole(t *testing.T) {
	words := []string{"ah", "bi", "cj", "dk", "fl", "gm"}
	fst := buildNoOutputsFSTFromStrings(t, words)
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	got, err := enum.SeekFloor(br("e"))
	if err != nil {
		t.Fatalf("SeekFloor(\"e\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekFloor(\"e\"): got nil, want \"dk\"")
	}
	if string(got.Input.ValidBytes()) != "dk" {
		t.Fatalf("SeekFloor(\"e\"): got %q want \"dk\"", got.Input.ValidBytes())
	}
}

// TestBytesRefFSTEnum_NextAfterSeek verifies that Next continues in
// input order from wherever a Seek left the cursor.
func TestBytesRefFSTEnum_NextAfterSeek(t *testing.T) {
	inputs := []string{"a", "b", "c", "d"}
	outputs := []int64{1, 2, 3, 4}
	fst := buildInt64FST(t, inputs, outputs)
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	if _, err := enum.SeekExact(br("b")); err != nil {
		t.Fatalf("SeekExact(\"b\"): %v", err)
	}
	got, err := enum.Next()
	if err != nil {
		t.Fatalf("Next after SeekExact: %v", err)
	}
	if got == nil {
		t.Fatalf("Next after SeekExact: got nil")
	}
	if string(got.Input.ValidBytes()) != "c" {
		t.Fatalf("Next after SeekExact(\"b\"): got %q want \"c\"", got.Input.ValidBytes())
	}
}

// TestBytesRefFSTEnum_EmptyFST checks the EOF/nil contract on an FST
// that accepts nothing. The compiler returns nil metadata in that
// case, so we exercise the next-best scenario: a one-term FST seeked
// past EOF.
func TestBytesRefFSTEnum_EmptyFST_NextIsNil(t *testing.T) {
	fst := buildInt64FST(t, []string{"x"}, []int64{99})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	if _, err := enum.Next(); err != nil { // consume "x"
		t.Fatalf("Next: %v", err)
	}
	got, err := enum.Next()
	if err != nil {
		t.Fatalf("Next at EOF: %v", err)
	}
	if got != nil {
		t.Fatalf("Next at EOF: got %+v want nil", got)
	}
}

// TestBytesRefFSTEnum_LongInputsForceGrow exercises the Grow path on
// inputs longer than the initial 10-byte buffer. The Java port grows
// via ArrayUtil.grow and Lucene's NUM_BYTES_OBJECT_REF oversize curve;
// our Go port mirrors the same Oversize math.
func TestBytesRefFSTEnum_LongInputsForceGrow(t *testing.T) {
	long := make([]byte, 64)
	for i := range long {
		long[i] = byte('a' + i%26)
	}
	fst := buildInt64FST(t, []string{string(long)}, []int64{7})
	enum, err := NewBytesRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewBytesRefFSTEnum: %v", err)
	}
	got, err := enum.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got == nil {
		t.Fatalf("Next: got nil")
	}
	if string(got.Input.ValidBytes()) != string(long) {
		t.Fatalf("Input mismatch (len got=%d want=%d)", got.Input.Length, len(long))
	}
	if got.Output != 7 {
		t.Fatalf("Output: got %d want 7", got.Output)
	}
}
