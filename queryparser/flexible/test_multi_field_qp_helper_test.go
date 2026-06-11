// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMultiFieldQPHelper verifies basic multi-field query parsing with
// StandardQueryParser.
func TestMultiFieldQPHelper(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("body")
	parser.SetAnalyzer(analysis.NewStandardAnalyzer())

	t.Run("single field term", func(t *testing.T) {
		q, err := parser.Parse("test")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermQuery); !ok {
			t.Errorf("expected TermQuery, got %T", q)
		}
	})

	t.Run("multi field boost", func(t *testing.T) {
		q, err := parser.Parse("title:test")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})
}
