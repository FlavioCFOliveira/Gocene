// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "testing"

func TestDocumentStoredFieldVisitor_LoadAll(t *testing.T) {
	v := NewDocumentStoredFieldVisitor()
	if !v.NeedsField("anything") {
		t.Fatalf("default visitor should accept all fields")
	}
	v.StringField("title", "hello")
	v.IntField("year", 2026)
	v.LongField("ts", 1234567890)
	v.FloatField("score", 1.25)
	v.DoubleField("ratio", 2.5)
	v.BinaryField("blob", []byte{1, 2, 3})
	d := v.GetDocument()
	if got, want := d.Size(), 6; got != want {
		t.Fatalf("Document.Size = %d, want %d", got, want)
	}
	if d.GetField("title").StringValue() != "hello" {
		t.Fatalf("title not stored")
	}
}

func TestDocumentStoredFieldVisitor_Filter(t *testing.T) {
	v := NewDocumentStoredFieldVisitorFor("title", "year")
	if !v.NeedsField("title") {
		t.Fatalf("title should be accepted")
	}
	if v.NeedsField("body") {
		t.Fatalf("body should be rejected")
	}
}

func TestDocumentStoredFieldVisitor_FilterEmpty(t *testing.T) {
	v := NewDocumentStoredFieldVisitorFor()
	if v.NeedsField("anything") {
		t.Fatalf("empty fields set should accept nothing")
	}
}
