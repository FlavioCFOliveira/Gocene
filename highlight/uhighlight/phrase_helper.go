package uhighlight

// PhraseInfo describes a single position-sensitive phrase query that the
// analysis offset strategy should highlight as a contiguous span instead of
// individual term matches.
type PhraseInfo struct {
	Field     string
	Terms     []string
	Positions []int
	Slop      int
}

// PhraseHelper tracks which terms participate in phrase queries and exposes
// helpers callers use to deduce whether a hit is part of a phrase. Mirrors
// org.apache.lucene.search.uhighlight.PhraseHelper.
type PhraseHelper struct {
	Field                  string
	PhraseTerms            map[string]bool
	HasPositionSensitivity bool
}

// NewPhraseHelper builds the helper.
func NewPhraseHelper(field string, phraseTerms []string) *PhraseHelper {
	pt := make(map[string]bool, len(phraseTerms))
	for _, t := range phraseTerms {
		pt[t] = true
	}
	return &PhraseHelper{
		Field:                  field,
		PhraseTerms:            pt,
		HasPositionSensitivity: len(pt) > 0,
	}
}

// HasPhraseQuery reports whether any registered term comes from a phrase.
func (p *PhraseHelper) HasPhraseQuery() bool { return p.HasPositionSensitivity }

// IsPhraseTerm reports whether term participates in a phrase.
func (p *PhraseHelper) IsPhraseTerm(term string) bool {
	return p.PhraseTerms[term]
}
