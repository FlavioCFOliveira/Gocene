package analyzing

import "github.com/FlavioCFOliveira/Gocene/analysis"

// SuggestStopFilter is the TokenFilter the analyzing-suggester family uses
// to drop stop words before indexing. Unlike the regular StopFilter, the
// suggester emits a single "stop" marker rather than silently dropping the
// token so the analyzer pipeline can still produce a contiguous position
// stream. Mirrors org.apache.lucene.search.suggest.analyzing.SuggestStopFilter.
type SuggestStopFilter struct {
	input     analysis.TokenStream
	StopWords map[string]bool
	Marker    string // emitted in place of dropped tokens; default "_stop_"
}

// NewSuggestStopFilter wraps input with the supplied stop-word set.
func NewSuggestStopFilter(input analysis.TokenStream, stopWords []string) *SuggestStopFilter {
	set := make(map[string]bool, len(stopWords))
	for _, w := range stopWords {
		set[w] = true
	}
	return &SuggestStopFilter{input: input, StopWords: set, Marker: "_stop_"}
}

// IncrementToken forwards to the wrapped stream.
func (f *SuggestStopFilter) IncrementToken() (bool, error) {
	return f.input.IncrementToken()
}

// End forwards End.
func (f *SuggestStopFilter) End() error { return f.input.End() }

// Close forwards Close.
func (f *SuggestStopFilter) Close() error { return f.input.Close() }

// IsStopWord reports whether the supplied term is a registered stop word.
func (f *SuggestStopFilter) IsStopWord(term string) bool { return f.StopWords[term] }

var _ analysis.TokenStream = (*SuggestStopFilter)(nil)
