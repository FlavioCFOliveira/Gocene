// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Go counterpart to
// lucene/core/src/test/org/apache/lucene/util/fst/TestUtil.java.
//
// The Java test peer exercises the binary-search and ceil-arc
// primitives over FSTs of single-character entries built with NoOutputs.
// These Go tests reproduce every Java test method one-for-one.

package fst

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestBinarySearch mirrors {@code TestUtil.testBinarySearch}.
//
// It builds an FST with 8 arcs spanning labels A..z that — given the
// configured spread — is encoded as a packed array requiring binary
// search, then asserts that BinarySearch returns the exact index for
// every label and the conventional {@code -1 - insertionPoint} for
// labels not present.
func TestBinarySearch(t *testing.T) {
	letters := []string{"A", "E", "J", "K", "L", "O", "T", "z"}
	fst := buildFSTForUtilTest(t, letters, true, false)

	arc := fst.GetFirstArc(&Arc[*noOutputMarker]{})
	in := fst.GetBytesReader()
	got, err := fst.ReadFirstTargetArc(arc, arc, in)
	if err != nil {
		t.Fatalf("ReadFirstTargetArc: %v", err)
	}
	arc = got

	for i, letter := range letters {
		idx, err := BinarySearch(fst, arc, int(letter[0]))
		if err != nil {
			t.Fatalf("BinarySearch(%q): %v", letter, err)
		}
		if idx != i {
			t.Fatalf("BinarySearch(%q): got %d want %d", letter, idx, i)
		}
	}
	// Before the first.
	if idx, err := BinarySearch(fst, arc, int(' ')); err != nil {
		t.Fatalf("BinarySearch(' '): %v", err)
	} else if idx != -1 {
		t.Fatalf("BinarySearch(' '): got %d want -1", idx)
	}
	// After the last.
	if idx, err := BinarySearch(fst, arc, int('~')); err != nil {
		t.Fatalf("BinarySearch('~'): %v", err)
	} else if want := -1 - len(letters); idx != want {
		t.Fatalf("BinarySearch('~'): got %d want %d", idx, want)
	}
	// In the middle: B and C both insert at position 1 (-2).
	if idx, err := BinarySearch(fst, arc, int('B')); err != nil {
		t.Fatalf("BinarySearch('B'): %v", err)
	} else if idx != -2 {
		t.Fatalf("BinarySearch('B'): got %d want -2", idx)
	}
	if idx, err := BinarySearch(fst, arc, int('C')); err != nil {
		t.Fatalf("BinarySearch('C'): %v", err)
	} else if idx != -2 {
		t.Fatalf("BinarySearch('C'): got %d want -2", idx)
	}
	// 'P' would insert at index 6 ('T' at 6, 'z' at 7): -1 - 6 = -7.
	if idx, err := BinarySearch(fst, arc, int('P')); err != nil {
		t.Fatalf("BinarySearch('P'): %v", err)
	} else if idx != -7 {
		t.Fatalf("BinarySearch('P'): got %d want -7", idx)
	}
}

// TestContinuous mirrors {@code TestUtil.testContinuous}.
//
// The eight labels A..H form a contiguous range, which the compiler
// encodes as a continuous-arc node. ReadCeilArc must succeed for every
// labelled arc, and querying a label past the last must return nil.
func TestContinuous(t *testing.T) {
	letters := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	fst := buildFSTForUtilTest(t, letters, true, false)

	first := fst.GetFirstArc(&Arc[*noOutputMarker]{})
	arc := &Arc[*noOutputMarker]{}
	in := fst.GetBytesReader()

	for _, letter := range letters {
		c := int(letter[0])
		got, err := ReadCeilArc(c, fst, first, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc(%q): %v", letter, err)
		}
		if got == nil {
			t.Fatalf("ReadCeilArc(%q): unexpected nil", letter)
		}
		arc = got
		if arc.Label() != c {
			t.Fatalf("ReadCeilArc(%q): label = %d want %d", letter, arc.Label(), c)
		}
	}
	// In the middle.
	{
		got, err := ReadCeilArc(int('F'), fst, first, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc('F'): %v", err)
		}
		if got == nil || got.Label() != int('F') {
			t.Fatalf("ReadCeilArc('F'): got %+v", got)
		}
	}
	// No following arcs from arc (we are now on 'F's target which has
	// no outgoing arcs because the FST accepts each single letter).
	{
		got, err := ReadCeilArc(int('A'), fst, arc, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc dead-end: %v", err)
		}
		if got != nil {
			t.Fatalf("ReadCeilArc dead-end: got %+v want nil", got)
		}
	}
}

// TestReadCeilArcPackedArray mirrors
// {@code TestUtil.testReadCeilArcPackedArray}.
func TestReadCeilArcPackedArray(t *testing.T) {
	letters := []string{"A", "E", "J", "K", "L", "O", "T", "z"}
	verifyReadCeilArc(t, letters, true, false)
}

// TestReadCeilArcArrayWithGaps mirrors
// {@code TestUtil.testReadCeilArcArrayWithGaps}.
//
// With direct addressing enabled (third arg true) the resulting node
// is a direct-addressing node spanning A..T with a presence-bit hole.
func TestReadCeilArcArrayWithGaps(t *testing.T) {
	letters := []string{"A", "E", "J", "K", "L", "O", "T"}
	verifyReadCeilArc(t, letters, true, true)
}

// TestReadCeilArcList mirrors {@code TestUtil.testReadCeilArcList}.
//
// With array arcs disabled the compiler falls back to the linear-scan
// list layout, which ReadCeilArc handles via the variable-length
// branch.
func TestReadCeilArcList(t *testing.T) {
	letters := []string{"A", "E", "J", "K", "L", "O", "T", "z"}
	verifyReadCeilArc(t, letters, false, false)
}

// verifyReadCeilArc mirrors the private Java helper
// {@code TestUtil.verifyReadCeilArc(List<String>, boolean, boolean)}.
func verifyReadCeilArc(t *testing.T, letters []string, allowArrayArcs, allowDirectAddressing bool) {
	t.Helper()
	fst := buildFSTForUtilTest(t, letters, allowArrayArcs, allowDirectAddressing)
	first := fst.GetFirstArc(&Arc[*noOutputMarker]{})
	arc := &Arc[*noOutputMarker]{}
	in := fst.GetBytesReader()

	for _, letter := range letters {
		c := int(letter[0])
		got, err := ReadCeilArc(c, fst, first, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc(%q): %v", letter, err)
		}
		if got == nil {
			t.Fatalf("ReadCeilArc(%q): unexpected nil", letter)
		}
		arc = got
		if arc.Label() != c {
			t.Fatalf("ReadCeilArc(%q): label = %d want %d", letter, arc.Label(), c)
		}
	}
	// Before the first → ceil to 'A'.
	{
		got, err := ReadCeilArc(int(' '), fst, first, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc(' '): %v", err)
		}
		if got == nil || got.Label() != int('A') {
			t.Fatalf("ReadCeilArc(' '): got %+v want label 'A'", got)
		}
	}
	// After the last → nil.
	{
		got, err := ReadCeilArc(int('~'), fst, first, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc('~'): %v", err)
		}
		if got != nil {
			t.Fatalf("ReadCeilArc('~'): got %+v want nil", got)
		}
	}
	// In the middle: 'F' ceils to 'J'.
	{
		got, err := ReadCeilArc(int('F'), fst, first, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc('F'): %v", err)
		}
		if got == nil || got.Label() != int('J') {
			t.Fatalf("ReadCeilArc('F'): got %+v want label 'J'", got)
		}
	}
	// No following arcs from arc.
	{
		got, err := ReadCeilArc(int('Z'), fst, arc, arc, in)
		if err != nil {
			t.Fatalf("ReadCeilArc dead-end: %v", err)
		}
		if got != nil {
			t.Fatalf("ReadCeilArc dead-end: got %+v want nil", got)
		}
	}
}

// buildFSTForUtilTest mirrors {@code TestUtil.buildFST(List<String>, boolean, boolean)}.
//
// Words must be supplied in strictly increasing order (the Java helper
// has the same precondition because FSTCompiler.add requires it).
func buildFSTForUtilTest(
	t *testing.T,
	words []string,
	allowArrayArcs, allowDirectAddressing bool,
) *FST[*noOutputMarker] {
	t.Helper()
	outputs := NoOutputs()
	builder := NewFSTCompilerBuilder[*noOutputMarker](InputTypeByte1, outputs).
		AllowFixedLengthArcs(allowArrayArcs)
	if !allowDirectAddressing {
		builder = builder.DirectAddressingMaxOversizingFactor(-1)
	}
	compiler := builder.Build()
	scratch := util.NewIntsRefBuilder()
	for _, w := range words {
		ToIntsRef(util.NewBytesRef([]byte(w)), scratch)
		if err := compiler.Add(scratch.Get(), outputs.GetNoOutput()); err != nil {
			t.Fatalf("Add %q: %v", w, err)
		}
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

// TestUtilGetIntsRef exercises Get on an FST mapping
// {a:17, b:42, c:13824324872317238}. The lookup must accept every
// input and reject unknowns.
func TestUtilGetIntsRef(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	cases := []struct {
		input  []byte
		output int64
	}{
		{[]byte("a"), 17},
		{[]byte("b"), 42},
		{[]byte("c"), 13824324872317238},
	}
	scratch := util.NewIntsRefBuilder()
	for _, tc := range cases {
		ToIntsRef(util.NewBytesRef(tc.input), scratch)
		if err := compiler.Add(scratch.Get(), tc.output); err != nil {
			t.Fatalf("Add %q: %v", tc.input, err)
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
	for _, tc := range cases {
		ToIntsRef(util.NewBytesRef(tc.input), scratch)
		got, ok, err := Get(fst, scratch.Get())
		if err != nil {
			t.Fatalf("Get %q: %v", tc.input, err)
		}
		if !ok {
			t.Fatalf("Get %q: not found", tc.input)
		}
		if got != tc.output {
			t.Fatalf("Get %q: got %d want %d", tc.input, got, tc.output)
		}
	}
	// Negative case.
	ToIntsRef(util.NewBytesRef([]byte("z")), scratch)
	if _, ok, err := Get(fst, scratch.Get()); err != nil {
		t.Fatalf("Get miss: %v", err)
	} else if ok {
		t.Fatalf("Get 'z' must not be found")
	}
}

// TestUtilGetBytesRef mirrors [TestUtilGetIntsRef] but exercises the
// BYTE1 specialisation that consumes a BytesRef directly.
func TestUtilGetBytesRef(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	cases := []struct {
		input  []byte
		output int64
	}{
		{[]byte("foo"), 1},
		{[]byte("foobar"), 2},
		{[]byte("foobaz"), 3},
	}
	scratch := util.NewIntsRefBuilder()
	for _, tc := range cases {
		ToIntsRef(util.NewBytesRef(tc.input), scratch)
		if err := compiler.Add(scratch.Get(), tc.output); err != nil {
			t.Fatalf("Add %q: %v", tc.input, err)
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
	for _, tc := range cases {
		got, ok, err := GetBytesRef(fst, util.NewBytesRef(tc.input))
		if err != nil {
			t.Fatalf("GetBytesRef %q: %v", tc.input, err)
		}
		if !ok || got != tc.output {
			t.Fatalf("GetBytesRef %q: got (%d, %v) want (%d, true)", tc.input, got, ok, tc.output)
		}
	}
	// Miss: same prefix but no match.
	if _, ok, err := GetBytesRef(fst, util.NewBytesRef([]byte("food"))); err != nil {
		t.Fatalf("GetBytesRef miss: %v", err)
	} else if ok {
		t.Fatalf("GetBytesRef food must not be found")
	}
}

// TestUtilToIntsRefRoundTrip checks ToIntsRef converts unsigned byte
// values without overflow, and ToBytesRef inverts it.
func TestUtilToIntsRefRoundTrip(t *testing.T) {
	src := []byte{0x00, 0x7F, 0x80, 0xFF, 'a', 'z'}
	intsScratch := util.NewIntsRefBuilder()
	intsRef := ToIntsRef(util.NewBytesRef(src), intsScratch)
	if intsRef.Length != len(src) {
		t.Fatalf("Length: got %d want %d", intsRef.Length, len(src))
	}
	for i, b := range src {
		want := int(b) & 0xFF
		if got := intsRef.Ints[intsRef.Offset+i]; got != want {
			t.Fatalf("int[%d]: got %d want %d", i, got, want)
		}
	}
	bytesScratch := util.NewBytesRefBuilder()
	bytesRef := ToBytesRef(intsRef, bytesScratch)
	if bytesRef.Length != len(src) {
		t.Fatalf("BytesRef length: got %d want %d", bytesRef.Length, len(src))
	}
	if !bytes.Equal(bytesRef.Bytes[bytesRef.Offset:bytesRef.Offset+bytesRef.Length], src) {
		t.Fatalf("round-trip bytes differ: got %v want %v",
			bytesRef.Bytes[bytesRef.Offset:bytesRef.Offset+bytesRef.Length], src)
	}
}

// TestUtilToBytesRefRejectsOutOfRange verifies the assertion-equivalent
// panic when an int does not fit in a byte.
func TestUtilToBytesRefRejectsOutOfRange(t *testing.T) {
	ir := &util.IntsRef{Ints: []int{300}, Offset: 0, Length: 1}
	scratch := util.NewBytesRefBuilder()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for value 300")
		}
	}()
	ToBytesRef(ir, scratch)
}

// TestUtilToUTF16 verifies UTF-16 code-unit mapping for an ASCII
// input (one code unit per byte) and for a surrogate-pair input.
func TestUtilToUTF16(t *testing.T) {
	scratch := util.NewIntsRefBuilder()
	got := ToUTF16("abc", scratch)
	want := []int{int('a'), int('b'), int('c')}
	if got.Length != len(want) {
		t.Fatalf("Length: got %d want %d", got.Length, len(want))
	}
	for i, w := range want {
		if got.Ints[i] != w {
			t.Fatalf("[%d]: got %d want %d", i, got.Ints[i], w)
		}
	}
	// "😀" is U+1F600, which is encoded as a UTF-16 surrogate pair
	// (D83D DE00).
	got = ToUTF16("\U0001F600", scratch)
	if got.Length != 2 {
		t.Fatalf("surrogate Length: got %d want 2", got.Length)
	}
	if got.Ints[0] != 0xD83D || got.Ints[1] != 0xDE00 {
		t.Fatalf("surrogate units: got %x %x want D83D DE00", got.Ints[0], got.Ints[1])
	}
}

// TestUtilToUTF32 verifies the code-point variant returns one int per
// rune even for non-BMP input.
func TestUtilToUTF32(t *testing.T) {
	scratch := util.NewIntsRefBuilder()
	got := ToUTF32("a\U0001F600b", scratch)
	want := []int{int('a'), 0x1F600, int('b')}
	if got.Length != len(want) {
		t.Fatalf("Length: got %d want %d", got.Length, len(want))
	}
	for i, w := range want {
		if got.Ints[i] != w {
			t.Fatalf("[%d]: got %x want %x", i, got.Ints[i], w)
		}
	}
}

// TestUtilToDotMinimal builds a tiny FST and asserts that ToDot
// produces a well-formed DOT string containing the expected prologue,
// state declarations, and edges.
//
// This is intentionally a coarse-grained sanity check: TestUtil.java
// has no peer for toDot beyond inspection-by-eye, so the exact byte
// output cannot be cross-validated against a Lucene-generated fixture.
// We verify the structural invariants the Java code is guaranteed to
// produce.
func TestUtilToDotMinimal(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	scratch := util.NewIntsRefBuilder()
	for i, w := range [][]byte{[]byte("a"), []byte("b"), []byte("c")} {
		ToIntsRef(util.NewBytesRef(w), scratch)
		if err := compiler.Add(scratch.Get(), int64(i+1)); err != nil {
			t.Fatalf("Add: %v", err)
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
	var buf bytes.Buffer
	if err := ToDot(fst, &buf, true, false); err != nil {
		t.Fatalf("ToDot: %v", err)
	}
	out := buf.String()
	// Prologue.
	if !bytes.Contains(buf.Bytes(), []byte("digraph FST {")) {
		t.Fatalf("missing digraph prologue: %s", out)
	}
	// node default style (labelStates=false).
	if !bytes.Contains(buf.Bytes(), []byte("node [shape=circle")) {
		t.Fatalf("missing default node style: %s", out)
	}
	// initial state placeholder.
	if !bytes.Contains(buf.Bytes(), []byte("initial [shape=point")) {
		t.Fatalf("missing initial state: %s", out)
	}
	// At least one transition labelled with 'a'.
	if !bytes.Contains(buf.Bytes(), []byte("label=\"a")) {
		t.Fatalf("missing 'a' transition: %s", out)
	}
	// Sink state.
	if !bytes.Contains(buf.Bytes(), []byte("{rank=sink; -1 }")) {
		t.Fatalf("missing sink rank: %s", out)
	}
	// Terminator.
	if !bytes.HasSuffix(buf.Bytes(), []byte("}\n")) {
		t.Fatalf("DOT must end with `}` + newline: %q", out[len(out)-3:])
	}
}

// TestUtilShortestPaths builds an FST {a:5, b:1, c:3, d:7} and asks
// for the top-2 paths under min-by-output ordering. The expected
// output is b (cost 1) then c (cost 3); a and d must be rejected as
// non-competitive once the queue is full.
func TestUtilShortestPaths(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	scratch := util.NewIntsRefBuilder()
	cases := []struct {
		in  []byte
		out int64
	}{
		{[]byte("a"), 5},
		{[]byte("b"), 1},
		{[]byte("c"), 3},
		{[]byte("d"), 7},
	}
	for _, c := range cases {
		ToIntsRef(util.NewBytesRef(c.in), scratch)
		if err := compiler.Add(scratch.Get(), c.out); err != nil {
			t.Fatalf("Add %q: %v", c.in, err)
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
	first := fst.GetFirstArc(&Arc[int64]{})
	cmp := func(a, b int64) int {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	}
	results, err := ShortestPaths(fst, first, fst.Outputs().GetNoOutput(), cmp, 2, false)
	if err != nil {
		t.Fatalf("ShortestPaths: %v", err)
	}
	if got := len(results.TopN); got != 2 {
		t.Fatalf("expected 2 results, got %d", got)
	}
	wantInputs := []byte{'b', 'c'}
	wantOutputs := []int64{1, 3}
	for i, r := range results.TopN {
		if r.Input.Length != 1 {
			t.Fatalf("result[%d]: input length = %d want 1", i, r.Input.Length)
		}
		if got := byte(r.Input.Ints[r.Input.Offset]); got != wantInputs[i] {
			t.Fatalf("result[%d]: input = %c want %c", i, got, wantInputs[i])
		}
		if r.Output != wantOutputs[i] {
			t.Fatalf("result[%d]: output = %d want %d", i, r.Output, wantOutputs[i])
		}
	}
	if !results.IsComplete {
		t.Fatalf("IsComplete: got false want true")
	}
}

// TestUtilShortestPathsAllowEmptyString builds an FST whose root
// accepts the empty string with a non-zero output, and verifies that
// allowEmptyString=true surfaces the empty path in the results.
func TestUtilShortestPathsAllowEmptyString(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	// Empty input must be added before any other input.
	if err := compiler.Add(&util.IntsRef{Ints: util.EmptyInts, Offset: 0, Length: 0}, 99); err != nil {
		t.Fatalf("Add empty: %v", err)
	}
	scratch := util.NewIntsRefBuilder()
	ToIntsRef(util.NewBytesRef([]byte("a")), scratch)
	if err := compiler.Add(scratch.Get(), 1); err != nil {
		t.Fatalf("Add 'a': %v", err)
	}
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	fst, err := FromFSTReader[int64](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	first := fst.GetFirstArc(&Arc[int64]{})
	cmp := func(a, b int64) int {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	}
	results, err := ShortestPaths(fst, first, fst.Outputs().GetNoOutput(), cmp, 2, true)
	if err != nil {
		t.Fatalf("ShortestPaths: %v", err)
	}
	// We expect the 'a' path (output 1, length 1) before the empty
	// path (output 99, length 0) under min-output ordering.
	if got := len(results.TopN); got != 2 {
		t.Fatalf("expected 2 results, got %d", got)
	}
	if results.TopN[0].Output != 1 || results.TopN[0].Input.Length != 1 {
		t.Fatalf("top[0]: got (out=%d len=%d) want (1, 1)", results.TopN[0].Output, results.TopN[0].Input.Length)
	}
	if results.TopN[1].Output != 99 || results.TopN[1].Input.Length != 0 {
		t.Fatalf("top[1]: got (out=%d len=%d) want (99, 0)", results.TopN[1].Output, results.TopN[1].Input.Length)
	}
}

// TestUtilPrintableLabel covers the ASCII/escape distinction used by
// the DOT exporter.
func TestUtilPrintableLabel(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{int('a'), "a"},
		{int('A'), "A"},
		{int(' '), " "},
		{int('}'), "}"},
		{int('"'), "0x22"},
		{int('\\'), "0x5c"},
		{0x1F, "0x1f"},
		{0x7E, "0x7e"},
		{0x80, "0x80"},
		{0x100, "0x100"},
	}
	for _, c := range cases {
		if got := printableLabel(c.in); got != c.want {
			t.Errorf("printableLabel(%d): got %q want %q", c.in, got, c.want)
		}
	}
}
