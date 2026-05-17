// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNumericDocValues_INDEXEDTYPE(t *testing.T) {
	if NumericDocValuesFieldINDEXEDTYPE.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeRange {
		t.Fatalf("INDEXED_TYPE must have RANGE skip index")
	}
	f, err := NewNumericDocValuesFieldIndexed("n", 42)
	if err != nil {
		t.Fatal(err)
	}
	if f.GetValue() != 42 {
		t.Fatalf("value = %d", f.GetValue())
	}
}

func TestSortedNumericDocValues_INDEXEDTYPE(t *testing.T) {
	if SortedNumericDocValuesFieldINDEXEDTYPE.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeRange {
		t.Fatalf("INDEXED_TYPE must have RANGE skip index")
	}
	f, err := NewSortedNumericDocValuesFieldIndexed("n", []int64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if f.ValueCount() != 3 {
		t.Fatalf("value count = %d", f.ValueCount())
	}
}

func TestDoubleDocValuesField(t *testing.T) {
	f, err := NewDoubleDocValuesField("d", 1.5)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.GetDoubleValue(); got != 1.5 {
		t.Fatalf("value = %v", got)
	}
	f.SetDoubleValue(-2.5)
	if got := f.GetDoubleValue(); got != -2.5 {
		t.Fatalf("after set = %v", got)
	}
}

func TestFloatDocValuesField(t *testing.T) {
	f, err := NewFloatDocValuesField("f", 3.14)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.GetFloatValue(); got != 3.14 {
		t.Fatalf("value = %v", got)
	}
	f.SetFloatValue(-1.5)
	if got := f.GetFloatValue(); got != -1.5 {
		t.Fatalf("after set = %v", got)
	}
}

func TestDocValuesFieldTYPEAliases(t *testing.T) {
	if NumericDocValuesFieldTYPE != NumericDocValuesFieldType ||
		SortedDocValuesFieldTYPE != SortedDocValuesFieldType ||
		SortedNumericDocValuesFieldTYPE != SortedNumericDocValuesFieldType ||
		SortedSetDocValuesFieldTYPE != SortedSetDocValuesFieldType {
		t.Fatalf("TYPE alias missing")
	}
}
