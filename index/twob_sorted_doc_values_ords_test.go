// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Test2BSortedDocValuesOrds validates that ordinals assigned by
// SortedDocValuesWriter are stable and round-trip correctly at moderate
// scale. Each unique value maps to a stable ordinal; duplicate values map
// to the same ordinal.
func Test2BSortedDocValuesOrds(t *testing.T) {
	fi := index.NewFieldInfo("ords", 0, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeSorted,
	})
	counter := util.NewCounter()
	pool := util.NewByteBlockPool(util.NewDirectAllocator())
	w := index.NewSortedDocValuesWriter(fi, counter, pool)

	values := []string{"val0", "val1", "val2", "val3", "val4"}
	const numDocs = 500
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

	// Expect 5 unique values.
	if dv.GetValueCount() != len(values) {
		t.Errorf("valueCount = %d, want %d", dv.GetValueCount(), len(values))
	}

	// Verify every doc's ordinal resolves to the correct value.
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

	// Verify all LookupOrd calls for each ordinal return consistent results.
	for ord := 0; ord < dv.GetValueCount(); ord++ {
		term, lookupErr := dv.LookupOrd(ord)
		if lookupErr != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, lookupErr)
		}
		if string(term) != values[ord] {
			t.Errorf("ord %d: lookup = %q, want %q", ord, string(term), values[ord])
		}
	}
}
