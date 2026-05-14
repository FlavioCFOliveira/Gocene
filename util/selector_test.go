// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestSelector_CheckArgs_OK(t *testing.T) {
	var s Selector
	// Must not panic.
	s.CheckArgs(0, 10, 0)
	s.CheckArgs(0, 10, 9)
	s.CheckArgs(3, 7, 5)
}

func TestSelector_CheckArgs_KBeforeFrom(t *testing.T) {
	var s Selector
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when k < from")
		}
	}()
	s.CheckArgs(5, 10, 4)
}

func TestSelector_CheckArgs_KAtOrPastTo(t *testing.T) {
	var s Selector
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when k >= to")
		}
	}()
	s.CheckArgs(0, 5, 5)
}

// Selector exists as the abstract base type for selection algorithms
// (IntroSelector, RadixSelector). The compile-time check below pins
// the existence of both the base type and the SelectorInterface
// contract.
func TestSelector_TypePins(t *testing.T) {
	var _ = (*Selector)(nil)
	var _ = (*SelectorInterface)(nil)
}
