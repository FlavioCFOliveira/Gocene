package document

// NRTSuggester is the near-real-time suggester that holds an in-memory
// completion index over a SearchManager-style snapshot. Mirrors
// org.apache.lucene.search.suggest.document.NRTSuggester.
type NRTSuggester struct {
	entries []NRTEntry
}

// NRTEntry is a single (key, weight, payload, contexts) tuple cached by the
// NRT suggester.
type NRTEntry struct {
	Key      string
	Weight   int64
	Payload  []byte
	Contexts [][]byte
}

// NewNRTSuggester builds an empty NRTSuggester.
func NewNRTSuggester() *NRTSuggester { return &NRTSuggester{} }

// Add records an entry.
func (s *NRTSuggester) Add(entry NRTEntry) { s.entries = append(s.entries, entry) }

// All returns a copy of the recorded entries.
func (s *NRTSuggester) All() []NRTEntry {
	out := make([]NRTEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

// SuggestIndexSearcher is the search-side facade exposed to callers. Mirrors
// org.apache.lucene.search.suggest.document.SuggestIndexSearcher.
type SuggestIndexSearcher struct {
	Suggester *NRTSuggester
}

// NewSuggestIndexSearcher wires the searcher to a suggester.
func NewSuggestIndexSearcher(s *NRTSuggester) *SuggestIndexSearcher {
	return &SuggestIndexSearcher{Suggester: s}
}
