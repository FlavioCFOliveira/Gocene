package complexphrase

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestPlainPhraseStaysClassic(t *testing.T) {
	p := NewComplexPhraseQueryParser("body", analysis.NewStandardAnalyzer())
	q, err := p.Parse(`"hello world"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.PhraseQuery); !ok {
		t.Errorf("got %T, want PhraseQuery", q)
	}
}

func TestPhraseWithWildcardBecomesSpan(t *testing.T) {
	p := NewComplexPhraseQueryParser("body", analysis.NewStandardAnalyzer())
	q, err := p.Parse(`"hello wo*"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanNearQuery); !ok {
		t.Errorf("got %T, want SpanNearQuery", q)
	}
}

func TestPhraseWithQuestionMarkBecomesSpan(t *testing.T) {
	p := NewComplexPhraseQueryParser("body", analysis.NewStandardAnalyzer())
	q, err := p.Parse(`"f?o bar"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanNearQuery); !ok {
		t.Errorf("got %T, want SpanNearQuery", q)
	}
}

func TestPlainTermStaysClassic(t *testing.T) {
	p := NewComplexPhraseQueryParser("body", analysis.NewStandardAnalyzer())
	q, err := p.Parse("hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.TermQuery); !ok {
		t.Errorf("got %T", q)
	}
}

func TestSinglePhraseTokenWithWildcardCollapses(t *testing.T) {
	p := NewComplexPhraseQueryParser("body", analysis.NewStandardAnalyzer())
	q, err := p.Parse(`"foo*"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(*search.SpanMultiTermQueryWrapper); !ok {
		t.Errorf("got %T, want SpanMultiTermQueryWrapper", q)
	}
}

func TestExtractComplexPhrasesPreservesNonComplex(t *testing.T) {
	p := NewComplexPhraseQueryParser("body", analysis.NewStandardAnalyzer())
	out, phrases := p.extractComplexPhrases(`"hello world" simple`)
	if len(phrases) != 0 {
		t.Errorf("phrases = %d, want 0", len(phrases))
	}
	if out != `"hello world" simple` {
		t.Errorf("out = %q", out)
	}
}

func TestExtractComplexPhrasesIdentifiesWildcards(t *testing.T) {
	p := NewComplexPhraseQueryParser("body", analysis.NewStandardAnalyzer())
	out, phrases := p.extractComplexPhrases(`"a b*" simple`)
	if len(phrases) != 1 {
		t.Fatalf("phrases = %d", len(phrases))
	}
	if !contains(out, phrases[0].Placeholder) {
		t.Errorf("placeholder missing from %q", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
