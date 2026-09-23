// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// This file pins the StringField defect that stopped every StringField from
// being indexed: the Go StringField inherited Field.BinaryValue (nil for a
// String value) and Field.InvertableType (TOKEN_STREAM), while Lucene 10.5.0's
// StringField (lucene/core/src/java/org/apache/lucene/document/StringField.java)
// caches binaryValue = new BytesRef(value), reports InvertableType.BINARY and
// keeps its own StoredValue. The indexing chain rejected the field with
// "returns BINARY for invertableType and nil for binaryValue". The assertions
// follow TestField.testStringField (setStringValue then stringValue /
// storedValue) plus the binaryValue and invertableType contract.

import (
	"bytes"
	"testing"
)

func TestStringField_BinaryValueRegression(t *testing.T) {
	for _, stored := range []bool{false, true} {
		field, err := NewStringField("foo", "bar", stored)
		if err != nil {
			t.Fatal(err)
		}
		var f IndexableField = field
		if got := f.InvertableType(); got != InvertableTypeBinary {
			t.Fatalf("stored=%v: invertableType() = %v, want BINARY", stored, got)
		}
		if got := f.BinaryValue(); !bytes.Equal(got, []byte("bar")) {
			t.Fatalf("stored=%v: binaryValue() = %q, want %q", stored, got, "bar")
		}

		field.SetStringValue("baz")
		if got := f.StringValue(); got != "baz" {
			t.Fatalf("stored=%v: stringValue() = %q, want baz", stored, got)
		}
		if got := f.BinaryValue(); !bytes.Equal(got, []byte("baz")) {
			t.Fatalf("stored=%v: binaryValue() after setStringValue = %q, want baz", stored, got)
		}
		if stored {
			if sv := field.StoredValue(); sv == nil || sv.StringValue() != "baz" {
				t.Fatalf("storedValue() = %v, want baz", sv)
			}
		} else if sv := field.StoredValue(); sv != nil {
			t.Fatalf("storedValue() = %v, want null", sv)
		}
	}

	field, err := NewStringFieldFromBytes("foo", []byte("bar"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got := field.BinaryValue(); !bytes.Equal(got, []byte("bar")) {
		t.Fatalf("binaryValue() = %q, want bar", got)
	}
}
