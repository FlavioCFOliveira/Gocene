// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// storedFieldInfo builds the stored-only FieldInfo a codec reader resolves
// before invoking the visitor.
func storedFieldInfo(name string, number int) *spi.FieldInfo {
	return spi.NewFieldInfo(name, number, spi.FieldInfoOptions{Stored: true})
}

func needs(t *testing.T, v *DocumentStoredFieldVisitor, name string) spi.StoredFieldVisitorStatus {
	t.Helper()
	status, err := v.NeedsField(storedFieldInfo(name, 0))
	if err != nil {
		t.Fatalf("NeedsField(%q): %v", name, err)
	}
	return status
}

func TestDocumentStoredFieldVisitor_LoadAll(t *testing.T) {
	v := NewDocumentStoredFieldVisitor()
	if needs(t, v, "anything") != spi.StoredFieldVisitorStatusYes {
		t.Fatalf("default visitor should accept all fields")
	}
	for _, call := range []func() error{
		func() error { return v.StringField(storedFieldInfo("title", 0), "hello") },
		func() error { return v.IntField(storedFieldInfo("year", 1), 2026) },
		func() error { return v.LongField(storedFieldInfo("ts", 2), 1234567890) },
		func() error { return v.FloatField(storedFieldInfo("score", 3), 1.25) },
		func() error { return v.DoubleField(storedFieldInfo("ratio", 4), 2.5) },
		func() error { return v.BinaryField(storedFieldInfo("blob", 5), []byte{1, 2, 3}) },
	} {
		if err := call(); err != nil {
			t.Fatalf("visitor callback: %v", err)
		}
	}
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
	if needs(t, v, "title") != spi.StoredFieldVisitorStatusYes {
		t.Fatalf("title should be accepted")
	}
	if needs(t, v, "body") != spi.StoredFieldVisitorStatusNo {
		t.Fatalf("body should be rejected")
	}
}

func TestDocumentStoredFieldVisitor_FilterEmpty(t *testing.T) {
	v := NewDocumentStoredFieldVisitorFor()
	if needs(t, v, "anything") != spi.StoredFieldVisitorStatusNo {
		t.Fatalf("empty fields set should accept nothing")
	}
}
