// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.misc.util.fst.TestFSTsMisc.
//
// The Java original depends on FSTTester (a heavy test-framework utility that
// builds and verifies full FST structures) and on UpToTwoPositiveIntOutputs /
// ListOfOutputs as complete util/fst.Outputs[T] implementations. Those are not
// yet ported to Gocene. Tests that require the full FST pipeline are explicitly
// skipped with a diagnostic message.
package fst_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
	corefst "github.com/FlavioCFOliveira/Gocene/util/fst"
	miscfst "github.com/FlavioCFOliveira/Gocene/misc/util/fst"
)

// lookupFST traverses an FST to find the output for a given byte string.
func lookupFST[T any](t *testing.T, fst *corefst.FST[T], word string) T {
	t.Helper()
	br := fst.GetBytesReader()
	arc := fst.GetFirstArc(new(corefst.Arc[T]))
	output := fst.Outputs().GetNoOutput()
	for _, r := range []byte(word) {
		next, err := fst.FindTargetArc(int(r), arc, arc, br)
		if err != nil || next == nil {
			var zero T
			return zero
		}
		output = fst.Outputs().Add(output, arc.Output())
	}
	if !arc.IsFinal() {
		var zero T
		return zero
	}
	return fst.Outputs().Add(output, arc.NextFinalOutput())
}

// TestFSTsMisc_RandomWords builds and queries FSTs with random-ish words
// using UpToTwoPositiveIntOutputs, verifying the full compile/query cycle.
func TestFSTsMisc_RandomWords(t *testing.T) {
	outputs := miscfst.GetUpToTwoPositiveIntOutputs(false)
	compiler := corefst.NewFSTCompilerBuilder[any](corefst.InputTypeByte1, outputs).Build()

	inputs := []struct {
		word   string
		output any
	}{
		{"cat", int64(1)},
		{"dog", int64(2)},
		{"elephant", int64(3)},
	}
	for _, in := range inputs {
		scratch := util.NewIntsRefBuilder()
		for _, r := range []byte(in.word) {
			scratch.Append(int(r))
		}
		if err := compiler.Add(scratch.Get(), in.output); err != nil {
			t.Fatalf("Add(%q): %v", in.word, err)
		}
	}
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	fst, err := corefst.FromFSTReader[any](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	if fst == nil {
		t.Fatal("FST is nil")
	}

	for _, in := range inputs {
		got := lookupFST[any](t, fst, in.word)
		if got == nil {
			t.Errorf("GetBytesRef(%q): not found", in.word)
			continue
		}
		gotVal, ok := got.(int64)
		if !ok {
			t.Errorf("GetBytesRef(%q): expected int64(%d), got %T(%v)", in.word, in.output, got, got)
			continue
		}
		if gotVal != in.output.(int64) {
			t.Errorf("GetBytesRef(%q): got %d, want %d", in.word, gotVal, in.output)
		}
	}
}

// TestFSTsMisc_ListOfOutputs builds an FST with ListOfOutputs, adding multiple
// outputs for the same key, then verifies the merged output is a []int64 slice.
func TestFSTsMisc_ListOfOutputs(t *testing.T) {
	inner := miscfst.GetUpToTwoPositiveIntOutputs(false)
	listOut := miscfst.NewListOfOutputs(inner)
	compiler := corefst.NewFSTCompilerBuilder[any](corefst.InputTypeByte1, listOut).Build()

	// Add multiple outputs for "a", merged into a list.
	scratch := util.NewIntsRefBuilder()
	for _, r := range []byte("a") {
		scratch.Append(int(r))
	}
	// First add: single output.
	if err := compiler.Add(scratch.Get(), int64(1)); err != nil {
		t.Fatalf("Add(a, 1): %v", err)
	}
	// Second add: merged via ListOfOutputs.Merge.
	if err := compiler.Add(scratch.Get(), int64(3)); err != nil {
		t.Fatalf("Add(a, 3): %v", err)
	}
	// Third add.
	if err := compiler.Add(scratch.Get(), int64(0)); err != nil {
		t.Fatalf("Add(a, 0): %v", err)
	}

	// Add a single-output key.
	scratchB := util.NewIntsRefBuilder()
	for _, r := range []byte("b") {
		scratchB.Append(int(r))
	}
	if err := compiler.Add(scratchB.Get(), int64(17)); err != nil {
		t.Fatalf("Add(b, 17): %v", err)
	}

	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	fst, err := corefst.FromFSTReader[any](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	if fst == nil {
		t.Fatal("FST is nil")
	}

	// Verify "a" returns a []int64 with [1 3 0].
	gotA := lookupFST[any](t, fst, "a")
	if gotA == nil {
		t.Fatal("GetBytesRef(a): not found")
	}
	list, ok := gotA.([]int64)
	if !ok {
		t.Fatalf("a: expected []int64, got %T(%v)", gotA, gotA)
	}
	if len(list) != 3 {
		t.Fatalf("a: expected 3 outputs, got %d: %v", len(list), list)
	}
	if list[0] != 1 || list[1] != 3 || list[2] != 0 {
		t.Fatalf("a: expected [1 3 0], got %v", list)
	}

	gotB := lookupFST[any](t, fst, "b")
	if gotB == nil {
		t.Fatal("GetBytesRef(b): not found")
	}
	bVal, ok := gotB.(int64)
	if !ok {
		t.Fatalf("b: expected int64, got %T(%v)", gotB, gotB)
	}
	if bVal != 17 {
		t.Fatalf("b: expected [17], got %v", bVal)
	}
}

// TestFSTsMisc_ListOfOutputsEmptyString builds an FST with ListOfOutputs
// where the empty string is an input.
func TestFSTsMisc_ListOfOutputsEmptyString(t *testing.T) {
	inner := miscfst.GetUpToTwoPositiveIntOutputs(false)
	listOut := miscfst.NewListOfOutputs(inner)
	compiler := corefst.NewFSTCompilerBuilder[any](corefst.InputTypeByte1, listOut).Build()

	// Empty input.
	scratch := util.NewIntsRefBuilder()
	if err := compiler.Add(scratch.Get(), int64(42)); err != nil {
		t.Fatalf("Add(empty, 42): %v", err)
	}

	// Non-empty input to verify non-empty path still works.
	scratchB := util.NewIntsRefBuilder()
	for _, r := range []byte("x") {
		scratchB.Append(int(r))
	}
	if err := compiler.Add(scratchB.Get(), int64(7)); err != nil {
		t.Fatalf("Add(x, 7): %v", err)
	}

	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	fst, err := corefst.FromFSTReader[any](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	if fst == nil {
		t.Fatal("FST is nil")
	}

	got := lookupFST[any](t, fst, "")
	if got == nil {
		t.Fatal("GetBytesRef(empty): not found")
	}
	v, ok := got.(int64)
	if !ok {
		t.Fatalf("empty: expected int64, got %T(%v)", got, got)
	}
	if v != 42 {
		t.Fatalf("empty: expected 42, got %d", v)
	}
}

// ---------------------------------------------------------------------------
// Smoke tests for the current misc/util/fst stubs
// ---------------------------------------------------------------------------

// TestUpToTwoPositiveIntOutputs_Encode verifies that UpToTwoPositiveIntOutputs
// encodes and decodes single and paired values correctly.
func TestUpToTwoPositiveIntOutputs_Encode(t *testing.T) {
	o := miscfst.UpToTwoPositiveIntOutputs{}

	single := o.Encode([]int64{42})
	got := o.Decode(single)
	if len(got) != 1 || got[0] != 42 {
		t.Errorf("single encode/decode: expected [42], got %v", got)
	}

	pair := o.Encode([]int64{7, 13})
	got = o.Decode(pair)
	if len(got) != 2 || got[0] != 7 || got[1] != 13 {
		t.Errorf("pair encode/decode: expected [7 13], got %v", got)
	}
}

// TestListOfOutputs_Add verifies the ListOfOutputs.Add merge logic.
func TestListOfOutputs_Add(t *testing.T) {
	lo := miscfst.ListOfOutputs{}

	// Two empty slices.
	if result := lo.Add(nil, nil); len(result) != 0 {
		t.Errorf("Add(nil,nil) expected empty, got %v", result)
	}

	a := [][]byte{[]byte("foo"), []byte("bar")}
	b := [][]byte{[]byte("baz")}

	merged := lo.Add(a, b)
	if len(merged) != 3 {
		t.Fatalf("Add len: expected 3, got %d", len(merged))
	}
	if string(merged[0]) != "foo" || string(merged[1]) != "bar" || string(merged[2]) != "baz" {
		t.Errorf("Add content mismatch: %v", merged)
	}
}

// TestUpToTwoPositiveIntOutputs_ZeroValue verifies that encoding 0 round-trips
// correctly (0 is the NO_OUTPUT sentinel in the Java implementation, but encoding
// must still produce valid bytes).
func TestUpToTwoPositiveIntOutputs_ZeroValue(t *testing.T) {
	o := miscfst.UpToTwoPositiveIntOutputs{}
	enc := o.Encode([]int64{0})
	got := o.Decode(enc)
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("zero encode/decode: expected [0], got %v", got)
	}
}
