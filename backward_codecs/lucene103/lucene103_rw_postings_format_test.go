// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene103

import (
	"testing"
)

// TestLucene103RWPostingsFormat_BlockSize verifies the BlockSize constant.
func TestLucene103RWPostingsFormat_BlockSize(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize = %d, want 128", BlockSize)
	}
}

// TestLucene103RWPostingsFormat_ForUtil verifies the ForUtil constructor.
func TestLucene103RWPostingsFormat_ForUtil(t *testing.T) {
	fu := NewForUtil("10.3.0")
	if fu.Name != "ForUtil" {
		t.Errorf("ForUtil.Name = %q, want %q", fu.Name, "ForUtil")
	}
	if fu.Version != "10.3.0" {
		t.Errorf("ForUtil.Version = %q, want %q", fu.Version, "10.3.0")
	}
}

// TestLucene103RWPostingsFormat_ForDeltaUtil verifies the ForDeltaUtil constructor.
func TestLucene103RWPostingsFormat_ForDeltaUtil(t *testing.T) {
	fdu := NewForDeltaUtil("10.3.0")
	if fdu.Name != "ForDeltaUtil" {
		t.Errorf("ForDeltaUtil.Name = %q, want %q", fdu.Name, "ForDeltaUtil")
	}
	if fdu.Version != "10.3.0" {
		t.Errorf("ForDeltaUtil.Version = %q, want %q", fdu.Version, "10.3.0")
	}
}

// TestLucene103RWPostingsFormat_PostingsFormat verifies the PostingsFormat constructor.
func TestLucene103RWPostingsFormat_PostingsFormat(t *testing.T) {
	pf := NewLucene103PostingsFormat("10.3.0")
	if pf.Name != "Lucene103PostingsFormat" {
		t.Errorf("Name = %q, want %q", pf.Name, "Lucene103PostingsFormat")
	}
	if pf.Version != "10.3.0" {
		t.Errorf("Version = %q, want %q", pf.Version, "10.3.0")
	}
}

// TestLucene103RWPostingsFormat_PostingsReader verifies the PostingsReader constructor.
func TestLucene103RWPostingsFormat_PostingsReader(t *testing.T) {
	pr := NewLucene103PostingsReader("10.3.0")
	if pr.Name != "Lucene103PostingsReader" {
		t.Errorf("Name = %q, want %q", pr.Name, "Lucene103PostingsReader")
	}
	if pr.Version != "10.3.0" {
		t.Errorf("Version = %q, want %q", pr.Version, "10.3.0")
	}
}
