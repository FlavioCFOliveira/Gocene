// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene101

import (
	"testing"
)

// TestLucene101PostingsWriter_BlockSize verifies the block size constant.
func TestLucene101PostingsWriter_BlockSize(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize = %d, want 128", BlockSize)
	}
}

// TestLucene101PostingsWriter_ForUtil verifies the ForUtil constructor.
func TestLucene101PostingsWriter_ForUtil(t *testing.T) {
	fu := NewForUtil("10.1")
	if fu.Name != "ForUtil" {
		t.Errorf("ForUtil.Name = %q, want %q", fu.Name, "ForUtil")
	}
	if fu.Version != "10.1" {
		t.Errorf("ForUtil.Version = %q, want %q", fu.Version, "10.1")
	}
}

// TestLucene101PostingsWriter_ForDeltaUtil verifies the ForDeltaUtil constructor.
func TestLucene101PostingsWriter_ForDeltaUtil(t *testing.T) {
	fdu := NewForDeltaUtil("10.1")
	if fdu.Name != "ForDeltaUtil" {
		t.Errorf("ForDeltaUtil.Name = %q, want %q", fdu.Name, "ForDeltaUtil")
	}
	if fdu.Version != "10.1" {
		t.Errorf("ForDeltaUtil.Version = %q, want %q", fdu.Version, "10.1")
	}
}

// TestLucene101PostingsWriter_Lucene101Codec verifies the codec constructor.
func TestLucene101PostingsWriter_Lucene101Codec(t *testing.T) {
	c := NewLucene101Codec("10.1")
	if c.Name != "Lucene101Codec" {
		t.Errorf("Codec.Name = %q, want %q", c.Name, "Lucene101Codec")
	}
	if c.Version != "10.1" {
		t.Errorf("Codec.Version = %q, want %q", c.Version, "10.1")
	}
}

// TestLucene101PostingsWriter_PostingsFormat verifies the PostingsFormat constructor.
func TestLucene101PostingsWriter_PostingsFormat(t *testing.T) {
	pf := NewLucene101PostingsFormat("10.1")
	if pf.Name != "Lucene101PostingsFormat" {
		t.Errorf("PostingsFormat.Name = %q, want %q", pf.Name, "Lucene101PostingsFormat")
	}
	if pf.Version != "10.1" {
		t.Errorf("PostingsFormat.Version = %q, want %q", pf.Version, "10.1")
	}
}

// TestLucene101PostingsWriter_PostingsReader verifies the PostingsReader constructor.
func TestLucene101PostingsWriter_PostingsReader(t *testing.T) {
	pr := NewLucene101PostingsReader("10.1")
	if pr.Name != "Lucene101PostingsReader" {
		t.Errorf("PostingsReader.Name = %q, want %q", pr.Name, "Lucene101PostingsReader")
	}
	if pr.Version != "10.1" {
		t.Errorf("PostingsReader.Version = %q, want %q", pr.Version, "10.1")
	}
}
