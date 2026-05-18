package document

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// CompletionAnalyzer wraps an analyzer in the configuration the
// suggest/document pipeline expects. Mirrors
// org.apache.lucene.search.suggest.document.CompletionAnalyzer.
type CompletionAnalyzer struct {
	Inner             analysis.Analyzer
	PreserveSep       bool
	PreservePosIncr   bool
	MaxGraphExpansions int
}

// NewCompletionAnalyzer builds the wrapper.
func NewCompletionAnalyzer(inner analysis.Analyzer) *CompletionAnalyzer {
	return &CompletionAnalyzer{
		Inner:              inner,
		PreserveSep:        true,
		PreservePosIncr:    true,
		MaxGraphExpansions: -1,
	}
}

// CompletionTokenStream pairs the analyzer output with a per-token weight.
// Mirrors org.apache.lucene.search.suggest.document.CompletionTokenStream.
type CompletionTokenStream struct {
	Wrapped analysis.TokenStream
	Weight  int64
	Payload []byte
}

// NewCompletionTokenStream wraps stream with the suggester metadata.
func NewCompletionTokenStream(stream analysis.TokenStream, weight int64, payload []byte) *CompletionTokenStream {
	return &CompletionTokenStream{Wrapped: stream, Weight: weight, Payload: append([]byte(nil), payload...)}
}

// IncrementToken forwards to the wrapped stream.
func (s *CompletionTokenStream) IncrementToken() (bool, error) { return s.Wrapped.IncrementToken() }

// End forwards End.
func (s *CompletionTokenStream) End() error { return s.Wrapped.End() }

// Close forwards Close.
func (s *CompletionTokenStream) Close() error { return s.Wrapped.Close() }

var _ analysis.TokenStream = (*CompletionTokenStream)(nil)

// CompletionTerms is the per-field terms view exposed by the suggester. The
// Go port keeps the contract minimal — concrete reading is deferred to the
// codec layer. Mirrors
// org.apache.lucene.search.suggest.document.CompletionTerms.
type CompletionTerms struct {
	Field string
}

// NewCompletionTerms builds a CompletionTerms for field.
func NewCompletionTerms(field string) *CompletionTerms { return &CompletionTerms{Field: field} }

// CompletionsTermsReader is the per-segment reader. Mirrors
// org.apache.lucene.search.suggest.document.CompletionsTermsReader.
type CompletionsTermsReader struct {
	Field string
}

// NewCompletionsTermsReader builds the reader.
func NewCompletionsTermsReader(field string) *CompletionsTermsReader {
	return &CompletionsTermsReader{Field: field}
}

// SuggestField is the indexable field every suggester input goes through.
// Mirrors org.apache.lucene.search.suggest.document.SuggestField.
type SuggestField struct {
	Name    string
	Value   string
	Weight  int
	Payload []byte
}

// NewSuggestField builds a SuggestField.
func NewSuggestField(name, value string, weight int) *SuggestField {
	return &SuggestField{Name: name, Value: value, Weight: weight}
}

// SetPayload stamps a payload on the field.
func (f *SuggestField) SetPayload(p []byte) {
	f.Payload = append([]byte(nil), p...)
}

// ContextSuggestField is the SuggestField variant that also carries a set
// of contexts. Mirrors
// org.apache.lucene.search.suggest.document.ContextSuggestField.
type ContextSuggestField struct {
	*SuggestField
	Contexts [][]byte
}

// NewContextSuggestField builds the field.
func NewContextSuggestField(name, value string, weight int, contexts ...string) *ContextSuggestField {
	ctxs := make([][]byte, len(contexts))
	for i, c := range contexts {
		ctxs[i] = []byte(c)
	}
	return &ContextSuggestField{
		SuggestField: NewSuggestField(name, value, weight),
		Contexts:     ctxs,
	}
}
