// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst_test

import (
	"testing"

	miscfst "github.com/FlavioCFOliveira/Gocene/misc/util/fst"
)

// TestUpToTwoPositiveIntOutputs_Encode verifies encode/decode round-trip.
func TestUpToTwoPositiveIntOutputs_Encode(t *testing.T) {
	o := miscfst.UpToTwoPositiveIntOutputs{}

	single := o.Encode([]int64{42})
	got := o.Decode(single)
	if len(got) != 1 || got[0] != 42 {
		t.Errorf("single: expected [42], got %v", got)
	}

	pair := o.Encode([]int64{7, 13})
	got = o.Decode(pair)
	if len(got) != 2 || got[0] != 7 || got[1] != 13 {
		t.Errorf("pair: expected [7 13], got %v", got)
	}

	zero := o.Encode([]int64{0})
	got = o.Decode(zero)
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("zero: expected [0], got %v", got)
	}
}

// TestListOfOutputs_Add verifies merge logic.
func TestListOfOutputs_Add(t *testing.T) {
	lo := miscfst.ListOfOutputs{}

	// Both nil
	if result := lo.Add(nil, nil); len(result) != 0 {
		t.Errorf("Add(nil,nil): expected empty, got %v", result)
	}

	// a nil, b non-nil
	result := lo.Add(nil, [][]byte{[]byte("foo")})
	if len(result) != 1 || string(result[0]) != "foo" {
		t.Errorf("Add(nil,[foo]): expected [foo], got %v", result)
	}

	// Both non-nil
	a := [][]byte{[]byte("a"), []byte("b")}
	b := [][]byte{[]byte("c")}
	result = lo.Add(a, b)
	if len(result) != 3 || string(result[0]) != "a" || string(result[1]) != "b" || string(result[2]) != "c" {
		t.Errorf("Add: expected [a b c], got %v", result)
	}
}
