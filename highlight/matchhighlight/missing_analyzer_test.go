package matchhighlight

// Port of org.apache.lucene.search.matchhighlight.MissingAnalyzer.
//
// The Java original is a test helper that panics when any real analyzer
// method is called, ensuring tests that should not reach the analysis path
// never accidentally do so.  The Go port mirrors that contract.

import (
	"testing"
)

// MissingAnalyzer panics whenever its analysis methods are called.  It is
// used in tests that must not reach the analysis path — calling Tokenize or
// InitReader signals a misconfiguration.
//
// Mirrors org.apache.lucene.search.matchhighlight.MissingAnalyzer.
type MissingAnalyzer struct{}

// GetOffsetGap returns 0 (safe to call — mirrors the Java override).
func (a *MissingAnalyzer) GetOffsetGap(_ string) int { return 0 }

// GetPositionIncrementGap returns 0 (safe to call).
func (a *MissingAnalyzer) GetPositionIncrementGap(_ string) int { return 0 }

// Tokenize panics — it must never be called in tests that use
// MissingAnalyzer.
func (a *MissingAnalyzer) Tokenize(fieldName, _ string) {
	panic("matchhighlight: field must have an explicit analyzer: " + fieldName)
}

// -- tests -------------------------------------------------------------------

func TestMissingAnalyzer_GetOffsetGap(t *testing.T) {
	a := &MissingAnalyzer{}
	for _, field := range []string{"title", "body", ""} {
		if got := a.GetOffsetGap(field); got != 0 {
			t.Errorf("GetOffsetGap(%q) = %d, want 0", field, got)
		}
	}
}

func TestMissingAnalyzer_GetPositionIncrementGap(t *testing.T) {
	a := &MissingAnalyzer{}
	if got := a.GetPositionIncrementGap("anyField"); got != 0 {
		t.Errorf("GetPositionIncrementGap = %d, want 0", got)
	}
}

func TestMissingAnalyzer_TokenizePanics(t *testing.T) {
	a := &MissingAnalyzer{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected Tokenize to panic but it did not")
		}
	}()
	a.Tokenize("myField", "some text")
}
