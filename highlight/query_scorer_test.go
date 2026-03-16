package highlight

import (
	"testing"
)

func TestNewQueryScorer(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorer(query)

	if qs == nil {
		t.Fatal("Expected QueryScorer to be created")
	}

	if qs.GetQuery() != query {
		t.Error("Expected query to match")
	}
}

func TestNewQueryScorerWithField(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorerWithField(query, "content")

	if qs == nil {
		t.Fatal("Expected QueryScorer to be created")
	}

	if qs.GetField() != "content" {
		t.Errorf("Expected field 'content', got '%s'", qs.GetField())
	}
}

func TestQueryScorerAddTerm(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorer(query)

	qs.AddTerm("test", 1.5)

	terms := qs.GetQueryTerms()
	found := false
	for _, term := range terms {
		if term == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected term 'test' to be added")
	}

	weights := qs.GetWeightedTerms()
	if weights["test"] != 1.5 {
		t.Errorf("Expected weight 1.5, got %f", weights["test"])
	}
}

func TestQueryScorerRemoveTerm(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorer(query)

	qs.AddTerm("test", 1.0)
	qs.AddTerm("example", 2.0)

	qs.RemoveTerm("test")

	terms := qs.GetQueryTerms()
	for _, term := range terms {
		if term == "test" {
			t.Error("Expected term 'test' to be removed")
		}
	}
}

func TestQueryScorerGetFragmentScore(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorer(query)

	qs.AddTerm("test", 1.0)

	// Fragment with term
	score := qs.GetFragmentScore("This is a test.")
	if score != 1.0 {
		t.Errorf("Expected score 1.0, got %f", score)
	}

	// Fragment without term
	score = qs.GetFragmentScore("This is a sentence.")
	if score != 0 {
		t.Errorf("Expected score 0, got %f", score)
	}
}

func TestQueryScorerIsTokenized(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorer(query)

	if !qs.IsTokenized("hello world") {
		t.Error("Expected 'hello world' to be tokenized")
	}

	if qs.IsTokenized("hello") {
		t.Error("Expected 'hello' to not be tokenized")
	}
}

func TestQueryScorerTokenize(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorer(query)

	terms := qs.Tokenize("Hello, world! This is a test.")

	if len(terms) == 0 {
		t.Error("Expected some terms")
	}

	// Check that punctuation was removed
	for _, term := range terms {
		if term == "" {
			t.Error("Expected non-empty term")
		}
	}
}

func TestQueryScorerScoreTerm(t *testing.T) {
	query := &MockQuery{}
	qs := NewQueryScorer(query)

	qs.AddTerm("test", 2.0)

	score := qs.ScoreTerm("test")
	if score != 2.0 {
		t.Errorf("Expected score 2.0, got %f", score)
	}

	score = qs.ScoreTerm("nonexistent")
	if score != 0 {
		t.Errorf("Expected score 0 for non-existent term, got %f", score)
	}
}

func TestNewQueryTermScorer(t *testing.T) {
	qts := NewQueryTermScorer("test", 1.5)

	if qts == nil {
		t.Fatal("Expected QueryTermScorer to be created")
	}

	if qts.GetTerm() != "test" {
		t.Errorf("Expected term 'test', got '%s'", qts.GetTerm())
	}

	if qts.GetWeight() != 1.5 {
		t.Errorf("Expected weight 1.5, got %f", qts.GetWeight())
	}
}

func TestQueryTermScorerScoreFragment(t *testing.T) {
	qts := NewQueryTermScorer("test", 2.0)

	score := qts.ScoreFragment("This is a test.")
	if score != 2.0 {
		t.Errorf("Expected score 2.0, got %f", score)
	}

	score = qts.ScoreFragment("No match here.")
	if score != 0 {
		t.Errorf("Expected score 0, got %f", score)
	}

	// Multiple occurrences
	score = qts.ScoreFragment("Test test test.")
	if score != 6.0 {
		t.Errorf("Expected score 6.0, got %f", score)
	}
}

func TestNewScoringResult(t *testing.T) {
	sr := NewScoringResult("test fragment", 1.5)

	if sr == nil {
		t.Fatal("Expected ScoringResult to be created")
	}

	if sr.Fragment != "test fragment" {
		t.Errorf("Expected fragment 'test fragment', got '%s'", sr.Fragment)
	}

	if sr.Score != 1.5 {
		t.Errorf("Expected score 1.5, got %f", sr.Score)
	}

	if len(sr.MatchedTerms) != 0 {
		t.Errorf("Expected 0 matched terms, got %d", len(sr.MatchedTerms))
	}
}

func TestScoringResultAddMatchedTerm(t *testing.T) {
	sr := NewScoringResult("test", 1.0)

	sr.AddMatchedTerm("test")
	sr.AddMatchedTerm("example")

	if sr.GetMatchedTermCount() != 2 {
		t.Errorf("Expected 2 matched terms, got %d", sr.GetMatchedTermCount())
	}
}
