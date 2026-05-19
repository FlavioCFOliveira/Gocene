// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package automaton

import (
	"reflect"
	"testing"
)

func TestFrozenIntSet_ConstructorRetainsFields(t *testing.T) {
	vals := []int32{1, 4, 9}
	f := NewFrozenIntSet(vals, 0xCAFEBABE, 42)

	if got := f.Size(); got != 3 {
		t.Errorf("Size: got %d, want 3", got)
	}
	if got := f.LongHashCode(); got != 0xCAFEBABE {
		t.Errorf("LongHashCode: got %#x, want 0xCAFEBABE", got)
	}
	if got := f.State; got != 42 {
		t.Errorf("State: got %d, want 42", got)
	}
	// GetArray must return the same backing slice (no copy), mirroring
	// Lucene's FrozenIntSet.getArray returning the values field as-is.
	if got := f.GetArray(); !reflect.DeepEqual(got, vals) {
		t.Errorf("GetArray contents: got %v, want %v", got, vals)
	}
	if &f.GetArray()[0] != &vals[0] {
		t.Error("GetArray must alias the constructor input (no defensive copy)")
	}
}

func TestFrozenIntSet_IntSetConformance(t *testing.T) {
	// IntSetEquals must treat a FrozenIntSet and an equivalent StateSet
	// as equal once both expose the same sorted contents.
	src := NewStateSet(0)
	src.Incr(7)
	src.Incr(3)
	src.Incr(11)

	f := src.Freeze(-1)
	if !IntSetEquals(f, src) {
		t.Fatal("IntSetEquals(frozen, source): got false, want true")
	}
	if got, want := f.GetArray(), []int32{3, 7, 11}; !reflect.DeepEqual(got, want) {
		t.Errorf("frozen GetArray: got %v, want %v", got, want)
	}
}

func TestFrozenIntSet_String(t *testing.T) {
	// Format must match java.util.Arrays.toString: "[]", "[v]", "[v1, v2, ...]".
	cases := []struct {
		in   []int32
		want string
	}{
		{nil, "[]"},
		{[]int32{}, "[]"},
		{[]int32{7}, "[7]"},
		{[]int32{-1, 0, 42}, "[-1, 0, 42]"},
	}
	for _, tc := range cases {
		got := NewFrozenIntSet(tc.in, 0, 0).String()
		if got != tc.want {
			t.Errorf("String(%v): got %q, want %q", tc.in, got, tc.want)
		}
	}
}
