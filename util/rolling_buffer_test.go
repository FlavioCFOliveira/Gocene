// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

type rbCell struct {
	pos   int
	dirty bool
}

func (c *rbCell) Reset() {
	c.dirty = false
	c.pos = -1
}

func newRBCells() *RollingBuffer[*rbCell] {
	return NewRollingBuffer[*rbCell](func() *rbCell { return &rbCell{pos: -1} })
}

func TestRollingBuffer_GetAssignsPosition(t *testing.T) {
	rb := newRBCells()
	for i := 0; i < 5; i++ {
		c := rb.Get(i)
		c.pos = i
		c.dirty = true
	}
	if got := rb.MaxPos(); got != 4 {
		t.Fatalf("MaxPos()=%d want 4", got)
	}
	if got := rb.BufferSize(); got != 5 {
		t.Fatalf("BufferSize()=%d want 5", got)
	}
	for i := 0; i < 5; i++ {
		if rb.Get(i).pos != i {
			t.Fatalf("Get(%d).pos=%d", i, rb.Get(i).pos)
		}
	}
}

func TestRollingBuffer_GrowBeyondInitialCapacity(t *testing.T) {
	rb := newRBCells()
	for i := 0; i < 20; i++ {
		rb.Get(i).pos = i
	}
	if got := rb.BufferSize(); got != 20 {
		t.Fatalf("BufferSize=%d want 20", got)
	}
	for i := 0; i < 20; i++ {
		if got := rb.Get(i).pos; got != i {
			t.Fatalf("Get(%d).pos=%d", i, got)
		}
	}
}

func TestRollingBuffer_FreeBeforeRecyclesSlots(t *testing.T) {
	rb := newRBCells()
	for i := 0; i < 10; i++ {
		c := rb.Get(i)
		c.pos = i
		c.dirty = true
	}
	rb.FreeBefore(6)
	if got := rb.BufferSize(); got != 4 {
		t.Fatalf("BufferSize=%d want 4", got)
	}
	for i := 6; i < 10; i++ {
		if rb.Get(i).pos != i {
			t.Fatalf("post-free Get(%d) lost original pos", i)
		}
	}
}

func TestRollingBuffer_GetFarFuturePosition(t *testing.T) {
	rb := newRBCells()
	c := rb.Get(100)
	c.pos = 100
	if rb.Get(100).pos != 100 {
		t.Fatalf("future position lost")
	}
	if rb.BufferSize() != 101 {
		t.Fatalf("BufferSize=%d want 101", rb.BufferSize())
	}
}

func TestRollingBuffer_ResetClearsAll(t *testing.T) {
	rb := newRBCells()
	for i := 0; i < 5; i++ {
		rb.Get(i).pos = i
	}
	rb.Reset()
	if rb.MaxPos() != -1 {
		t.Fatalf("MaxPos after Reset=%d want -1", rb.MaxPos())
	}
	if rb.BufferSize() != 0 {
		t.Fatalf("BufferSize after Reset=%d want 0", rb.BufferSize())
	}
	c := rb.Get(0)
	if c.pos != -1 || c.dirty {
		t.Fatalf("slot not reset: pos=%d dirty=%v", c.pos, c.dirty)
	}
}

func TestRollingBuffer_PanicsOnOutOfBounds(t *testing.T) {
	rb := newRBCells()
	for i := 0; i < 5; i++ {
		rb.Get(i)
	}
	rb.FreeBefore(3)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on access before freeBefore threshold")
		}
	}()
	_ = rb.Get(0)
}

func TestRollingBuffer_NilFactoryPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on nil factory")
		}
	}()
	_ = NewRollingBuffer[*rbCell](nil)
}
