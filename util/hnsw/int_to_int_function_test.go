// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import "testing"

func TestIntToIntFuncAdapter(t *testing.T) {
	var f IntToIntFunction = IntToIntFunc(func(v int) int { return v * 2 })
	if got := f.Apply(21); got != 42 {
		t.Fatalf("Apply: got %d want 42", got)
	}
}

type doubler struct{}

func (doubler) Apply(v int) int { return v * 2 }

func TestIntToIntFunctionStructImpl(t *testing.T) {
	var f IntToIntFunction = doubler{}
	if got := f.Apply(7); got != 14 {
		t.Fatalf("Apply: got %d want 14", got)
	}
}
