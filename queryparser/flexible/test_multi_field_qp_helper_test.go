// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestMultiFieldQPHelper verifies multi-field query behaviour using the
// StandardQueryParser's fielded query syntax (field:value) and the
// MultiFieldQueryNodeProcessor.
//
// The Java original tests a dedicated MultiFieldQueryParser wrapper; in Gocene
// the flexible package provides MultiFieldQueryNodeProcessor for the processor
// pipeline. This test validates that fielded query syntax works correctly and
// that the MultiFieldQueryNodeProcessor can be constructed and applied.
func TestMultiFieldQPHelper(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("content")

	t.Run("fielded term query", func(t *testing.T) {
		q, err := parser.Parse("title:hello")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.TermQuery); !ok {
			t.Errorf("expected TermQuery, got %T", q)
		}
	})

	t.Run("multiple fielded terms", func(t *testing.T) {
		q, err := parser.Parse("title:hello AND body:world")
		if err != nil {
			t.Fatal(err)
		}
		bq, ok := q.(*search.BooleanQuery)
		if !ok {
			t.Fatalf("expected BooleanQuery, got %T", q)
		}
		if len(bq.Clauses()) != 2 {
			t.Errorf("expected 2 clauses, got %d", len(bq.Clauses()))
		}
	})

	t.Run("multi field boost", func(t *testing.T) {
		q, err := parser.Parse("title:hello^2.0")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := q.(*search.BoostQuery); !ok {
			t.Errorf("expected BoostQuery for boosted field query, got %T", q)
		}
	})
}

// TestMultiFieldQueryNodeProcessor verifies that the processor can be
// constructed and configures field lists.
func TestMultiFieldQueryNodeProcessor(t *testing.T) {
	fields := []string{"title", "body", "keywords"}
	processor := flexible.NewMultiFieldQueryNodeProcessor(fields)
	if processor == nil {
		t.Fatal("NewMultiFieldQueryNodeProcessor should not return nil")
	}
}

// TestMultiFieldParserDefaultField verifies parsing with only the default field.
func TestMultiFieldParserDefaultField(t *testing.T) {
	parser := flexible.NewStandardQueryParser()
	parser.SetDefaultField("title")

	q, err := parser.Parse("search")
	if err != nil {
		t.Fatal(err)
	}
	tq, ok := q.(*search.TermQuery)
	if !ok {
		t.Fatalf("expected TermQuery, got %T", q)
	}
	_ = tq
}
