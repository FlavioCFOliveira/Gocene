// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"testing"
)

// TestNewForDeltaUtil_Defaults verifies that NewForDeltaUtil sets Name and Version
// correctly.
func TestNewForDeltaUtil_Defaults(t *testing.T) {
	fd := NewForDeltaUtil("9.9.0")
	if fd.Name != "ForDeltaUtil" {
		t.Errorf("Name: got %q, want %q", fd.Name, "ForDeltaUtil")
	}
	if fd.Version != "9.9.0" {
		t.Errorf("Version: got %q, want %q", fd.Version, "9.9.0")
	}
}

// TestNewForDeltaUtil_MultipleVersions verifies that different version strings
// are stored correctly.
func TestNewForDeltaUtil_MultipleVersions(t *testing.T) {
	versions := []string{"", "1.0", "9.9.0", "10.4.0"}
	for _, v := range versions {
		fd := NewForDeltaUtil(v)
		if fd.Version != v {
			t.Errorf("Version: got %q, want %q", fd.Version, v)
		}
	}
}

// TestNewLucene99PostingsFormat_WithVersion verifies the postings format
// constructor accepts a version string.
func TestNewLucene99PostingsFormat_WithVersion(t *testing.T) {
	pf := NewLucene99PostingsFormat("10.4.0-backward")
	if pf.Name != "Lucene99PostingsFormat" {
		t.Errorf("Name: got %q, want %q", pf.Name, "Lucene99PostingsFormat")
	}
	if pf.Version != "10.4.0-backward" {
		t.Errorf("Version: got %q, want %q", pf.Version, "10.4.0-backward")
	}
}

// TestNewLucene99PostingsReader_ReadOnlyName verifies the reader name mirrors the
// Java class name.
func TestNewLucene99PostingsReader_ReadOnlyName(t *testing.T) {
	r := NewLucene99PostingsReader("test")
	if r.Name != "Lucene99PostingsReader" {
		t.Errorf("Name: got %q, want %q", r.Name, "Lucene99PostingsReader")
	}
}
