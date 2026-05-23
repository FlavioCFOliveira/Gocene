// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of the caching behaviour tested implicitly in
// org.apache.lucene.expressions.TestExpressionValueSource.testFibonacciExpr.
package expressions_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions"
)

// TestCachingExpressionValueSource_SharedCache verifies that when the same
// variable name appears multiple times, the CachingExpressionValueSource
// evaluates it only once per GetValues call by sharing the value cache.
func TestCachingExpressionValueSource_SharedCache(t *testing.T) {
	evalCount := 0

	// A source that increments evalCount on each GetValues call.
	countingSource := &countingDoubleValuesSource{
		inner:     &constantSource{42.0},
		evalCount: &evalCount,
	}

	// expr = a + a  — "a" appears twice in variables (simulating repeated reference).
	expr := expressions.NewExpressionWithDoubleValues(
		"a + a",
		[]string{"a", "a"},
		func(fvs []expressions.DoubleValues) (float64, error) {
			v0, err := fvs[0].DoubleValue()
			if err != nil {
				return 0, err
			}
			v1, err := fvs[1].DoubleValue()
			if err != nil {
				return 0, err
			}
			return v0 + v1, nil
		},
	)

	bindings := mapBindings{"a": countingSource}
	evs, err := expressions.NewExpressionValueSource(bindings, expr)
	if err != nil {
		t.Fatalf("NewExpressionValueSource: %v", err)
	}
	cevs := expressions.NewCachingExpressionValueSource(evs)

	// Reset counter before GetValues call.
	evalCount = 0
	dv, err := cevs.GetValues(nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if _, err := dv.AdvanceExact(0); err != nil {
		t.Fatalf("AdvanceExact: %v", err)
	}
	got, err := dv.DoubleValue()
	if err != nil {
		t.Fatalf("DoubleValue: %v", err)
	}

	if got != 84.0 {
		t.Errorf("expected 84.0, got %v", got)
	}
	// The cache should have caused only 1 GetValues call for "a", not 2.
	if evalCount != 1 {
		t.Errorf("expected 1 GetValues call for 'a' (cache hit for duplicate), got %d", evalCount)
	}
}

// TestCachingExpressionValueSource_NilPanics verifies the nil-guard.
func TestCachingExpressionValueSource_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil src")
		}
	}()
	expressions.NewCachingExpressionValueSource(nil)
}

// TestCachingExpressionValueSource_ImplementsDoubleValuesSource is a
// compile-time interface assertion test.
func TestCachingExpressionValueSource_ImplementsDoubleValuesSource(t *testing.T) {
	var _ expressions.DoubleValuesSource = (*expressions.CachingExpressionValueSource)(nil)
}

// --- helper ---

type countingDoubleValuesSource struct {
	inner     expressions.DoubleValuesSource
	evalCount *int
}

func (c *countingDoubleValuesSource) GetValues(scores expressions.DoubleValues) (expressions.DoubleValues, error) {
	*c.evalCount++
	return c.inner.GetValues(scores)
}
func (c *countingDoubleValuesSource) NeedsScores() bool { return false }
func (c *countingDoubleValuesSource) IsCacheable() bool { return true }
