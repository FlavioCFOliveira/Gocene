// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestFieldScoreQuery.java

package function

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestFieldScoreQuery exercises the FunctionScoreQuery and BoostByValue
// helpers that substitute or multiply document scores with a
// DoubleValuesSource.
//
// The Java original requires RandomIndexWriter + IndexSearcher + full
// index round-trip. Gocene tests the construction, query identity, and
// basic accessors of FunctionScoreQuery and its factory helpers.
func TestFieldScoreQuery(t *testing.T) {
	// FunctionScoreQuery basic constructor.
	inner := search.NewMatchAllDocsQuery()
	src := ConstantDoubleValuesSource(2.0, "doubled")
	q := NewFunctionScoreQuery(inner, src)
	if q.GetWrappedQuery() != inner {
		t.Error("GetWrappedQuery does not return the inner query")
	}
	if q.GetSource() != src {
		t.Error("GetSource does not return the wrapped source")
	}
	if str := q.String(); str == "" {
		t.Error("String() returned empty")
	}

	// BoostByValue factory.
	inner2 := search.NewMatchAllDocsQuery()
	boosted := BoostByValue(inner2, src)
	if boosted.GetWrappedQuery() != inner2 {
		t.Error("BoostByValue changed the inner query unexpectedly")
	}
}
