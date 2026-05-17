// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestKeywordField_StringNotStored(t *testing.T) {
	f, err := NewKeywordField("k", "v", false)
	if err != nil {
		t.Fatal(err)
	}
	if f.StringValue() != "v" {
		t.Fatalf("StringValue = %q", f.StringValue())
	}
	if f.FieldType() != KeywordFieldType {
		t.Fatalf("FieldType mismatch")
	}
	if f.FieldType().IsStored() {
		t.Fatalf("not-stored variant must not have Stored=true")
	}
	if f.FieldType().GetDocValuesType() != index.DocValuesTypeSortedSet {
		t.Fatalf("DocValuesType must be SORTED_SET")
	}
}

func TestKeywordField_StringStored(t *testing.T) {
	f, err := NewKeywordField("k", "v", true)
	if err != nil {
		t.Fatal(err)
	}
	if !f.FieldType().IsStored() {
		t.Fatalf("stored variant must have Stored=true")
	}
}

func TestKeywordField_FromBytes(t *testing.T) {
	f, err := NewKeywordFieldFromBytes("k", []byte{1, 2, 3}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.BinaryValue(), []byte{1, 2, 3}) {
		t.Fatalf("BinaryValue mismatch")
	}
	if _, err := NewKeywordFieldFromBytes("k", nil, false); err == nil {
		t.Fatalf("expected error for nil bytes")
	}
	if _, err := NewKeywordField("", "v", false); err == nil {
		t.Fatalf("expected error for empty name")
	}
}

func TestKeywordField_TYPEAliases(t *testing.T) {
	if KeywordFieldFIELDTYPE != KeywordFieldType {
		t.Fatalf("FIELD_TYPE alias mismatch")
	}
	if KeywordFieldFIELDTYPESTORED != KeywordFieldTypeStored {
		t.Fatalf("FIELD_TYPE_STORED alias mismatch")
	}
}
