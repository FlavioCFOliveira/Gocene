// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Test2BSortedDocValuesFixedSorted validates that the SortedDocValuesWriter
// correctly buffers and returns fixed-size (2-byte) sorted doc values at
// moderate scale. This exercises the same term-ordinal deduplication and
// doc-ID tracking code paths as Lucene's @Monster test that indexes 2B docs
// with SortedDocValues.
func Test2BSortedDocValuesFixedSorted(t *testing.T) {
	fi := index.NewFieldInfo("sort", 0, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeSorted,
	})
	counter := util.NewCounter()
	pool := util.NewByteBlockPool(util.NewDirectAllocator())
	w := index.NewSortedDocValuesWriter(fi, counter, pool)

	values := []string{"aa", "bb", "cc", "dd", "ee"}
	const numDocs = 1000
	for i := 0; i < numDocs; i++ {
		val := []byte(values[i%len(values)])
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

	if dv.GetValueCount() != len(values) {
		t.Errorf("valueCount = %d, want %d", dv.GetValueCount(), len(values))
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
		ord, ordErr := dv.OrdValue()
		if ordErr != nil {
			t.Fatalf("OrdValue: %v", ordErr)
		}
		term, termErr := dv.LookupOrd(ord)
		if termErr != nil {
			t.Fatalf("LookupOrd: %v", termErr)
		}
		want := values[docID%len(values)]
		if string(term) != want {
			t.Errorf("doc %d (ord=%d): got %q, want %q", docID, ord, string(term), want)
		}
		seen++
	}
	if seen != numDocs {
		t.Errorf("read %d docs, want %d", seen, numDocs)
	}
}
