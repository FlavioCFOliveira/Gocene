package uhighlight

// Port of
// org.apache.lucene.search.uhighlight.visibility.TestUnifiedHighlighterExtensibility.
//
// The Java test verifies that FieldOffsetStrategy and UnifiedHighlighter can
// be extended from outside their package (i.e. that the correct methods are
// accessible and overridable).  The Go port exercises the same extensibility
// concern by implementing custom FieldOffsetStrategy types from within the
// package's test suite.

import "testing"

// customExtensibilityStrategy is a concrete FieldOffsetStrategy subtype
// defined outside the package (here, in a test file), mirroring the anonymous
// class instantiated in testFieldOffsetStrategyExtensibility.
type customExtensibilityStrategy struct {
	BaseFieldOffsetStrategy
	source OffsetSource
}

func newCustomExtensibilityStrategy(field string, source OffsetSource) *customExtensibilityStrategy {
	return &customExtensibilityStrategy{
		BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field),
		source:                  source,
	}
}

func (s *customExtensibilityStrategy) GetOffsetSource() OffsetSource { return s.source }

func (s *customExtensibilityStrategy) GetOffsetsEnum(_ any) (OffsetsEnum, error) {
	return NewSliceOffsetsEnum(nil), nil
}

var _ FieldOffsetStrategy = (*customExtensibilityStrategy)(nil)

// -- tests -------------------------------------------------------------------

// TestUnifiedHighlighterVisibility_FieldOffsetStrategyExtensibility mirrors
// testFieldOffsetStrategyExtensibility: a custom concrete strategy must be
// usable as a FieldOffsetStrategy and must return the configured OffsetSource.
func TestUnifiedHighlighterVisibility_FieldOffsetStrategyExtensibility(t *testing.T) {
	for _, src := range AllOffsetSources() {
		strat := newCustomExtensibilityStrategy("field", src)

		if got := strat.GetOffsetSource(); got != src {
			t.Errorf("source=%d: GetOffsetSource() = %d", src, got)
		}
		if got := strat.Field(); got != "field" {
			t.Errorf("source=%d: Field() = %q, want %q", src, got, "field")
		}
		enum, err := strat.GetOffsetsEnum(nil)
		if err != nil {
			t.Errorf("source=%d: GetOffsetsEnum: %v", src, err)
		}
		if enum.Next() {
			t.Errorf("source=%d: expected empty enum", src)
		}
		_ = enum.Close()
	}
}

// TestUnifiedHighlighterVisibility_StrategyFieldAccessor verifies that
// BaseFieldOffsetStrategy.Field() is accessible after embedding, mirroring
// the fact that the Java abstract class field is protected.
func TestUnifiedHighlighterVisibility_StrategyFieldAccessor(t *testing.T) {
	const fieldName = "myField"
	strat := newCustomExtensibilityStrategy(fieldName, OffsetSourceNone)
	if got := strat.Field(); got != fieldName {
		t.Errorf("Field() = %q, want %q", got, fieldName)
	}
}

// TestUnifiedHighlighterVisibility_UHComponents verifies that UHComponents
// exposes its fields, mirroring the components.field() etc. assertions in the
// Java test.
func TestUnifiedHighlighterVisibility_UHComponents(t *testing.T) {
	strat := NewNoOpOffsetStrategy("contents")
	phrase := NewPhraseHelper("contents", nil)
	matchers := []CharArrayMatcher{NewLiteralCharArrayMatcher("fox")}
	comp := NewUHComponents("contents", strat, phrase, nil, matchers)

	if comp.Field != "contents" {
		t.Errorf("Field = %q, want %q", comp.Field, "contents")
	}
	if comp.OffsetStrat == nil {
		t.Error("OffsetStrat should not be nil")
	}
	if comp.PhraseHelper == nil {
		t.Error("PhraseHelper should not be nil")
	}
	if len(comp.Matchers) != 1 {
		t.Errorf("Matchers: want 1, got %d", len(comp.Matchers))
	}
}
