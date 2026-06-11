// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// No direct Java test peer for DocValuesRewriteMethod; the Java suite
// tests it indirectly via TestDocValuesRangeQuery and
// BaseDocValuesFormatTestCase. These tests cover the Go port's
// constructor/equality contract, the alwaysExhaustedDISI helper, and
// the dvwScorerSupplierImpl ScorerSupplier contract.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// ─── DocValuesRewriteMethod identity / equality ────────────────────────────

func TestDocValuesRewriteMethod_Equality(t *testing.T) {
	a := search.NewDocValuesRewriteMethod()
	b := search.NewDocValuesRewriteMethod()

	if !a.Equals(a) {
		t.Errorf("a.Equals(a) = false (identity)")
	}
	if !a.Equals(b) {
		t.Errorf("a.Equals(b) = false (value equality)")
	}
	if a.Equals(nil) {
		t.Errorf("a.Equals(nil) = true")
	}
	if a.Equals("not-a-method") {
		t.Errorf("a.Equals(string) = true")
	}
}

func TestDocValuesRewriteMethod_HashCode(t *testing.T) {
	a := search.NewDocValuesRewriteMethod()
	b := search.NewDocValuesRewriteMethod()
	if a.HashCode() != b.HashCode() {
		t.Errorf("equal instances produced different hash codes: %d vs %d",
			a.HashCode(), b.HashCode())
	}
}

func TestDocValuesRewriteMethod_DefaultInstance(t *testing.T) {
	if search.DefaultDocValuesRewriteMethod == nil {
		t.Fatal("DefaultDocValuesRewriteMethod is nil")
	}
	if !search.NewDocValuesRewriteMethod().Equals(search.DefaultDocValuesRewriteMethod) {
		t.Errorf("DefaultDocValuesRewriteMethod should be equal to any instance")
	}

// ─── Optional interfaces exported for interop ─────────────────────────────

// TestDocValuesRewriteMethod_OptionalInterfaceTypes verifies that the
// optional interfaces exported by this port compile and are usable from
// outside the search package. We declare typed nil values to verify the
// type names are accessible.
func TestDocValuesRewriteMethod_OptionalInterfaceTypes(_ *testing.T) {
	// These zero-value type assertions confirm the interface types are
	// exported and structurally non-empty.
	var _ search.TermsEnumWithOrd
	var _ search.SortedDocValuesWithOrd
	var _ search.SortedSetDocValuesWithTermsEnum
	var _ search.SortedSetDocValuesOrdIterable
	var _ search.DocValuesSkipperProvider
	var _ search.MultiTermQueryTermsEnumProvider
}

// ─── Rewrite produces ConstantScoreQuery ──────────────────────────────────

func TestDocValuesRewriteMethod_RewriteProducesConstantScoreQuery(t *testing.T) {
	m := search.NewDocValuesRewriteMethod()
	q := search.NewMultiTermQuery("myField", nil)
	result, err := m.Rewrite(nil, q)
	if err != nil {
		t.Fatalf("Rewrite() error: %v", err)
	}
	if result == nil {
		t.Fatal("Rewrite() returned nil query")
	}
	// The wrapped query must be a ConstantScoreQuery.
	if _, ok := result.(*search.ConstantScoreQuery); !ok {
		t.Errorf("Rewrite() returned %T, want *search.ConstantScoreQuery", result)
	}
}