// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene101

import (
	"testing"
)

// TestLucene101PostingsFormatV0_BlockSize verifies the block size constant.
func TestLucene101PostingsFormatV0_BlockSize(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize = %d, want 128", BlockSize)
	}
}

// TestLucene101PostingsFormatV0_PostingsFormat verifies the postings format
// constructor and field values.
func TestLucene101PostingsFormatV0_PostingsFormat(t *testing.T) {
	pf := NewLucene101PostingsFormat("v0")
	if pf.Name != "Lucene101PostingsFormat" {
		t.Errorf("Name = %q, want %q", pf.Name, "Lucene101PostingsFormat")
	}
	if pf.Version != "v0" {
		t.Errorf("Version = %q, want %q", pf.Version, "v0")
	}
}

// TestLucene101PostingsFormatV0_ForUtil verifies the ForUtil constructor.
func TestLucene101PostingsFormatV0_ForUtil(t *testing.T) {
	fu := NewForUtil("10.1.0")
	if fu.Name != "ForUtil" {
		t.Errorf("ForUtil.Name = %q, want %q", fu.Name, "ForUtil")
	}
	if fu.Version != "10.1.0" {
		t.Errorf("ForUtil.Version = %q, want %q", fu.Version, "10.1.0")
	}
}

// TestLucene101PostingsFormatV0_ForDeltaUtil verifies the ForDeltaUtil constructor.
func TestLucene101PostingsFormatV0_ForDeltaUtil(t *testing.T) {
	fdu := NewForDeltaUtil("10.1.0")
	if fdu.Name != "ForDeltaUtil" {
		t.Errorf("ForDeltaUtil.Name = %q, want %q", fdu.Name, "ForDeltaUtil")
	}
	if fdu.Version != "10.1.0" {
		t.Errorf("ForDeltaUtil.Version = %q, want %q", fdu.Version, "10.1.0")
	}
}

// TestLucene101PostingsFormatV0_PostingsReader verifies the PostingsReader constructor.
func TestLucene101PostingsFormatV0_PostingsReader(t *testing.T) {
	pr := NewLucene101PostingsReader("10.1.0")
	if pr.Name != "Lucene101PostingsReader" {
		t.Errorf("PostingsReader.Name = %q, want %q", pr.Name, "Lucene101PostingsReader")
	}
	if pr.Version != "10.1.0" {
		t.Errorf("PostingsReader.Version = %q, want %q", pr.Version, "10.1.0")
	}
}
