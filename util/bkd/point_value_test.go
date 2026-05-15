// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubPointValue is a minimal in-test implementation that exercises
// the PointValue interface contract.
type stubPointValue struct {
	packed *util.BytesRef
	id     int
	combo  *util.BytesRef
}

func (s *stubPointValue) PackedValue() *util.BytesRef           { return s.packed }
func (s *stubPointValue) DocID() int                            { return s.id }
func (s *stubPointValue) PackedValueDocIDBytes() *util.BytesRef { return s.combo }

// TestPointValueInterfaceShape confirms that a concrete type can
// satisfy PointValue with the three documented accessors.
func TestPointValueInterfaceShape(t *testing.T) {
	packed := &util.BytesRef{Bytes: []byte{1, 2, 3, 4}, Offset: 0, Length: 4}
	combo := &util.BytesRef{Bytes: []byte{1, 2, 3, 4, 0, 0, 0, 42}, Offset: 0, Length: 8}
	var pv PointValue = &stubPointValue{packed: packed, id: 42, combo: combo}

	if pv.DocID() != 42 {
		t.Fatalf("DocID: got %d want 42", pv.DocID())
	}
	if got := pv.PackedValue(); got != packed {
		t.Fatalf("PackedValue: identity mismatch")
	}
	if got := pv.PackedValueDocIDBytes(); got != combo {
		t.Fatalf("PackedValueDocIDBytes: identity mismatch")
	}
}
