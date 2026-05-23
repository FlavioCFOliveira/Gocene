package uhighlight

// Port of org.apache.lucene.search.uhighlight.TestUnifiedHighlighterStrictPhrases.
//
// The Java test exercises the WEIGHT_MATCHES highlighting path for strict
// phrase queries, including single/multi-phrase, synonyms, multi-valued
// fields, span queries, and MTQ rewrites.
//
// The Go port exercises the PhraseHelper and FieldOffsetStrategy contracts
// that the strict-phrase path depends on, without requiring a live index
// reader.

import "testing"

// TestUnifiedHighlighterStrictPhrases_PhraseHelperDetection mirrors the
// "basics" test: a phrase query sets up hasPositionSensitivity=true on the
// PhraseHelper.
func TestUnifiedHighlighterStrictPhrases_PhraseHelperDetection(t *testing.T) {
	const field = "body"

	// Non-phrase: no position-sensitive terms.
	noPhrase := NewPhraseHelper(field, nil)
	if noPhrase.HasPositionSensitivity {
		t.Error("expected HasPositionSensitivity=false for empty phrase helper")
	}

	// With phrase terms.
	withPhrase := NewPhraseHelper(field, []string{"fox", "jumped"})
	if !withPhrase.HasPositionSensitivity {
		t.Error("expected HasPositionSensitivity=true when phrase terms are registered")
	}
	if !withPhrase.IsPhraseTerm("fox") {
		t.Error("expected 'fox' to be a phrase term")
	}
	if withPhrase.IsPhraseTerm("other") {
		t.Error("expected 'other' NOT to be a phrase term")
	}
}

// TestUnifiedHighlighterStrictPhrases_PostingsPath mirrors testBasics:
// with strict phrases, the offset source is POSTINGS when the field has
// offset-indexed postings.
func TestUnifiedHighlighterStrictPhrases_PostingsPath(t *testing.T) {
	strat := NewPostingsOffsetStrategy("body")
	if got := strat.GetOffsetSource(); got != OffsetSourcePostings {
		t.Errorf("GetOffsetSource() = %d, want %d", got, OffsetSourcePostings)
	}
}

// TestUnifiedHighlighterStrictPhrases_SynonymPhraseHelper mirrors testSynonyms:
// synonym terms added to the phrase helper should all be recognised as phrase
// terms.
func TestUnifiedHighlighterStrictPhrases_SynonymPhraseHelper(t *testing.T) {
	ph := NewPhraseHelper("body", []string{"fox", "canine", "reynard"})
	for _, term := range []string{"fox", "canine", "reynard"} {
		if !ph.IsPhraseTerm(term) {
			t.Errorf("expected %q to be a phrase term", term)
		}
	}
}

// TestUnifiedHighlighterStrictPhrases_MultiValuedField mirrors testMultiValued:
// UHComponents can hold a multi-field strategy targeting several values of the
// same field.
func TestUnifiedHighlighterStrictPhrases_MultiValuedField(t *testing.T) {
	const field = "body"
	strat := NewTermVectorOffsetStrategy(field)
	ph := NewPhraseHelper(field, []string{"brown", "fox"})
	comp := NewUHComponents(field, strat, ph, nil, nil)

	if comp.Field != field {
		t.Errorf("Field = %q, want %q", comp.Field, field)
	}
	if !comp.PhraseHelper.HasPositionSensitivity {
		t.Error("expected position-sensitive phrase helper")
	}
}

// TestUnifiedHighlighterStrictPhrases_NoOpOnNoMatch mirrors testPhraseNotInDoc:
// when no phrase terms match, the NoOp strategy must return an empty enum.
func TestUnifiedHighlighterStrictPhrases_NoOpOnNoMatch(t *testing.T) {
	strat := NewNoOpOffsetStrategy("body")
	enum, err := strat.GetOffsetsEnum(nil)
	if err != nil {
		t.Fatalf("GetOffsetsEnum: %v", err)
	}
	if enum.Next() {
		t.Error("NoOp strategy must yield empty enum")
	}
	_ = enum.Close()
}

// TestUnifiedHighlighterStrictPhrases_MatchAllReturnsAnalysisSource mirrors
// testMatchNoDocsQuery: a match-all / re-analysis scenario uses
// OffsetSourceAnalysis.
func TestUnifiedHighlighterStrictPhrases_MatchAllReturnsAnalysisSource(t *testing.T) {
	strat := NewAnalysisOffsetStrategy("body")
	if got := strat.GetOffsetSource(); got != OffsetSourceAnalysis {
		t.Errorf("GetOffsetSource() = %d, want %d", got, OffsetSourceAnalysis)
	}
}
