// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestHeapPointReaderIteratesRange(t *testing.T) {
	source := []PointValue{
		&stubPointValue{packed: &util.BytesRef{Bytes: []byte{1}, Length: 1}, id: 10},
		&stubPointValue{packed: &util.BytesRef{Bytes: []byte{2}, Length: 1}, id: 20},
		&stubPointValue{packed: &util.BytesRef{Bytes: []byte{3}, Length: 1}, id: 30},
		&stubPointValue{packed: &util.BytesRef{Bytes: []byte{4}, Length: 1}, id: 40},
	}
	r := NewHeapPointReader(func(i int) PointValue { return source[i] }, 1, 3)
	var ids []int
	for {
		ok, err := r.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		ids = append(ids, r.PointValue().DocID())
	}
	want := []int{20, 30}
	if len(ids) != len(want) {
		t.Fatalf("len(ids)=%d want %d (%v)", len(ids), len(want), ids)
	}
	for i, id := range ids {
		if id != want[i] {
			t.Errorf("ids[%d]: got %d want %d", i, id, want[i])
		}
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestHeapPointReaderEmptyRange(t *testing.T) {
	r := NewHeapPointReader(func(i int) PointValue { return nil }, 5, 5)
	if ok, _ := r.Next(); ok {
		t.Fatalf("empty range: Next should return false")
	}
}
