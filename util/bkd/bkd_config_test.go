// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"strings"
	"testing"
)

func TestBKDConfigValid(t *testing.T) {
	c, err := NewBKDConfig(2, 2, 4, 512)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	if c.NumDims() != 2 {
		t.Errorf("NumDims: got %d want 2", c.NumDims())
	}
	if c.NumIndexDims() != 2 {
		t.Errorf("NumIndexDims: got %d want 2", c.NumIndexDims())
	}
	if c.BytesPerDim() != 4 {
		t.Errorf("BytesPerDim: got %d want 4", c.BytesPerDim())
	}
	if c.MaxPointsInLeafNode() != 512 {
		t.Errorf("MaxPointsInLeafNode: got %d want 512", c.MaxPointsInLeafNode())
	}
	if c.PackedBytesLength() != 8 {
		t.Errorf("PackedBytesLength: got %d want 8", c.PackedBytesLength())
	}
	if c.PackedIndexBytesLength() != 8 {
		t.Errorf("PackedIndexBytesLength: got %d want 8", c.PackedIndexBytesLength())
	}
	if c.BytesPerDoc() != 12 {
		t.Errorf("BytesPerDoc: got %d want 12", c.BytesPerDoc())
	}
}

func TestBKDConfigInvalid(t *testing.T) {
	cases := []struct {
		name    string
		numDims int
		numIdx  int
		bpd     int
		maxLeaf int
		errSub  string
	}{
		{"numDims_low", 0, 1, 4, 512, "numDims must be"},
		{"numDims_high", MaxDims + 1, 1, 4, 512, "numDims must be"},
		{"numIndexDims_low", 2, 0, 4, 512, "numIndexDims must be"},
		{"numIndexDims_high", 2, MaxIndexDims + 1, 4, 512, "numIndexDims must be"},
		{"numIdx_exceeds_numDims", 2, 3, 4, 512, "numIndexDims cannot exceed numDims"},
		{"bytesPerDim_zero", 1, 1, 0, 512, "bytesPerDim must be"},
		{"maxPointsInLeafNode_zero", 1, 1, 4, 0, "maxPointsInLeafNode must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewBKDConfig(tc.numDims, tc.numIdx, tc.bpd, tc.maxLeaf); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			} else if !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("err %q does not contain %q", err.Error(), tc.errSub)
			}
		})
	}
}

func TestBKDConfigOfReturnsInterned(t *testing.T) {
	c1, err := Of(2, 2, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	c2, err := Of(2, 2, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if c1 != c2 {
		t.Fatalf("interned configs differ: %+v vs %+v", c1, c2)
	}
}

func TestBKDConfigOfFallsThroughOnNonDefault(t *testing.T) {
	c, err := Of(3, 3, 4, 256) // not in defaultConfigs (max=256, not 512)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if c.MaxPointsInLeafNode() != 256 {
		t.Fatalf("MaxPointsInLeafNode: got %d want 256", c.MaxPointsInLeafNode())
	}
}

func TestBKDConfigMaxDimensions(t *testing.T) {
	if MaxDims != 16 {
		t.Errorf("MaxDims: got %d want 16", MaxDims)
	}
	if MaxIndexDims != 8 {
		t.Errorf("MaxIndexDims: got %d want 8", MaxIndexDims)
	}
	if DefaultMaxPointsInLeafNode != 512 {
		t.Errorf("DefaultMaxPointsInLeafNode: got %d want 512", DefaultMaxPointsInLeafNode)
	}
}
