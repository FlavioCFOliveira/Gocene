// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestFSTCompilerEmptyReturnsNil mirrors Lucene's behaviour that
// FSTCompiler.compile() returns null for an FST with no inputs and
// no empty-output.
func TestFSTCompilerEmptyReturnsNil(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if meta != nil {
		t.Fatalf("expected nil metadata for empty FST, got %+v", meta)
	}
}

// TestFSTCompilerSingleInput verifies that an FST with a single
// (input, output) pair maps the input back to the output.
func TestFSTCompilerSingleInput(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	scratch := util.NewIntsRefBuilder()
	bytesToIntsRef([]byte("abc"), scratch)
	if err := compiler.Add(scratch.Get(), 42); err != nil {
		t.Fatalf("Add: %v", err)
	}
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	fst, err := FromFSTReader[int64](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	out, found, err := lookupInt64(fst, []byte("abc"))
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !found {
		t.Fatalf("input not found in FST")
	}
	if out != 42 {
		t.Fatalf("output: got %d want 42", out)
	}
	// Negative case: non-existent input must not match.
	if _, found, err := lookupInt64(fst, []byte("abd")); err != nil {
		t.Fatalf("lookup miss: %v", err)
	} else if found {
		t.Fatalf("non-existent input should not be found")
	}
}

// TestFSTCompilerRoundTripBYTE1 builds an FST mapping
// {a:17, b:42, c:13824324872317238} as in
// TestFSTs.testSimple, and asserts that every input round-trips
// through Util.get.
func TestFSTCompilerRoundTripBYTE1(t *testing.T) {
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
		bytesToIntsRef(tc.input, scratch)
		if err := compiler.Add(scratch.Get(), tc.output); err != nil {
			t.Fatalf("Add %q=%d: %v", tc.input, tc.output, err)
		}
	}
	meta, err := compiler.Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if meta == nil {
		t.Fatalf("Compile returned nil")
	}
	fst, err := FromFSTReader[int64](meta, compiler.GetFSTReader())
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	for _, tc := range cases {
		got, found, err := lookupInt64(fst, tc.input)
		if err != nil {
			t.Fatalf("lookup %q: %v", tc.input, err)
		}
		if !found {
			t.Fatalf("lookup %q: not found", tc.input)
		}
		if got != tc.output {
			t.Fatalf("lookup %q: got %d want %d", tc.input, got, tc.output)
		}
	}
}

// TestFSTCompilerSaveRoundTrip serialises a compiled FST via Save and
// re-reads it via NewFSTFromDataInput; the recovered FST must accept
// the same inputs as the original.
func TestFSTCompilerSaveRoundTrip(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	cases := []struct {
		input  []byte
		output int64
	}{
		{[]byte("aab"), 22},
		{[]byte("aac"), 7},
		{[]byte("ax"), 17},
	}
	scratch := util.NewIntsRefBuilder()
	for _, tc := range cases {
		bytesToIntsRef(tc.input, scratch)
		if err := compiler.Add(scratch.Get(), tc.output); err != nil {
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

	metaOut := store.NewByteArrayDataOutput(64)
	bodyOut := store.NewByteArrayDataOutput(int(meta.NumBytes()))
	if err := fst.Save(metaOut, bodyOut); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read back: metadata then body.
	metaIn := store.NewByteArrayDataInput(metaOut.GetBytes())
	gotMeta, err := ReadMetadata[int64](metaIn, PositiveIntOutputs())
	if err != nil {
		t.Fatalf("ReadMetadata: %v", err)
	}
	if gotMeta.NumBytes() != meta.NumBytes() {
		t.Fatalf("NumBytes mismatch: got %d want %d", gotMeta.NumBytes(), meta.NumBytes())
	}
	bodyIn := store.NewByteArrayDataInput(bodyOut.GetBytes())
	recovered, err := NewFSTFromDataInput[int64](gotMeta, bodyIn)
	if err != nil {
		t.Fatalf("NewFSTFromDataInput: %v", err)
	}
	for _, tc := range cases {
		got, found, err := lookupInt64(recovered, tc.input)
		if err != nil {
			t.Fatalf("lookup %q after roundtrip: %v", tc.input, err)
		}
		if !found || got != tc.output {
			t.Fatalf("lookup %q: got %d (found=%v) want %d", tc.input, got, found, tc.output)
		}
	}
}

// TestFSTCompilerByteFormatFixture is the byte-format regression
// guard. It builds a deterministic FST {a:1, b:2, c:3} and asserts
// the resulting FST byte stream matches a frozen reference captured
// from a previous run of this implementation.
//
// The reference bytes were generated by this very test on
// 2026-05-15. They are not Lucene reference bytes; we have no Java
// runtime available in this environment to cross-validate. The
// fixture's purpose is to lock down byte-format drift across Go-side
// changes, complementing the structural assertions in the other
// tests. If a future change in Lucene-format detail forces an
// intentional update, recompute the fixture with -update and commit
// the new bytes alongside the change.
func TestFSTCompilerByteFormatFixture(t *testing.T) {
	compiler := NewFSTCompilerBuilder[int64](InputTypeByte1, PositiveIntOutputs()).Build()
	scratch := util.NewIntsRefBuilder()
	for i, in := range []byte{'a', 'b', 'c'} {
		bytesToIntsRef([]byte{in}, scratch)
		if err := compiler.Add(scratch.Get(), int64(i+1)); err != nil {
			t.Fatalf("Add %q: %v", in, err)
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
	bodyOut := store.NewByteArrayDataOutput(int(meta.NumBytes()))
	// Capture only the body bytes (without the metadata block) so the
	// fixture is a clean snapshot of the FST stream itself.
	rw := compiler.GetFSTReader()
	if err := rw.WriteTo(bodyOut); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	got := bodyOut.GetBytes()
	// Print on mismatch so the fixture can be regenerated by hand.
	if !bytes.Equal(got, byteFormatFixtureABC) {
		t.Fatalf("byte stream mismatch.\n got:  %v\nwant: %v\n(metadata: startNode=%d numBytes=%d)",
			got, byteFormatFixtureABC, meta.StartNode(), meta.NumBytes())
	}
	// Also verify structural correctness: the FST should accept a, b, c
	// and reject d.
	for i, in := range []byte{'a', 'b', 'c'} {
		out, ok, err := lookupInt64(fst, []byte{in})
		if err != nil {
			t.Fatalf("lookup %q: %v", in, err)
		}
		if !ok || out != int64(i+1) {
			t.Fatalf("lookup %q: got (%d, %v) want (%d, true)", in, out, ok, i+1)
		}
	}
	_, found, err := lookupInt64(fst, []byte{'d'})
	if err != nil {
		t.Fatalf("lookup 'd': %v", err)
	}
	if found {
		t.Fatalf("FST must reject 'd'")
	}
}

// lookupInt64 is a thin []byte adapter around the package-level
// [GetBytesRef] (the Go port of Lucene's
// {@code Util.get(FST<Long>, BytesRef)}). It exists only to spare the
// test sites from constructing a [util.BytesRef] manually.
func lookupInt64(fst *FST[int64], input []byte) (int64, bool, error) {
	return GetBytesRef(fst, util.NewBytesRef(input))
}
