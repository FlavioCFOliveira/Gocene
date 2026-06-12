// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Test2BBinaryDocValuesFixedBinary validates that the BinaryDocValuesWriter
// correctly buffers and returns fixed-size (4-byte) values at moderate scale.
// This exercises the same buffering and doc-ID tracking code paths as Lucene's
// @Monster test that indexes 2B documents, but at a tractable scale.
func Test2BBinaryDocValuesFixedBinary(t *testing.T) {
	fi := index.NewFieldInfo("fixed", 0, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeBinary,
	})
	counter := util.NewCounter()
	w, err := index.NewBinaryDocValuesWriter(fi, counter)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesWriter: %v", err)
	}

	const numDocs = 2500
	for i := 0; i < numDocs; i++ {
		buf := make([]byte, 4)
		buf[0] = byte(i >> 24)
		buf[1] = byte(i >> 16)
		buf[2] = byte(i >> 8)
		buf[3] = byte(i)
		if err := w.AddValue(i, &util.BytesRef{Bytes: buf, Offset: 0, Length: 4}); err != nil {
			t.Fatalf("AddValue(%d): %v", i, err)
		}
	}

	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetDocValues returned nil")
	}

	seen := 0
	for {
		docID, nextErr := dv.NextDoc()
		if nextErr != nil {
			t.Fatalf("NextDoc: %v", nextErr)
		}
		if docID == index.NO_MORE_DOCS {
			break
		}
		val, valErr := dv.BinaryValue()
		if valErr != nil {
			t.Fatalf("BinaryValue@%d: %v", docID, valErr)
		}
		if len(val) != 4 {
			t.Errorf("doc %d: value length = %d, want 4", docID, len(val))
		}
		got := int(uint32(val[0])<<24 | uint32(val[1])<<16 | uint32(val[2])<<8 | uint32(val[3]))
		if got != docID {
			t.Errorf("doc %d: decoded = %d, want %d", docID, got, docID)
		}
		seen++
	}
	if seen != numDocs {
		t.Errorf("read %d docs, want %d", seen, numDocs)
	}
}

// Test2BBinaryDocValuesVariableBinary validates that the BinaryDocValuesWriter
// correctly buffers and returns variable-length values at moderate scale.
func Test2BBinaryDocValuesVariableBinary(t *testing.T) {
	fi := index.NewFieldInfo("var", 0, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeBinary,
	})
	counter := util.NewCounter()
	w, err := index.NewBinaryDocValuesWriter(fi, counter)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesWriter: %v", err)
	}

	const numDocs = 2500
	for i := 0; i < numDocs; i++ {
		val := []byte(strings.Repeat("v", i%200+1))
		if err := w.AddValue(i, &util.BytesRef{Bytes: val, Offset: 0, Length: len(val)}); err != nil {
			t.Fatalf("AddValue(%d): %v", i, err)
		}
	}

	dv, err := w.GetDocValues()
	if err != nil {
		t.Fatalf("GetDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("GetDocValues returned nil")
	}

	seen := 0
	for {
		docID, nextErr := dv.NextDoc()
		if nextErr != nil {
			t.Fatalf("NextDoc: %v", nextErr)
		}
		if docID == index.NO_MORE_DOCS {
			break
		}
		val, valErr := dv.BinaryValue()
		if valErr != nil {
			t.Fatalf("BinaryValue@%d: %v", docID, valErr)
		}
		wantLen := docID%200 + 1
		if len(val) != wantLen {
			t.Errorf("doc %d: value length = %d, want %d", docID, len(val), wantLen)
		}
		for _, b := range val {
			if b != 'v' {
				t.Errorf("doc %d: unexpected byte %x", docID, b)
				break
			}
		}
		seen++
	}
	if seen != numDocs {
		t.Errorf("read %d docs, want %d", seen, numDocs)
	}
}
