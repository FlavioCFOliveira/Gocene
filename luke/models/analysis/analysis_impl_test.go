package analysis

import (
	"testing"
)

func TestAnalysisImpl_Analyze(t *testing.T) {
	impl := NewAnalysisImpl()
	text := "The quick brown fox jumps over the lazy dog"
	tokens, err := impl.Analyze(text)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(tokens) == 0 {
		t.Error("expected tokens, got 0")
	}

	expectedTerm := "the"
	found := false
	for _, token := range tokens {
		if token.Term() == expectedTerm {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected token %q not found", expectedTerm)
	}
}

func TestAnalysisImpl_GetAvailable(t *testing.T) {
	impl := NewAnalysisImpl()
	if impl.GetAvailableCharFilters() == nil {
		t.Error("expected non-nil char filters")
	}
	if impl.GetAvailableTokenizers() == nil {
		t.Error("expected non-nil tokenizers")
	}
	if impl.GetAvailableTokenFilters() == nil {
		t.Error("expected non-nil token filters")
	}
}
