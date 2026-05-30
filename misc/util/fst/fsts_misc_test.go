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

	miscfst "github.com/FlavioCFOliveira/Gocene/misc/util/fst"
)

// TestFSTsMisc_RandomWords mirrors testRandomWords.
// Requires FSTTester, UpToTwoPositiveIntOutputs, and ListOfOutputs as full
// util/fst.Outputs[T] implementations — not yet ported.
func TestFSTsMisc_RandomWords(t *testing.T) {
	t.Fatal("requires FSTTester and full UpToTwoPositiveIntOutputs/ListOfOutputs — not yet ported")
}

// TestFSTsMisc_ListOfOutputs mirrors testListOfOutputs.
// Requires ListOfOutputs as a full util/fst.Outputs[any] and a working
// FSTCompiler/FST pipeline — not yet ported to misc/util/fst.
func TestFSTsMisc_ListOfOutputs(t *testing.T) {
	t.Fatal("requires ListOfOutputs as a full FST Outputs implementation — not yet ported")
}

// TestFSTsMisc_ListOfOutputsEmptyString mirrors testListOfOutputsEmptyString.
// Same dependency on ListOfOutputs + FSTCompiler as TestFSTsMisc_ListOfOutputs.
func TestFSTsMisc_ListOfOutputsEmptyString(t *testing.T) {
	t.Fatal("requires ListOfOutputs as a full FST Outputs implementation — not yet ported")
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
