// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "testing"

func mustStoredField(t *testing.T, name, value string) IndexableField {
	t.Helper()
	f, err := NewField(name, value, newStoredType())
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDocument_GetField(t *testing.T) {
	d := NewDocument()
	d.Add(mustStoredField(t, "title", "abc"))
	d.Add(mustStoredField(t, "title", "def"))
	if got := d.GetField("title"); got == nil || got.StringValue() != "abc" {
		t.Fatalf("GetField('title') = %v", got)
	}
	if got := d.GetField("missing"); got != nil {
		t.Fatalf("GetField('missing') should be nil, got %v", got)
	}
}

func TestDocument_GetString(t *testing.T) {
	d := NewDocument()
	d.Add(mustStoredField(t, "title", "hello"))
	if got := d.GetString("title"); got != "hello" {
		t.Fatalf("GetString = %q", got)
	}
	if got := d.GetString("missing"); got != "" {
		t.Fatalf("GetString missing = %q", got)
	}
}

func TestDocument_Iterate(t *testing.T) {
	d := NewDocument()
	d.Add(mustStoredField(t, "a", "1"))
	d.Add(mustStoredField(t, "b", "2"))
	d.Add(mustStoredField(t, "c", "3"))
	var names []string
	d.Iterate(func(f IndexableField) bool {
		names = append(names, f.Name())
		return true
	})
	if len(names) != 3 || names[0] != "a" || names[2] != "c" {
		t.Fatalf("iteration order/length wrong: %v", names)
	}
	// Early exit.
	count := 0
	d.Iterate(func(f IndexableField) bool {
		count++
		return f.Name() != "b"
	})
	if count != 2 {
		t.Fatalf("early-exit count = %d, want 2", count)
	}
}

func TestDocument_NeverNil(t *testing.T) {
	d := NewDocument()
	if got := d.GetFieldsArray("nope"); got == nil {
		t.Fatalf("GetFieldsArray must never be nil")
	} else if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
	if got := d.GetValuesArray("nope"); got == nil || len(got) != 0 {
		t.Fatalf("GetValuesArray must be empty-not-nil, got %v", got)
	}
	if got := d.GetBinaryValuesArray("nope"); got == nil || len(got) != 0 {
		t.Fatalf("GetBinaryValuesArray must be empty-not-nil, got %v", got)
	}
}
