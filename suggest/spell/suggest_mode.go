package spell

// SuggestMode controls how an existing term influences spell-check
// suggestions. Mirrors org.apache.lucene.search.spell.SuggestMode.
type SuggestMode int

const (
	// SuggestWhenNotInIndex suggests for every input regardless of presence.
	SuggestWhenNotInIndex SuggestMode = iota
	// SuggestMoreFrequentlyThanExisting suggests only when the candidate is
	// more frequent than the input term.
	SuggestMoreFrequentlyThanExisting
	// SuggestAlways suggests even when the term already exists.
	SuggestAlways
)
