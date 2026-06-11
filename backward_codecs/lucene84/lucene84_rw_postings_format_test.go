// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene84

import "testing"

func TestLucene84PostingsFormat_NewFromFormat(t *testing.T) {
	f := NewLucene84PostingsFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene84PostingsFormat returned nil")
	}
	if f.Name != "Lucene84PostingsFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene84PostingsFormat")
	}
}

func TestLucene84PostingsFormat_Version(t *testing.T) {
	f := NewLucene84PostingsFormat("test-version")
	if f.Version != "test-version" {
		t.Fatalf("got Version=%q, want %q", f.Version, "test-version")
	}
}

func TestLucene84PostingsReader_New(t *testing.T) {
	r := NewLucene84PostingsReader("1.0")
	if r == nil {
		t.Fatal("NewLucene84PostingsReader returned nil")
	}
	if r.Name != "Lucene84PostingsReader" {
		t.Fatalf("got Name=%q, want %q", r.Name, "Lucene84PostingsReader")
	}
}

func TestLucene84PostingsReader_Version(t *testing.T) {
	r := NewLucene84PostingsReader("reader-v2")
	if r.Version != "reader-v2" {
		t.Fatalf("got Version=%q, want %q", r.Version, "reader-v2")
	}
}
