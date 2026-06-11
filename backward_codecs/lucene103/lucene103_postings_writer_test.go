// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene103

import (
	"testing"
)

// TestLucene103PostingsWriter_BlockSize verifies the block size constant.
func TestLucene103PostingsWriter_BlockSize(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize = %d, want 128", BlockSize)
	}
}

// TestLucene103PostingsWriter_ForUtil verifies the ForUtil constructor.
func TestLucene103PostingsWriter_ForUtil(t *testing.T) {
	fu := NewForUtil("10.3")
	if fu.Name != "ForUtil" {
		t.Errorf("ForUtil.Name = %q, want %q", fu.Name, "ForUtil")
	}
	if fu.Version != "10.3" {
		t.Errorf("ForUtil.Version = %q, want %q", fu.Version, "10.3")
	}
}

// TestLucene103PostingsWriter_ForDeltaUtil verifies the ForDeltaUtil constructor.
func TestLucene103PostingsWriter_ForDeltaUtil(t *testing.T) {
	fdu := NewForDeltaUtil("10.3")
	if fdu.Name != "ForDeltaUtil" {
		t.Errorf("ForDeltaUtil.Name = %q, want %q", fdu.Name, "ForDeltaUtil")
	}
	if fdu.Version != "10.3" {
		t.Errorf("ForDeltaUtil.Version = %q, want %q", fdu.Version, "10.3")
	}
}

// TestLucene103PostingsWriter_Lucene103Codec verifies the codec constructor.
func TestLucene103PostingsWriter_Lucene103Codec(t *testing.T) {
	c := NewLucene103Codec("10.3")
	if c.Name != "Lucene103Codec" {
		t.Errorf("Codec.Name = %q, want %q", c.Name, "Lucene103Codec")
	}
	if c.Version != "10.3" {
		t.Errorf("Codec.Version = %q, want %q", c.Version, "10.3")
	}
}

// TestLucene103PostingsWriter_PostingsFormat verifies the PostingsFormat constructor.
func TestLucene103PostingsWriter_PostingsFormat(t *testing.T) {
	pf := NewLucene103PostingsFormat("10.3")
	if pf.Name != "Lucene103PostingsFormat" {
		t.Errorf("PostingsFormat.Name = %q, want %q", pf.Name, "Lucene103PostingsFormat")
	}
	if pf.Version != "10.3" {
		t.Errorf("PostingsFormat.Version = %q, want %q", pf.Version, "10.3")
	}
}

// TestLucene103PostingsWriter_PostingsReader verifies the PostingsReader constructor.
func TestLucene103PostingsWriter_PostingsReader(t *testing.T) {
	pr := NewLucene103PostingsReader("10.3")
	if pr.Name != "Lucene103PostingsReader" {
		t.Errorf("PostingsReader.Name = %q, want %q", pr.Name, "Lucene103PostingsReader")
	}
	if pr.Version != "10.3" {
		t.Errorf("PostingsReader.Version = %q, want %q", pr.Version, "10.3")
	}
}
