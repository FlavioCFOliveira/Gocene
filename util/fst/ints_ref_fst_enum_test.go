// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Tests for IntsRefFSTEnum (and indirectly FSTEnum) — sibling to
// bytes_ref_fst_enum_test.go.
//
// Lucene has no dedicated TestIntsRefFSTEnum.java; the behavioural
// coverage chosen for this port mirrors the indirect coverage found
// in Lucene 10.4.0:
//
//   - Test2BFST and Test2BFSTOffHeap: iterate every term via
//     IntsRefFSTEnum<...>.next() and assert pair contents.
//   - TestFSTDirectAddressing.recompile / walk: iterate a BYTE4 FST
//     over CharsRef outputs using IntsRefFSTEnum.
//
// The cases below also re-run the BytesRefFSTEnum behavioural shape
// (single-string seekFloor/seekCeil, simple ascending FST,
// dense-with-gap direct-addressing, long-input grow) but in IntsRef
// form. They are joined by one BYTE2 and one BYTE4 case that
// IntsRefFSTEnum uniquely supports.

package fst

import (
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ir wraps a string in a *util.IntsRef whose Ints are the unsigned
// byte values of s. Equivalent to ir = bytesToIntsRef(s) but inline
// for short test call sites.
func ir(s string) *util.IntsRef {
	if len(s) == 0 {
		return &util.IntsRef{Ints: util.EmptyInts, Offset: 0, Length: 0}
	}
	ints := make([]int, len(s))
	for i := 0; i < len(s); i++ {
		ints[i] = int(s[i])
	}
	return &util.IntsRef{Ints: ints, Offset: 0, Length: len(ints)}
}

// irFromInts builds a *util.IntsRef directly over the given int
// values (no UTF-8 / byte interpretation). Used for BYTE2/BYTE4 cases
// where labels exceed 0xFF.
func irFromInts(values ...int) *util.IntsRef {
	cp := make([]int, len(values))
	copy(cp, values)
	return &util.IntsRef{Ints: cp, Offset: 0, Length: len(cp)}
}

// buildInt64FSTByte1FromIntsRefs is a small test helper around
// FSTCompiler for an InputTypeByte1, PositiveIntOutputs FST whose
// inputs are already provided as IntsRefs. Inputs must be pre-sorted
// and unique; the helper does not sort.
func buildInt64FSTByte1FromIntsRefs(t *testing.T, inputs []*util.IntsRef, outputs []int64) *FST[int64] {
	t.Helper()
	if len(inputs) != len(outputs) {
		t.Fatalf("buildInt64FSTByte1FromIntsRefs: inputs/outputs length mismatch (%d vs %d)", len(inputs), len(outputs))
	}
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	for i, in := range inputs {
		if err := compiler.Add(in, outputs[i]); err != nil {
			t.Fatalf("Add %v=%d: %v", in.ValidInts(), outputs[i], err)
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

// buildInt64FSTWithInputType is the BYTE2/BYTE4-aware sibling of
// buildInt64FSTByte1FromIntsRefs. Inputs must be sorted ascending
// over CompareTo and unique.
func buildInt64FSTWithInputType(t *testing.T, inputType InputType, inputs []*util.IntsRef, outputs []int64) *FST[int64] {
	t.Helper()
	if len(inputs) != len(outputs) {
		t.Fatalf("buildInt64FSTWithInputType: inputs/outputs length mismatch (%d vs %d)", len(inputs), len(outputs))
	}
	compiler := NewFSTCompilerBuilder[int64](inputType, PositiveIntOutputs()).Build()
	for i, in := range inputs {
		if err := compiler.Add(in, outputs[i]); err != nil {
			t.Fatalf("Add[%d]: %v", i, err)
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

// buildNoOutputsFSTFromIntsRefs is the IntsRef-flavoured analogue of
// buildNoOutputsFSTFromStrings.
func buildNoOutputsFSTFromIntsRefs(t *testing.T, entries []*util.IntsRef) *FST[*noOutputMarker] {
	t.Helper()
	compiler := NewFSTCompilerBuilder[*noOutputMarker](
		InputTypeByte1, NoOutputs(),
	).DirectAddressingMaxOversizingFactor(DirectAddressingMaxOversizingFactor).Build()

	sorted := append([]*util.IntsRef(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool { return util.IntsRefCompare(sorted[i], sorted[j]) < 0 })
	var last *util.IntsRef
	for _, e := range sorted {
		if last != nil && util.IntsRefEquals(e, last) {
			continue
		}
		if err := compiler.Add(e, NoOutputValue()); err != nil {
			t.Fatalf("Add: %v", err)
		}
		last = e
	}
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if meta == nil {
		t.Fatalf("Compile returned nil metadata")
	}
	fst, err := FromFSTReader[*noOutputMarker](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	return fst
}

// TestIntsRefFSTEnum_RejectsNilFST documents the constructor's
// nil-fst contract.
func TestIntsRefFSTEnum_RejectsNilFST(t *testing.T) {
	if _, err := NewIntsRefFSTEnum[int64](nil); err == nil {
		t.Fatalf("expected error for nil fst, got nil")
	}
}

// TestIntsRefFSTEnum_RejectsNilTargets documents the seek methods'
// nil-target contract.
func TestIntsRefFSTEnum_RejectsNilTargets(t *testing.T) {
	fst := buildInt64FSTByte1FromIntsRefs(t, []*util.IntsRef{ir("a")}, []int64{1})
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
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

// TestIntsRefFSTEnum_CurrentBeforeAdvance ensures Current returns
// nil before any Next/Seek call (Go-only invariant).
func TestIntsRefFSTEnum_CurrentBeforeAdvance(t *testing.T) {
	fst := buildInt64FSTByte1FromIntsRefs(t, []*util.IntsRef{ir("a")}, []int64{1})
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	if got := enum.Current(); got != nil {
		t.Fatalf("Current before advance: got %+v want nil", got)
	}
}

// TestIntsRefFSTEnum_AcceptsAllInputTypes covers the constructor on
// each of BYTE1, BYTE2, BYTE4 — the Go enumerator must work over any
// input width, mirroring Java's IntsRefFSTEnum.
func TestIntsRefFSTEnum_AcceptsAllInputTypes(t *testing.T) {
	for _, it := range []InputType{InputTypeByte1, InputTypeByte2, InputTypeByte4} {
		t.Run(it.String(), func(t *testing.T) {
			m := NewFSTMetadata[int64](
				it, PositiveIntOutputs(), 0, false, 0, VERSION_CURRENT, 0,
			)
			store := NewOnHeapFSTStoreFromBytes(nil)
			fst, err := NewFSTFromReader[int64](m, store)
			if err != nil {
				t.Fatalf("NewFSTFromReader: %v", err)
			}
			if _, err := NewIntsRefFSTEnum(fst); err != nil {
				t.Fatalf("NewIntsRefFSTEnum: %v", err)
			}
		})
	}
}

// TestIntsRefFSTEnum_SingleEntry_NextReturnsTheOnlyTerm covers the
// trivial Next path for a one-term FST.
func TestIntsRefFSTEnum_SingleEntry_NextReturnsTheOnlyTerm(t *testing.T) {
	fst := buildInt64FSTByte1FromIntsRefs(t, []*util.IntsRef{ir("foobar")}, []int64{42})
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	got, err := enum.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got == nil {
		t.Fatalf("Next: got nil, expected (foobar, 42)")
	}
	if !util.IntsRefEquals(got.Input, ir("foobar")) {
		t.Fatalf("Next.Input: got %v want %v", got.Input.ValidInts(), ir("foobar").ValidInts())
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

// TestIntsRefFSTEnum_TestFSTsTestSingleString mirrors the Java
// TestFSTs.testSingleString. With a one-term FST {"foobar"},
// seekFloor("foo") and seekCeil("foobaz") must both return null.
func TestIntsRefFSTEnum_TestFSTsTestSingleString(t *testing.T) {
	fst := buildNoOutputsFSTFromIntsRefs(t, []*util.IntsRef{ir("foobar")})
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	if got, err := enum.SeekFloor(ir("foo")); err != nil {
		t.Fatalf("SeekFloor: %v", err)
	} else if got != nil {
		t.Fatalf("SeekFloor(\"foo\"): got %v want nil", got.Input.ValidInts())
	}
	if got, err := enum.SeekCeil(ir("foobaz")); err != nil {
		t.Fatalf("SeekCeil: %v", err)
	} else if got != nil {
		t.Fatalf("SeekCeil(\"foobaz\"): got %v want nil", got.Input.ValidInts())
	}
}

// TestIntsRefFSTEnum_TestFSTsTestSimple mirrors TestFSTs.testSimple
// — seekFloor (exact match), seekFloor (between-terms), and seekCeil
// (between-terms) on a small but non-trivial FST.
func TestIntsRefFSTEnum_TestFSTsTestSimple(t *testing.T) {
	fst := buildInt64FSTByte1FromIntsRefs(t,
		[]*util.IntsRef{ir("a"), ir("b"), ir("c")},
		[]int64{17, 42, 13824324872317238})
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	got, err := enum.SeekFloor(ir("a"))
	if err != nil {
		t.Fatalf("SeekFloor(\"a\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekFloor(\"a\"): got nil")
	}
	if !util.IntsRefEquals(got.Input, ir("a")) || got.Output != 17 {
		t.Fatalf("SeekFloor(\"a\"): got (%v, %d) want (\"a\", 17)", got.Input.ValidInts(), got.Output)
	}
	// "aa" floors to "a".
	got, err = enum.SeekFloor(ir("aa"))
	if err != nil {
		t.Fatalf("SeekFloor(\"aa\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekFloor(\"aa\"): got nil")
	}
	if !util.IntsRefEquals(got.Input, ir("a")) || got.Output != 17 {
		t.Fatalf("SeekFloor(\"aa\"): got (%v, %d) want (\"a\", 17)", got.Input.ValidInts(), got.Output)
	}
	// "aa" ceils to "b".
	got, err = enum.SeekCeil(ir("aa"))
	if err != nil {
		t.Fatalf("SeekCeil(\"aa\"): %v", err)
	}
	if got == nil {
		t.Fatalf("SeekCeil(\"aa\"): got nil")
	}
	if !util.IntsRefEquals(got.Input, ir("b")) || got.Output != 42 {
		t.Fatalf("SeekCeil(\"aa\"): got (%v, %d) want (\"b\", 42)", got.Input.ValidInts(), got.Output)
	}
}

// TestIntsRefFSTEnum_NextWalksAllTerms walks a small FST and asserts
// the (input, output) pairs in input order, including the EOF nil.
func TestIntsRefFSTEnum_NextWalksAllTerms(t *testing.T) {
	inputs := []*util.IntsRef{ir("aa"), ir("ab"), ir("ac"), ir("b"), ir("ba")}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FSTByte1FromIntsRefs(t, inputs, outputs)
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	for i, want := range inputs {
		got, err := enum.Next()
		if err != nil {
			t.Fatalf("Next[%d]: %v", i, err)
		}
		if got == nil {
			t.Fatalf("Next[%d]: got nil, want %v", i, want.ValidInts())
		}
		if !util.IntsRefEquals(got.Input, want) {
			t.Fatalf("Next[%d].Input: got %v want %v", i, got.Input.ValidInts(), want.ValidInts())
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

// TestIntsRefFSTEnum_SeekExactCoversAllInputs exercises SeekExact on
// every input plus a representative miss for each.
func TestIntsRefFSTEnum_SeekExactCoversAllInputs(t *testing.T) {
	inputs := []*util.IntsRef{ir("aa"), ir("ab"), ir("ac"), ir("b"), ir("ba")}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FSTByte1FromIntsRefs(t, inputs, outputs)
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	for i, in := range inputs {
		got, err := enum.SeekExact(in)
		if err != nil {
			t.Fatalf("SeekExact(%v): %v", in.ValidInts(), err)
		}
		if got == nil {
			t.Fatalf("SeekExact(%v): got nil, want hit", in.ValidInts())
		}
		if !util.IntsRefEquals(got.Input, in) {
			t.Fatalf("SeekExact(%v).Input: got %v want %v", in.ValidInts(), got.Input.ValidInts(), in.ValidInts())
		}
		if got.Output != outputs[i] {
			t.Fatalf("SeekExact(%v).Output: got %d want %d", in.ValidInts(), got.Output, outputs[i])
		}
	}
	// Misses.
	for _, miss := range []*util.IntsRef{ir(""), ir("a"), ir("aaa"), ir("abc"), ir("bb"), ir("c"), ir("z")} {
		got, err := enum.SeekExact(miss)
		if err != nil {
			t.Fatalf("SeekExact(%v) miss: %v", miss.ValidInts(), err)
		}
		if got != nil {
			t.Fatalf("SeekExact(%v): got %v (output %d), want nil", miss.ValidInts(), got.Input.ValidInts(), got.Output)
		}
	}
}

// TestIntsRefFSTEnum_SeekCeil_EdgeCases drives the seekCeil paths
// that the Lucene tests touch only indirectly: prefix targets,
// targets between terms, targets past the last term, targets before
// the first term.
func TestIntsRefFSTEnum_SeekCeil_EdgeCases(t *testing.T) {
	inputs := []*util.IntsRef{ir("aa"), ir("ab"), ir("ac"), ir("b"), ir("ba")}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FSTByte1FromIntsRefs(t, inputs, outputs)

	cases := []struct {
		name   string
		target *util.IntsRef
		want   *util.IntsRef // nil means "expect nil"
	}{
		{"before-all", ir(""), ir("aa")},
		{"prefix-of-first", ir("a"), ir("aa")},
		{"exact-aa", ir("aa"), ir("aa")},
		{"between-aa-ab", ir("aab"), ir("ab")},
		{"between-ac-b", ir("ad"), ir("b")},
		{"exact-b", ir("b"), ir("b")},
		{"past-last", ir("baa"), nil},
		{"far-past", ir("\xff\xff"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enum, err := NewIntsRefFSTEnum(fst)
			if err != nil {
				t.Fatalf("NewIntsRefFSTEnum: %v", err)
			}
			got, err := enum.SeekCeil(tc.target)
			if err != nil {
				t.Fatalf("SeekCeil(%v): %v", tc.target.ValidInts(), err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("SeekCeil(%v): got %v want nil", tc.target.ValidInts(), got.Input.ValidInts())
				}
				return
			}
			if got == nil {
				t.Fatalf("SeekCeil(%v): got nil want %v", tc.target.ValidInts(), tc.want.ValidInts())
			}
			if !util.IntsRefEquals(got.Input, tc.want) {
				t.Fatalf("SeekCeil(%v): got %v want %v", tc.target.ValidInts(), got.Input.ValidInts(), tc.want.ValidInts())
			}
		})
	}
}

// TestIntsRefFSTEnum_SeekFloor_EdgeCases is the symmetric
// counterpart to TestIntsRefFSTEnum_SeekCeil_EdgeCases for
// seekFloor.
func TestIntsRefFSTEnum_SeekFloor_EdgeCases(t *testing.T) {
	inputs := []*util.IntsRef{ir("aa"), ir("ab"), ir("ac"), ir("b"), ir("ba")}
	outputs := []int64{1, 2, 3, 4, 5}
	fst := buildInt64FSTByte1FromIntsRefs(t, inputs, outputs)

	cases := []struct {
		name   string
		target *util.IntsRef
		want   *util.IntsRef // nil means "expect nil"
	}{
		{"before-all", ir(""), nil},
		{"prefix-of-first", ir("a"), nil},
		{"exact-aa", ir("aa"), ir("aa")},
		{"between-aa-ab", ir("aab"), ir("aa")},
		{"between-ac-b", ir("ad"), ir("ac")},
		{"exact-b", ir("b"), ir("b")},
		{"between-ba-eof", ir("baa"), ir("ba")},
		{"far-past", ir("\xff"), ir("ba")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enum, err := NewIntsRefFSTEnum(fst)
			if err != nil {
				t.Fatalf("NewIntsRefFSTEnum: %v", err)
			}
			got, err := enum.SeekFloor(tc.target)
			if err != nil {
				t.Fatalf("SeekFloor(%v): %v", tc.target.ValidInts(), err)
			}
			if tc.want == nil {
				if got != nil {
					t.Fatalf("SeekFloor(%v): got %v want nil", tc.target.ValidInts(), got.Input.ValidInts())
				}
				return
			}
			if got == nil {
				t.Fatalf("SeekFloor(%v): got nil want %v", tc.target.ValidInts(), tc.want.ValidInts())
			}
			if !util.IntsRefEquals(got.Input, tc.want) {
				t.Fatalf("SeekFloor(%v): got %v want %v", tc.target.ValidInts(), got.Input.ValidInts(), tc.want.ValidInts())
			}
		})
	}
}

// TestIntsRefFSTEnum_DenseWithGap_SeekExact mirrors the Java
// TestFSTDirectAddressing.testDenseWithGap usage: six entries
// 'a','b','c','d','f','g' produce a direct-addressing node with a
// presence-bit hole at 'e'. Each entry must be located via
// SeekExact, and the hole at 'e?' must miss.
func TestIntsRefFSTEnum_DenseWithGap_SeekExact(t *testing.T) {
	words := []*util.IntsRef{ir("ah"), ir("bi"), ir("cj"), ir("dk"), ir("fl"), ir("gm")}
	fst := buildNoOutputsFSTFromIntsRefs(t, words)
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	for _, w := range words {
		got, err := enum.SeekExact(w)
		if err != nil {
			t.Fatalf("SeekExact(%v): %v", w.ValidInts(), err)
		}
		if got == nil {
			t.Fatalf("SeekExact(%v): got nil, want hit", w.ValidInts())
		}
		if !util.IntsRefEquals(got.Input, w) {
			t.Fatalf("SeekExact(%v).Input: got %v want %v", w.ValidInts(), got.Input.ValidInts(), w.ValidInts())
		}
	}
	// 'e?' falls in the presence-bit hole — must miss.
	for _, miss := range []*util.IntsRef{ir("e"), ir("el"), ir("em")} {
		got, err := enum.SeekExact(miss)
		if err != nil {
			t.Fatalf("SeekExact(%v) miss: %v", miss.ValidInts(), err)
		}
		if got != nil {
			t.Fatalf("SeekExact(%v): got %v want nil", miss.ValidInts(), got.Input.ValidInts())
		}
	}
}

// TestIntsRefFSTEnum_NextAfterSeek verifies that Next continues in
// input order from wherever a Seek left the cursor.
func TestIntsRefFSTEnum_NextAfterSeek(t *testing.T) {
	inputs := []*util.IntsRef{ir("a"), ir("b"), ir("c"), ir("d")}
	outputs := []int64{1, 2, 3, 4}
	fst := buildInt64FSTByte1FromIntsRefs(t, inputs, outputs)
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	if _, err := enum.SeekExact(ir("b")); err != nil {
		t.Fatalf("SeekExact(\"b\"): %v", err)
	}
	got, err := enum.Next()
	if err != nil {
		t.Fatalf("Next after SeekExact: %v", err)
	}
	if got == nil {
		t.Fatalf("Next after SeekExact: got nil")
	}
	if !util.IntsRefEquals(got.Input, ir("c")) {
		t.Fatalf("Next after SeekExact(\"b\"): got %v want \"c\"", got.Input.ValidInts())
	}
}

// TestIntsRefFSTEnum_LongInputsForceGrow exercises the Grow path on
// inputs longer than the initial 10-int buffer.
func TestIntsRefFSTEnum_LongInputsForceGrow(t *testing.T) {
	longLen := 64
	longBytes := make([]int, longLen)
	for i := range longBytes {
		longBytes[i] = int('a') + (i % 26)
	}
	longRef := &util.IntsRef{Ints: longBytes, Offset: 0, Length: longLen}
	fst := buildInt64FSTByte1FromIntsRefs(t, []*util.IntsRef{longRef}, []int64{7})
	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	got, err := enum.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got == nil {
		t.Fatalf("Next: got nil")
	}
	if got.Input.Length != longLen {
		t.Fatalf("Input length: got %d want %d", got.Input.Length, longLen)
	}
	for i := 0; i < longLen; i++ {
		want := longBytes[i]
		gotV := got.Input.Ints[got.Input.Offset+i]
		if gotV != want {
			t.Fatalf("Input[%d]: got %d want %d", i, gotV, want)
		}
	}
	if got.Output != 7 {
		t.Fatalf("Output: got %d want 7", got.Output)
	}
}

// TestIntsRefFSTEnum_Byte2_MultiByteLabels exercises a BYTE2 FST
// whose labels are unsigned 16-bit ints — beyond the 0..255 range
// that BytesRefFSTEnum can carry. This locks down the unique
// advantage of IntsRefFSTEnum over BytesRefFSTEnum: it can iterate
// FSTs with wide labels.
func TestIntsRefFSTEnum_Byte2_MultiByteLabels(t *testing.T) {
	inputs := []*util.IntsRef{
		irFromInts(0x0041, 0x0042),         // 'A','B'
		irFromInts(0x0041, 0x4E2D),         // 'A',中
		irFromInts(0x4E2D, 0x6587),         // 中,文
		irFromInts(0xFFFE),                 // single wide label, near MAX_UINT16
		irFromInts(0xFFFF, 0x0001, 0x0002), // MAX_UINT16-anchored term
	}
	outputs := []int64{10, 11, 12, 13, 14}
	fst := buildInt64FSTWithInputType(t, InputTypeByte2, inputs, outputs)

	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	for i, want := range inputs {
		got, err := enum.Next()
		if err != nil {
			t.Fatalf("Next[%d]: %v", i, err)
		}
		if got == nil {
			t.Fatalf("Next[%d]: got nil, want %v", i, want.ValidInts())
		}
		if !util.IntsRefEquals(got.Input, want) {
			t.Fatalf("Next[%d].Input: got %v want %v", i, got.Input.ValidInts(), want.ValidInts())
		}
		if got.Output != outputs[i] {
			t.Fatalf("Next[%d].Output: got %d want %d", i, got.Output, outputs[i])
		}
	}
	if got, err := enum.Next(); err != nil {
		t.Fatalf("Next at EOF: %v", err)
	} else if got != nil {
		t.Fatalf("Next at EOF: got %v want nil", got.Input.ValidInts())
	}
	// SeekExact roundtrip — every input must be found.
	for i, in := range inputs {
		got, err := enum.SeekExact(in)
		if err != nil {
			t.Fatalf("SeekExact[%d]: %v", i, err)
		}
		if got == nil {
			t.Fatalf("SeekExact[%d]: got nil, want hit", i)
		}
		if got.Output != outputs[i] {
			t.Fatalf("SeekExact[%d].Output: got %d want %d", i, got.Output, outputs[i])
		}
	}
	// SeekExact miss for a label that doesn't appear at the root.
	miss := irFromInts(0x0042) // 'B' alone — there is no 'B' prefix
	got, err := enum.SeekExact(miss)
	if err != nil {
		t.Fatalf("SeekExact(miss): %v", err)
	}
	if got != nil {
		t.Fatalf("SeekExact(miss): got %v want nil", got.Input.ValidInts())
	}
}

// TestIntsRefFSTEnum_Byte4_LargeLabels exercises a BYTE4 FST whose
// labels are values that don't fit in 16 bits. BYTE4 inputs are
// VInt-encoded in the FST byte stream; iterating them via
// IntsRefFSTEnum must return labels equal to the originally-added
// int values, regardless of the VInt encoding.
func TestIntsRefFSTEnum_Byte4_LargeLabels(t *testing.T) {
	inputs := []*util.IntsRef{
		irFromInts(0x00010000, 0x00010001), // 65536, 65537
		irFromInts(0x00010000, 0x00020000), // 65536, 131072
		irFromInts(0x00100000),             // 1048576 (single label)
		irFromInts(0x7FFFFFFE),             // near INT32_MAX
		irFromInts(0x7FFFFFFF, 0x00000001), // INT32_MAX-anchored term
	}
	outputs := []int64{20, 21, 22, 23, 24}
	fst := buildInt64FSTWithInputType(t, InputTypeByte4, inputs, outputs)

	enum, err := NewIntsRefFSTEnum(fst)
	if err != nil {
		t.Fatalf("NewIntsRefFSTEnum: %v", err)
	}
	for i, want := range inputs {
		got, err := enum.Next()
		if err != nil {
			t.Fatalf("Next[%d]: %v", i, err)
		}
		if got == nil {
			t.Fatalf("Next[%d]: got nil, want %v", i, want.ValidInts())
		}
		if !util.IntsRefEquals(got.Input, want) {
			t.Fatalf("Next[%d].Input: got %v want %v", i, got.Input.ValidInts(), want.ValidInts())
		}
		if got.Output != outputs[i] {
			t.Fatalf("Next[%d].Output: got %d want %d", i, got.Output, outputs[i])
		}
	}
	if got, err := enum.Next(); err != nil {
		t.Fatalf("Next at EOF: %v", err)
	} else if got != nil {
		t.Fatalf("Next at EOF: got %v want nil", got.Input.ValidInts())
	}
	// SeekCeil to a value between the second and third inputs.
	target := irFromInts(0x00010000, 0x00030000) // > inputs[1], < inputs[2]
	got, err := enum.SeekCeil(target)
	if err != nil {
		t.Fatalf("SeekCeil: %v", err)
	}
	if got == nil {
		t.Fatalf("SeekCeil: got nil, want inputs[2]")
	}
	if !util.IntsRefEquals(got.Input, inputs[2]) {
		t.Fatalf("SeekCeil.Input: got %v want %v", got.Input.ValidInts(), inputs[2].ValidInts())
	}
	if got.Output != outputs[2] {
		t.Fatalf("SeekCeil.Output: got %d want %d", got.Output, outputs[2])
	}
	// SeekFloor symmetric: floor of the same target gives inputs[1].
	got, err = enum.SeekFloor(target)
	if err != nil {
		t.Fatalf("SeekFloor: %v", err)
	}
	if got == nil {
		t.Fatalf("SeekFloor: got nil, want inputs[1]")
	}
	if !util.IntsRefEquals(got.Input, inputs[1]) {
		t.Fatalf("SeekFloor.Input: got %v want %v", got.Input.ValidInts(), inputs[1].ValidInts())
	}
}
