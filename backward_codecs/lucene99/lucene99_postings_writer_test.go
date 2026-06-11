// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestLucene99PostingsFormat_BlockSize verifies the postings block size constant.
func TestLucene99PostingsFormat_BlockSize(t *testing.T) {
	if got := BlockSize; got != 128 {
		t.Errorf("BlockSize: got %d, want 128", got)
	}
}

// TestNewLucene99PostingsFormat_Defaults verifies that NewLucene99PostingsFormat
// sets Name and Version correctly.
func TestNewLucene99PostingsFormat_Defaults(t *testing.T) {
	pf := NewLucene99PostingsFormat("9.9.0")
	if pf.Name != "Lucene99PostingsFormat" {
		t.Errorf("Name: got %q, want %q", pf.Name, "Lucene99PostingsFormat")
	}
	if pf.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", pf.Version, "9.9.0")
	}
}

// TestNewLucene99PostingsFormat_EmptyVersion verifies that an empty version string
// is accepted.
func TestNewLucene99PostingsFormat_EmptyVersion(t *testing.T) {
	pf := NewLucene99PostingsFormat("")
	if pf.Version != "" {
		t.Errorf("Version: got %q, want empty", pf.Version)
	}
}

// TestNewLucene99PostingsReader_Defaults verifies that NewLucene99PostingsReader
// sets Name and Version correctly.
func TestNewLucene99PostingsReader_Defaults(t *testing.T) {
	r := NewLucene99PostingsReader("9.9.0")
	if r.Name != "Lucene99PostingsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene99PostingsReader")
	}
	if r.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", r.Version, "9.9.0")
	}
}
