package intervalfn

import (
	"fmt"
	"regexp"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
)

var whitespaceRegex = regexp.MustCompile(`[\s]`)

// AnalyzedText is a node that represents an analyzed text interval source.
type AnalyzedText struct {
	term string
}

// NewAnalyzedText creates a new AnalyzedText node.
func NewAnalyzedText(term string) *AnalyzedText {
	return &AnalyzedText{
		term: term,
	}
}

// ToIntervalSource converts the node to an IntervalsSource.
func (at *AnalyzedText) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	gaps := 0
	ordered := true
	source, err := intervals.AnalyzedText(at.term, analyzer, field, gaps, ordered)
	if err != nil {
		// Faithful to Lucene's throw new RuntimeException(e)
		panic(err)
	}
	return source
}

// String returns the string representation of the node.
func (at *AnalyzedText) String() string {
	if requiresQuotes(at.term) {
		return fmt.Sprintf(`"%s"`, at.term)
	}
	return at.term
}

func requiresQuotes(term string) bool {
	return whitespaceRegex.MatchString(term)
}
