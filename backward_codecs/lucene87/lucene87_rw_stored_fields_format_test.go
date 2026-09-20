// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene87

import "testing"

// TestLucene87StoredFieldsFormat_New covers the no-argument constructor,
// which mirrors the Java default constructor
// org.apache.lucene.backward_codecs.lucene87.Lucene87StoredFieldsFormat():
//
//	public Lucene87StoredFieldsFormat() {
//	  this(Mode.BEST_SPEED);
//	}
func TestLucene87StoredFieldsFormat_New(t *testing.T) {
	f := NewLucene87StoredFieldsFormat()
	if f == nil {
		t.Fatal("NewLucene87StoredFieldsFormat returned nil")
	}
	if f.Name() != "Lucene87StoredFieldsFormat" {
		t.Fatalf("got Name=%q, want %q", f.Name(), "Lucene87StoredFieldsFormat")
	}
	if f.mode != BestSpeed {
		t.Fatalf("got mode=%v, want %v", f.mode, BestSpeed)
	}
}

// TestLucene87StoredFieldsFormat_Mode covers the mode-carrying constructor,
// which mirrors Lucene87StoredFieldsFormat(Mode mode), and the MODE_KEY
// spelling of each Mode, which is the value written to and read back from
// the segment attribute.
func TestLucene87StoredFieldsFormat_Mode(t *testing.T) {
	for _, tc := range []struct {
		mode Mode
		want string
	}{
		{BestSpeed, "BEST_SPEED"},
		{BestCompression, "BEST_COMPRESSION"},
	} {
		f := NewLucene87StoredFieldsFormatWithMode(tc.mode)
		if f == nil {
			t.Fatalf("NewLucene87StoredFieldsFormatWithMode(%v) returned nil", tc.mode)
		}
		if f.mode != tc.mode {
			t.Fatalf("got mode=%v, want %v", f.mode, tc.mode)
		}
		if got := tc.mode.String(); got != tc.want {
			t.Fatalf("got Mode.String()=%q, want %q", got, tc.want)
		}
		parsed, err := parseMode(tc.want)
		if err != nil {
			t.Fatalf("parseMode(%q) returned error: %v", tc.want, err)
		}
		if parsed != tc.mode {
			t.Fatalf("parseMode(%q) = %v, want %v", tc.want, parsed, tc.mode)
		}
	}
}
