// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import "testing"

func TestLucene90PostingsFormat_New(t *testing.T) {
	f := NewLucene90PostingsFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene90PostingsFormat returned nil")
	}
	if f.Name != "Lucene90PostingsFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene90PostingsFormat")
	}
}

func TestLucene90PostingsFormat_Version(t *testing.T) {
	f := NewLucene90PostingsFormat("p90")
	if f.Version != "p90" {
		t.Fatalf("got Version=%q, want %q", f.Version, "p90")
	}
}

func TestLucene90PostingsReader_New(t *testing.T) {
	r := NewLucene90PostingsReader("1.0")
	if r == nil {
		t.Fatal("NewLucene90PostingsReader returned nil")
	}
	if r.Name != "Lucene90PostingsReader" {
		t.Fatalf("got Name=%q, want %q", r.Name, "Lucene90PostingsReader")
	}
}

func TestLucene90PostingsReader_Version(t *testing.T) {
	r := NewLucene90PostingsReader("pr90")
	if r.Version != "pr90" {
		t.Fatalf("got Version=%q, want %q", r.Version, "pr90")
	}
}

func TestLucene90PostingsWriter_New(t *testing.T) {
	w := NewLucene90PostingsWriter("1.0")
	if w == nil {
		t.Fatal("NewLucene90PostingsWriter returned nil")
	}
	if w.Name != "Lucene90PostingsWriter" {
		t.Fatalf("got Name=%q, want %q", w.Name, "Lucene90PostingsWriter")
	}
}

func TestLucene90PostingsWriter_Version(t *testing.T) {
	w := NewLucene90PostingsWriter("pw90")
	if w.Version != "pw90" {
		t.Fatalf("got Version=%q, want %q", w.Version, "pw90")
	}
}
