// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Test2BNumericDocValues validates that the NumericDocValuesWriter correctly
// buffers and returns numeric values at moderate scale. This exercises the
// same buffering and doc-ID tracking code paths as Lucene's @Monster test
// that indexes 2B docs with numeric doc values.
func Test2BNumericDocValues(t *testing.T) {
	fi := index.NewFieldInfo("num", 0, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeNumeric,
	})
	counter := util.NewCounter()
	w := index.NewNumericDocValuesWriter(fi, counter)

	const numDocs = 2500
	for i := 0; i < numDocs; i++ {
		if err := w.AddValue(i, int64(i*2)); err != nil {
			t.Fatalf("AddValue(%d): %v", i, err)
		}
	}

	dv := w.GetDocValues()
	if dv == nil {
		t.Fatal("GetDocValues returned nil")
	}

	seen := 0
	for {
		docID, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if docID == index.NO_MORE_DOCS {
			break
		}
		v, err := dv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		want := int64(docID * 2)
		if v != want {
			t.Errorf("doc %d: value = %d, want %d", docID, v, want)
		}
		seen++
	}
	if seen != numDocs {
		t.Errorf("read %d docs, want %d", seen, numDocs)
	}
}
