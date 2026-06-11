// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
)

// TestStandardQPEnhancements verifies StandardQueryParser enhancements.
func TestStandardQPEnhancements(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("content")
	parser.SetAnalyzer(analysis.NewStandardAnalyzer())

	t.Run("basic term", func(t *testing.T) {
		q, err := parser.Parse("hello")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("boost handling", func(t *testing.T) {
		t.Run("boost term", func(t *testing.T) {
			q, err := parser.Parse("hello^2")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
		t.Run("boost group", func(t *testing.T) {
			q, err := parser.Parse("(hello world)^2")
			if err != nil {
				t.Fatal(err)
			}
			if q == nil {
				t.Fatal("expected non-nil query")
			}
		})
	})
}
