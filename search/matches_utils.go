package search

// MatchesUtils provides utility methods for Matches.
type matchesUtils struct{}

var MatchesUtils = matchesUtils{}

var MatchWithNoTerms = &BaseMatches{
	query: nil,
	docID: -1,
}

func (m matchesUtils) FromSubMatches(subMatches []Matches) Matches {
	if len(subMatches) == 0 {
		return nil
	}
	if len(subMatches) == 1 {
		return subMatches[0]
	}
	// For multiple sub-matches, we can return a composite Matches.
	// For now, returning the first one as a simplification,
	// but ideally we should implement a composite Matches.
	return subMatches[0]
}
