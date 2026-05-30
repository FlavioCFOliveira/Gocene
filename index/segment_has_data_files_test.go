// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestSegmentHasDataFiles classifies a segment's file set as data-bearing or
// metadata-only. openSegmentReader (rmp #4) uses this to decide whether a
// core-readers wiring failure is a genuine corruption (data present but
// unreadable -> explicit error) or a benign metadata-only segment (a not-yet-
// data-merged ForceMerge result or an AddIndexes placeholder -> graceful
// FieldInfos-only fallback).
func TestSegmentHasDataFiles(t *testing.T) {
	cases := []struct {
		name     string
		files    []string
		compound bool
		want     bool
	}{
		{
			name:  "metadata-only (.si + .fnm)",
			files: []string{"_3.si", "_3.fnm"},
			want:  false,
		},
		{
			name:  "si only",
			files: []string{"_0.si"},
			want:  false,
		},
		{
			name:     "compound segment",
			files:    []string{"_0.si", "_0.cfs", "_0.cfe"},
			compound: true,
			want:     true,
		},
		{
			name:  "non-compound with doc-values data",
			files: []string{"_3.si", "_3.fnm", "_3.dvd", "_3.dvm"},
			want:  true,
		},
		{
			name:  "non-compound with stored fields",
			files: []string{"_3.si", "_3.fnm", "_3.fdt", "_3.fdx"},
			want:  true,
		},
		{
			name:  "non-compound with postings",
			files: []string{"_3.si", "_3.fnm", "_3.doc", "_3.tim", "_3.tip"},
			want:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			si := NewSegmentInfo("_x", 1, nil)
			si.SetFiles(tc.files)
			si.SetCompoundFile(tc.compound)
			if got := segmentHasDataFiles(si); got != tc.want {
				t.Errorf("segmentHasDataFiles(%v, compound=%v) = %v, want %v",
					tc.files, tc.compound, got, tc.want)
			}
		})
	}
}
