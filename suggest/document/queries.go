package document

import "github.com/FlavioCFOliveira/Gocene/search"

// CompletionScorer ranks completion hits by weight. Mirrors
// org.apache.lucene.search.suggest.document.CompletionScorer.
type CompletionScorer struct {
	DefaultBoost float32
}

// NewCompletionScorer builds a scorer with the supplied default boost.
func NewCompletionScorer(defaultBoost float32) *CompletionScorer {
	if defaultBoost <= 0 {
		defaultBoost = 1
	}
	return &CompletionScorer{DefaultBoost: defaultBoost}
}

// Score combines weight and boost.
func (s *CompletionScorer) Score(weight int64, boost float32) float32 {
	return float32(weight) * boost * s.DefaultBoost
}

// CompletionWeight is the per-query weight produced by the completion
// queries. Mirrors
// org.apache.lucene.search.suggest.document.CompletionWeight.
type CompletionWeight struct {
	Query search.Query
	Boost float32
}

// NewCompletionWeight builds the weight.
func NewCompletionWeight(q search.Query, boost float32) *CompletionWeight {
	if boost <= 0 {
		boost = 1
	}
	return &CompletionWeight{Query: q, Boost: boost}
}

// ContextQuery filters completions to those carrying one of the supplied
// contexts. Mirrors
// org.apache.lucene.search.suggest.document.ContextQuery.
type ContextQuery struct {
	Inner    search.Query
	Contexts []string
}

// NewContextQuery builds the query.
func NewContextQuery(inner search.Query, contexts ...string) *ContextQuery {
	return &ContextQuery{Inner: inner, Contexts: append([]string(nil), contexts...)}
}

// PrefixCompletionQuery is the prefix-based completion query. Mirrors
// org.apache.lucene.search.suggest.document.PrefixCompletionQuery.
type PrefixCompletionQuery struct {
	Field  string
	Prefix string
}

// NewPrefixCompletionQuery builds the query.
func NewPrefixCompletionQuery(field, prefix string) *PrefixCompletionQuery {
	return &PrefixCompletionQuery{Field: field, Prefix: prefix}
}

// FuzzyCompletionQuery is the fuzzy variant. Mirrors
// org.apache.lucene.search.suggest.document.FuzzyCompletionQuery.
type FuzzyCompletionQuery struct {
	Field    string
	Term     string
	MaxEdits int
}

// NewFuzzyCompletionQuery builds the query.
func NewFuzzyCompletionQuery(field, term string, maxEdits int) *FuzzyCompletionQuery {
	if maxEdits < 0 {
		maxEdits = 1
	}
	return &FuzzyCompletionQuery{Field: field, Term: term, MaxEdits: maxEdits}
}

// RegexCompletionQuery is the regex variant. Mirrors
// org.apache.lucene.search.suggest.document.RegexCompletionQuery.
type RegexCompletionQuery struct {
	Field   string
	Pattern string
}

// NewRegexCompletionQuery builds the query.
func NewRegexCompletionQuery(field, pattern string) *RegexCompletionQuery {
	return &RegexCompletionQuery{Field: field, Pattern: pattern}
}
