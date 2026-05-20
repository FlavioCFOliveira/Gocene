// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"testing"
)

// TestDocValuesLeafReader_ErrorReturningMethodsUnsupported verifies that every
// shadowed method with an error channel returns errDocValuesLeafReaderUnsupported.
func TestDocValuesLeafReader_ErrorReturningMethodsUnsupported(t *testing.T) {
	r := NewDocValuesLeafReader(nil)

	if _, err := r.Terms("f"); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("Terms: %v", err)
	}
	if _, err := r.GetNormValues("f"); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("GetNormValues: %v", err)
	}
	if _, err := r.GetPointValues("f"); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("GetPointValues: %v", err)
	}
	if _, err := r.GetFloatVectorValues("f"); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("GetFloatVectorValues: %v", err)
	}
	if _, err := r.GetByteVectorValues("f"); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("GetByteVectorValues: %v", err)
	}
	if _, err := r.SearchNearestVectors("f", nil, 1, nil); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("SearchNearestVectors: %v", err)
	}
	if err := r.CheckIntegrity(); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("CheckIntegrity: %v", err)
	}
	if _, err := r.TermVectors(); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("TermVectors: %v", err)
	}
	if _, err := r.GetTermVectors(0); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("GetTermVectors: %v", err)
	}
	if _, err := r.StoredFields(); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("StoredFields: %v", err)
	}
	if _, err := r.GetDocValuesSkipper("f"); !errors.Is(err, errDocValuesLeafReaderUnsupported) {
		t.Errorf("GetDocValuesSkipper: %v", err)
	}
}

// TestDocValuesLeafReader_PanickingMethodsUnsupported verifies that every
// shadowed method without an error channel panics with the sentinel,
// mirroring Lucene's unconditional UnsupportedOperationException.
func TestDocValuesLeafReader_PanickingMethodsUnsupported(t *testing.T) {
	cases := []struct {
		name string
		call func()
	}{
		{"GetCoreCacheKey", func() { NewDocValuesLeafReader(nil).GetCoreCacheKey() }},
		{"GetLiveDocs", func() { NewDocValuesLeafReader(nil).GetLiveDocs() }},
		{"GetMetaData", func() { NewDocValuesLeafReader(nil).GetMetaData() }},
		{"NumDocs", func() { NewDocValuesLeafReader(nil).NumDocs() }},
		{"MaxDoc", func() { NewDocValuesLeafReader(nil).MaxDoc() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				switch p := recover(); p {
				case nil:
					t.Errorf("%s: expected panic, got none", tc.name)
				case errDocValuesLeafReaderUnsupported:
				default:
					t.Errorf("%s: panicked with %v, want sentinel", tc.name, p)
				}
			}()
			tc.call()
		})
	}
}
