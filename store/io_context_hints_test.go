// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "testing"

func TestDataAccessHint_FileOpenHint(t *testing.T) {
	var _ FileOpenHint = DataAccessRandom
	var _ FileOpenHint = DataAccessSequential
	if got, want := DataAccessRandom.String(), "RANDOM"; got != want {
		t.Fatalf("DataAccessRandom.String() = %q, want %q", got, want)
	}
	if got, want := DataAccessSequential.String(), "SEQUENTIAL"; got != want {
		t.Fatalf("DataAccessSequential.String() = %q, want %q", got, want)
	}
}

func TestFileDataHint_FileOpenHint(t *testing.T) {
	var _ FileOpenHint = FileDataPostings
	var _ FileOpenHint = FileDataKNNVectors
	if got, want := FileDataPostings.String(), "POSTINGS"; got != want {
		t.Fatalf("FileDataPostings.String() = %q, want %q", got, want)
	}
	if got, want := FileDataKNNVectors.String(), "KNN_VECTORS"; got != want {
		t.Fatalf("FileDataKNNVectors.String() = %q, want %q", got, want)
	}
}

func TestFileTypeHint_FileOpenHint(t *testing.T) {
	var _ FileOpenHint = FileTypeIndex
	var _ FileOpenHint = FileTypeData
	if got, want := FileTypeIndex.String(), "INDEX"; got != want {
		t.Fatalf("FileTypeIndex.String() = %q, want %q", got, want)
	}
	if got, want := FileTypeData.String(), "DATA"; got != want {
		t.Fatalf("FileTypeData.String() = %q, want %q", got, want)
	}
}

func TestPreloadHint_FileOpenHint(t *testing.T) {
	var _ FileOpenHint = PreloadInstance
	if got, want := PreloadInstance.String(), "INSTANCE"; got != want {
		t.Fatalf("PreloadInstance.String() = %q, want %q", got, want)
	}
}

func TestReadOnceHint_FileOpenHint(t *testing.T) {
	var _ FileOpenHint = ReadOnceInstance
	if got, want := ReadOnceInstance.String(), "INSTANCE"; got != want {
		t.Fatalf("ReadOnceInstance.String() = %q, want %q", got, want)
	}
}

func TestReadAdvice_NotFileOpenHint(t *testing.T) {
	// ReadAdvice is intentionally not a FileOpenHint; we just verify the
	// string mapping and that the type exists.
	tests := []struct {
		advice ReadAdvice
		want   string
	}{
		{ReadAdviceNormal, "NORMAL"},
		{ReadAdviceRandom, "RANDOM"},
		{ReadAdviceSequential, "SEQUENTIAL"},
	}
	for _, tc := range tests {
		if got := tc.advice.String(); got != tc.want {
			t.Fatalf("%d.String() = %q, want %q", tc.advice, got, tc.want)
		}
	}
}
