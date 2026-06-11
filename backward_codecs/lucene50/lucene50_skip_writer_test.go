// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50SkipWriter_Constants verifies the exported format constants.
func TestLucene50SkipWriter_Constants(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize: got %d, want 128", BlockSize)
	}
	if VersionStart != 0 {
		t.Errorf("VersionStart: got %d, want 0", VersionStart)
	}
	if VersionImpactSkipData != 1 {
		t.Errorf("VersionImpactSkipData: got %d, want 1", VersionImpactSkipData)
	}
	if VersionCurrent != VersionImpactSkipData {
		t.Errorf("VersionCurrent: got %d, want VersionImpactSkipData", VersionCurrent)
	}
}

// TestLucene50SkipWriter_Trim verifies the df trimming logic.
func TestLucene50SkipWriter_Trim(t *testing.T) {
	cases := []struct {
		df   int
		want int
	}{
		{BlockSize, BlockSize - 1},
		{2 * BlockSize, 2*BlockSize - 1},
		{BlockSize + 1, BlockSize + 1},
		{BlockSize - 1, BlockSize - 1},
		{1, 1},
	}
	for _, tc := range cases {
		if got := trim(tc.df); got != tc.want {
			t.Errorf("trim(%d): got %d, want %d", tc.df, got, tc.want)
		}
	}
}

// TestLucene50SkipWriter_NewSkipReader verifies that constructing a
// Lucene50SkipReader does not panic.
func TestLucene50SkipWriter_NewSkipReader(t *testing.T) {
	stream := newTestStream(t)
	r := NewLucene50SkipReader(VersionCurrent, stream, 2, false, false, false)
	if r == nil {
		t.Fatal("NewLucene50SkipReader returned nil")
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestLucene50SkipWriter_NewScoreSkipReader verifies that constructing a
// Lucene50ScoreSkipReader with valid version does not panic.
func TestLucene50SkipWriter_NewScoreSkipReader(t *testing.T) {
	stream := newTestStream(t)
	r := NewLucene50ScoreSkipReader(VersionCurrent, stream, 2, false, false, false)
	if r == nil {
		t.Fatal("NewLucene50ScoreSkipReader returned nil")
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
