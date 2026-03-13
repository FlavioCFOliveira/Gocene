// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestLucene104StoredFieldsFormatInterface(t *testing.T) {
	var _ StoredFieldsFormat = NewLucene104StoredFieldsFormat()
}

func TestLucene104StoredFieldsFormatPlaceholder(t *testing.T) {
	format := NewLucene104StoredFieldsFormat()
	if format.Name() != "Lucene104StoredFieldsFormat" {
		t.Errorf("expected name Lucene104StoredFieldsFormat, got %s", format.Name())
	}

	dir := store.NewByteBuffersDirectory()
	si := index.NewSegmentInfo("_0", 1, dir)
	fi := index.NewFieldInfos()
	ctx := store.IOContextRead

	_, err := format.FieldsReader(dir, si, fi, ctx)
	if err == nil {
		t.Error("expected error from FieldsReader placeholder, got nil")
	}

	_, err = format.FieldsWriter(dir, si, ctx)
	if err == nil {
		t.Error("expected error from FieldsWriter placeholder, got nil")
	}
}

// The following tests are placeholders for when the implementation is ready.
// They are based on Lucene's BaseStoredFieldsFormatTestCase.

func TestEmptyDocs(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}

func TestRandomStoredFields(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}

func TestBigDocuments(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}

func TestDoubleStoredFields(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}

func TestNumericField(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}

func TestStoredFieldsOrder(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}

func TestBinaryFieldOffsetLength(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}

func TestConcurrentReads(t *testing.T) {
	t.Skip("Lucene104StoredFieldsFormat is not yet implemented")
}
