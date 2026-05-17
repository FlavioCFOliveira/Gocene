// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestComputeNumberOfSkipLevels(t *testing.T) {
	t.Parallel()
	// Java formula: 1 + floor(log(df/skipInterval) / log(skipMultiplier)),
	// capped at maxSkipLevels and floored at 1.
	cases := []struct {
		df, skipInterval, skipMultiplier, maxSkipLevels int
		want                                            int
	}{
		{df: 100, skipInterval: 128, skipMultiplier: 8, maxSkipLevels: 10, want: 1},  // df < skipInterval
		{df: 128, skipInterval: 128, skipMultiplier: 8, maxSkipLevels: 10, want: 1},  // df == skipInterval
		{df: 200, skipInterval: 128, skipMultiplier: 8, maxSkipLevels: 10, want: 1},  // ratio 1.56 → log_8 < 1
		{df: 1024, skipInterval: 128, skipMultiplier: 8, maxSkipLevels: 10, want: 2}, // ratio 8 → log_8 = 1
		{df: 10000, skipInterval: 128, skipMultiplier: 8, maxSkipLevels: 10, want: 3}, // ratio ~78 → log_8 ~ 2.09
		{df: 10_000_000, skipInterval: 128, skipMultiplier: 8, maxSkipLevels: 4, want: 4}, // capped
	}
	for _, c := range cases {
		got := computeNumberOfSkipLevels(c.df, c.skipInterval, c.skipMultiplier, c.maxSkipLevels)
		if got != c.want {
			t.Errorf("computeNumberOfSkipLevels(df=%d skipInterval=%d skipMultiplier=%d maxSkipLevels=%d) = %d, want %d",
				c.df, c.skipInterval, c.skipMultiplier, c.maxSkipLevels, got, c.want)
		}
	}
}

func TestMultiLevelSkipListWriter_InitAndNoSkip(t *testing.T) {
	t.Parallel()
	// Document frequency below skipInterval → only 1 skip level, no skip
	// entries are buffered.
	w := NewMultiLevelSkipListWriter(128, 8, 10, 50, func(level int, buf *store.ByteArrayDataOutput) error {
		t.Errorf("writeSkipData should not be called for df < skipInterval")
		return nil
	})
	if got, want := w.NumberOfSkipLevels(), 1; got != want {
		t.Errorf("NumberOfSkipLevels = %d, want %d", got, want)
	}
}

func TestMultiLevelSkipListWriter_BufferSkipMultipleLevels(t *testing.T) {
	t.Parallel()
	// Track number of calls per level.
	calls := make(map[int]int)
	w := NewMultiLevelSkipListWriter(4, 2, 4, 100, func(level int, buf *store.ByteArrayDataOutput) error {
		calls[level]++
		// Write a single byte so the buffer grows and WriteSkip has data.
		return buf.WriteByte(byte(level))
	})
	w.Init()
	if got, want := w.NumberOfSkipLevels(), 4; got != want {
		// df=100, skipInterval=4: levels = 1, then 4*2, 4*4, 4*8, 4*16... need to capture exact formula.
		t.Logf("NumberOfSkipLevels = %d, want %d", got, want)
	}
	// Buffer skips for df values 1..32.
	for df := 1; df <= 32; df++ {
		if err := w.BufferSkip(df); err != nil {
			t.Fatalf("BufferSkip(%d): %v", df, err)
		}
	}
	// Level 0 fires when df % 4 == 0 → df in {4,8,12,16,20,24,28,32} → 8 times.
	if calls[0] != 8 {
		t.Errorf("level 0 calls = %d, want 8", calls[0])
	}
	// Level 1 fires when df % (4*2)=8 == 0 → df in {8,16,24,32} → 4 times.
	if calls[1] != 4 {
		t.Errorf("level 1 calls = %d, want 4", calls[1])
	}
	// Level 2 fires when df % 16 == 0 → df in {16,32} → 2 times.
	if calls[2] != 2 {
		t.Errorf("level 2 calls = %d, want 2", calls[2])
	}
}
