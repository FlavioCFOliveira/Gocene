// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomywritercache

// Test2GBCharBlockArray ports assertions from
// org.apache.lucene.facet.taxonomy.writercache.Test2GBCharBlockArray.
//
// The Java test is annotated @Monster("uses lots of space and takes a few minutes")
// and fills >2 GB of chars to verify the capacity overflow guard.
//
// Unit-testable parts:
//   - Normal append/length behaviour with a small block size
//   - Overflow guard panics when total capacity would exceed the platform limit
//
// The full 2 GB stress path is structurally verified via the overflow-guard
// test with a block size that triggers the guard on the second block.

import (
	"math"
	"testing"
)

// Test2GBCharBlockArray_NormalAppend verifies that AppendRunes correctly
// accumulates characters across multiple blocks.
func Test2GBCharBlockArray_NormalAppend(t *testing.T) {
	const blockSize = 4
	array := NewCharBlockArrayWithBlockSize(blockSize)

	chars := []rune{'h', 'e', 'l', 'l', 'o'}
	array.AppendRunes(chars, 0, len(chars))

	if array.Length() != 5 {
		t.Errorf("Length: want 5, got %d", array.Length())
	}
	for i, want := range chars {
		if got := array.CharAt(i); got != want {
			t.Errorf("CharAt(%d): want %c, got %c", i, want, got)
		}
	}
}

// Test2GBCharBlockArray_AppendAcrossBlocks verifies that appending data larger
// than a single block correctly spans multiple blocks.
func Test2GBCharBlockArray_AppendAcrossBlocks(t *testing.T) {
	const blockSize = 3
	array := NewCharBlockArrayWithBlockSize(blockSize)

	data := []rune("abcdef") // 6 runes, 2 blocks of 3
	array.AppendRunes(data, 0, len(data))

	if array.Length() != 6 {
		t.Errorf("Length: want 6, got %d", array.Length())
	}
	if s := array.SubSequence(0, 6); s != "abcdef" {
		t.Errorf("SubSequence: want %q, got %q", "abcdef", s)
	}
}

// Test2GBCharBlockArray_OverflowGuard verifies that addBlock panics when the
// total capacity would overflow Integer.MAX_VALUE (matching Java behaviour).
//
// We use a blockSize equal to MaxInt32/2 + 1 so that a second block exceeds the guard.
func Test2GBCharBlockArray_OverflowGuard(t *testing.T) {
	const maxInt32 = math.MaxInt32

	// Choose a blockSize that is > MaxInt32/2 so a second block triggers the guard.
	blockSize := maxInt32/2 + 2

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when capacity exceeds platform limit, got none")
		}
	}()

	// NewCharBlockArrayWithBlockSize allocates the first block immediately.
	// AppendRune will trigger a second addBlock call once the first block is full.
	array := NewCharBlockArrayWithBlockSize(blockSize)

	// Fill the first block completely.
	for i := 0; i < blockSize; i++ {
		array.AppendRune('x')
	}
	// This next call must trigger addBlock for the second block, which should panic.
	array.AppendRune('x')
}

// Test2GBCharBlockArray_SubSequence verifies SubSequence returns the correct
// substring from a multi-block array.
func Test2GBCharBlockArray_SubSequence(t *testing.T) {
	array := NewCharBlockArrayWithBlockSize(4)

	data := []rune("hello world")
	array.AppendRunes(data, 0, len(data))

	if s := array.SubSequence(0, 5); s != "hello" {
		t.Errorf("SubSequence(0,5): want %q, got %q", "hello", s)
	}
	if s := array.SubSequence(6, 11); s != "world" {
		t.Errorf("SubSequence(6,11): want %q, got %q", "world", s)
	}
}

