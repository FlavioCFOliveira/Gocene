// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene87

import "testing"

func TestLucene87StoredFieldsFormat_New(t *testing.T) {
	f := NewLucene87StoredFieldsFormat("1.0")
	if f == nil {
		t.Fatal("NewLucene87StoredFieldsFormat returned nil")
	}
	if f.Name != "Lucene87StoredFieldsFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name, "Lucene87StoredFieldsFormat")
	}
}

func TestLucene87StoredFieldsFormat_Version(t *testing.T) {
	f := NewLucene87StoredFieldsFormat("v87")
	if f.Version != "v87" {
		t.Fatalf("got Version=%q, want %q", f.Version, "v87")
	}
}
