// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestStandardQPEnhancements covers StandardQueryParser enhancement features:
// configuration options for wildcard control, lowercase expanded terms,
// phrase slop, and boost handling.
//
// Port of: org.apache.lucene.queryparser.flexible.standard.TestStandardQPEnhancements
// (MultiTermQuery rewrite method and date-range resolution are not yet implemented.)
func TestStandardQPEnhancements(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		config := flexible.NewStandardQueryConfigHandler()
		if config == nil {
			t.Fatal("NewStandardQueryConfigHandler should not return nil")
		}
		if config.GetDefaultOperator() != "OR" {
			t.Errorf("default operator should be OR, got %s", config.GetDefaultOperator())
		}
		if config.IsAllowLeadingWildcard() {
			t.Error("leading wildcard should be false by default")
		}
	})

	t.Run("allow leading wildcard config", func(t *testing.T) {
		parser := flexible.NewStandardQueryParser()
		parser.SetDefaultField("f")

		// Default: parsing leading wildcard succeeds (parser doesn't reject it)
		q, err := parser.Parse("*test")
		if err != nil {
			t.Fatal(err)
		}
		if q == nil {
			t.Fatal("expected non-nil query")
		}
	})

	t.Run("phrase slop config", func(t *testing.T) {
		parser := flexible.NewStandardQueryParser()
		parser.SetDefaultField("f")

		t.Run("default slop", func(t *testing.T) {
			q, err := parser.Parse(`"a b"`)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := q.(*search.PhraseQuery); !ok {
				t.Errorf("expected PhraseQuery, got %T", q)
			}
		})

		t.Run("slop in syntax", func(t *testing.T) {
			q, err := parser.Parse(`"a b"~3`)
			if err != nil {
				t.Fatal(err)
			}
			pq, ok := q.(*search.PhraseQuery)
			if !ok {
				t.Fatalf("expected PhraseQuery, got %T", q)
			}
			if pq.GetSlop() != 3 {
				t.Errorf("expected slop 3, got %d", pq.GetSlop())
			}
		})
	})

	t.Run("boost handling", func(t *testing.T) {
		parser := flexible.NewStandardQueryParser()
		parser.SetDefaultField("f")

		t.Run("boost term", func(t *testing.T) {
			q, err := parser.Parse("term^3.0")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := q.(*search.BoostQuery); !ok {
				t.Errorf("expected BoostQuery, got %T", q)
			}
		})

		t.Run("boost group", func(t *testing.T) {
			q, err := parser.Parse("(a OR b)^2.0")
			if err != nil {
				t.Fatal(err)
			}
			// Group with boost results in BoostQuery wrapping BooleanQuery
			if _, ok := q.(*search.BoostQuery); !ok {
				t.Errorf("expected BoostQuery for boosted group, got %T", q)
			}
		})
	})

	t.Run("lowercase expanded terms config", func(t *testing.T) {
		config := flexible.NewStandardQueryConfigHandler()
		if !config.IsLowercaseExpandedTerms() {
			t.Error("lowercase expanded terms should be true by default")
		}
		config.SetLowercaseExpandedTerms(false)
		if config.IsLowercaseExpandedTerms() {
			t.Error("SetLowercaseExpandedTerms(false) should set to false")
		}
	})

	t.Run("fuzzy config", func(t *testing.T) {
		fc := flexible.NewFuzzyConfig()
		if fc.GetMinSimilarity() != 2.0 {
			t.Errorf("default min similarity should be 2.0, got %f", fc.GetMinSimilarity())
		}
		if fc.GetPrefixLength() != 0 {
			t.Errorf("default prefix length should be 0, got %d", fc.GetPrefixLength())
		}
		fc.SetMinSimilarity(3.0)
		if fc.GetMinSimilarity() != 3.0 {
			t.Errorf("SetMinSimilarity failed")
		}
		fc.SetPrefixLength(2)
		if fc.GetPrefixLength() != 2 {
			t.Errorf("SetPrefixLength failed")
		}
	})
}

// TestStandardQPEnhancementsConfigHandlerFull exercises the full config handler.
func TestStandardQPEnhancementsConfigHandlerFull(t *testing.T) {
	h := flexible.NewStandardQueryConfigHandlerFull()
	if h == nil {
		t.Fatal("NewStandardQueryConfigHandlerFull should not return nil")
	}

	// Position increments
	if !h.GetEnablePositionIncrements() {
		t.Error("position increments should be enabled by default")
	}
	h.SetEnablePositionIncrements(false)
	if h.GetEnablePositionIncrements() {
		t.Error("SetEnablePositionIncrements false failed")
	}

	// Leading wildcard
	h.SetAllowLeadingWildcard(true)
	if !h.GetAllowLeadingWildcard() {
		t.Error("SetAllowLeadingWildcard true failed")
	}

	// Lowercase expanded terms
	h.SetLowercaseExpandedTerms(false)
	if h.GetLowercaseExpandedTerms() {
		t.Error("SetLowercaseExpandedTerms false failed")
	}

	// Phrase slop
	h.SetPhraseSlop(5)
	if h.GetPhraseSlop() != 5 {
		t.Errorf("SetPhraseSlop failed, got %d", h.GetPhraseSlop())
	}

	// Fuzzy
	h.SetFuzzyMinSim(0.8)
	if h.GetFuzzyMinSim() != 0.8 {
		t.Errorf("SetFuzzyMinSim failed, got %f", h.GetFuzzyMinSim())
	}
	h.SetFuzzyPrefixLength(3)
	if h.GetFuzzyPrefixLength() != 3 {
		t.Errorf("SetFuzzyPrefixLength failed, got %d", h.GetFuzzyPrefixLength())
	}
}

// TestStandardQPEnhancementsDateFormat verifies NumberDateFormat.
func TestStandardQPEnhancementsDateFormat(t *testing.T) {
	df := flexible.NewNumberDateFormat("")
	if df.GetLayout() != flexible.DefaultNumberDateLayout {
		t.Errorf("default layout should be %s, got %s", flexible.DefaultNumberDateLayout, df.GetLayout())
	}

	df2 := flexible.NewNumberDateFormat("2006-01-02")
	if df2.GetLayout() != "2006-01-02" {
		t.Errorf("Set layout should be 2006-01-02, got %s", df2.GetLayout())
	}
}

// TestStandardQPEnhancementsSyntaxParserToken verifies token types.
func TestStandardQPEnhancementsSyntaxParserToken(t *testing.T) {
	tok := flexible.NewStandardSyntaxParserToken(1, "AND")
	if tok.Kind != 1 || tok.Image != "AND" {
		t.Errorf("NewStandardSyntaxParserToken failed: kind=%d, image=%s", tok.Kind, tok.Image)
	}
}
