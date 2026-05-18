package analyzing

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// SuggestStopFilterFactory builds SuggestStopFilter instances from a
// configuration map (Lucene-style "factory" pattern). Mirrors
// org.apache.lucene.search.suggest.analyzing.SuggestStopFilterFactory.
type SuggestStopFilterFactory struct {
	Words []string
}

// NewSuggestStopFilterFactory parses the supplied configuration. The only
// recognised entry is "words" — a comma-separated stop-word list.
func NewSuggestStopFilterFactory(config map[string]string) *SuggestStopFilterFactory {
	var words []string
	if v, ok := config["words"]; ok && v != "" {
		for _, w := range strings.Split(v, ",") {
			w = strings.TrimSpace(w)
			if w != "" {
				words = append(words, w)
			}
		}
	}
	return &SuggestStopFilterFactory{Words: words}
}

// Create wraps input with a freshly-built SuggestStopFilter.
func (f *SuggestStopFilterFactory) Create(input analysis.TokenStream) *SuggestStopFilter {
	return NewSuggestStopFilter(input, f.Words)
}
