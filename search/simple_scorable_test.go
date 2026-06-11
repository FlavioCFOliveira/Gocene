// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/SimpleScorable.java
//
// No dedicated Java test peer found.  These tests cover the Go public
// contract: default zero values, SetScore/Score round-trip, and
// SetMinCompetitiveScore/MinCompetitiveScore round-trip.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestSimpleScorable_Defaults verifies zero-value initial state.
func TestSimpleScorable_Defaults(t *testing.T) {
	var s search.SimpleScorable
	score, err := s.Score()
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}
	if score != 0 {
		t.Errorf("Score()=%v, want 0", score)
	}
	if s.MinCompetitiveScore() != 0 {
		t.Errorf("MinCompetitiveScore()=%v, want 0", s.MinCompetitiveScore())
	}
}

// TestSimpleScorable_SetScore verifies SetScore / Score round-trip.
func TestSimpleScorable_SetScore(t *testing.T) {
	var s search.SimpleScorable
	s.SetScore(3.14)
	got, err := s.Score()
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}
	const eps = float32(1e-6)
	if got < 3.14-eps || got > 3.14+eps {
		t.Errorf("Score()=%v, want ~3.14", got)
	}
}

// TestSimpleScorable_SetMinCompetitiveScore verifies the round-trip.
func TestSimpleScorable_SetMinCompetitiveScore(t *testing.T) {
	var s search.SimpleScorable
	if err := s.SetMinCompetitiveScore(1.5); err != nil {
		t.Fatalf("SetMinCompetitiveScore: %v", err)
	}
	const eps = float32(1e-6)
	got := s.MinCompetitiveScore()
	if got < 1.5-eps || got > 1.5+eps {
		t.Errorf("MinCompetitiveScore()=%v, want ~1.5", got)
	}
}

// TestSimpleScorable_ImplementsScorable checks interface satisfaction.
func TestSimpleScorable_ImplementsScorable(t *testing.T) {
	var s search.SimpleScorable
	var _ search.Scorable = &s
}

// TestSimpleScorable_MultipleUpdates verifies repeated SetScore calls.
func TestSimpleScorable_MultipleUpdates(t *testing.T) {
	var s search.SimpleScorable
	for _, v := range []float32{0, 1, 2.5, 100, -1} {
		s.SetScore(v)
		got, err := s.Score()
		if err != nil {
			t.Fatalf("Score() error: %v", err)
		}
		if got != v {
			t.Errorf("after SetScore(%v): Score()=%v", v, got)
		}
}