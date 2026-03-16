package highlight

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryScorer scores text fragments based on query terms.
// This is the Go port of Lucene's org.apache.lucene.search.highlight.QueryScorer.
type QueryScorer struct {
	// query is the query to score against
	query search.Query

	// field is the field being highlighted
	field string

	// terms are the extracted query terms
	terms []string

	// termWeights maps terms to their weights
	termWeights map[string]float32

	// maxTermWeight is the maximum term weight
	maxTermWeight float32
}

// NewQueryScorer creates a new QueryScorer for the given query.
func NewQueryScorer(query search.Query) *QueryScorer {
	return NewQueryScorerWithField(query, "")
}

// NewQueryScorerWithField creates a new QueryScorer for the given query and field.
func NewQueryScorerWithField(query search.Query, field string) *QueryScorer {
	qs := &QueryScorer{
		query:       query,
		field:       field,
		terms:       make([]string, 0),
		termWeights: make(map[string]float32),
	}
	qs.extractTerms()
	return qs
}

// extractTerms extracts terms from the query.
func (qs *QueryScorer) extractTerms() {
	// In a full implementation, this would recursively extract terms
	// from the query tree and their weights
	// For now, just initialize
	qs.terms = make([]string, 0)
	qs.termWeights = make(map[string]float32)
}

// GetFragmentScore returns the score for the given fragment.
func (qs *QueryScorer) GetFragmentScore(fragment string) float32 {
	if len(qs.terms) == 0 {
		return 0
	}

	score := float32(0)
	lowerFragment := strings.ToLower(fragment)

	for term, weight := range qs.termWeights {
		lowerTerm := strings.ToLower(term)
		count := strings.Count(lowerFragment, lowerTerm)
		score += float32(count) * weight
	}

	// Normalize by max term weight
	if qs.maxTermWeight > 0 {
		score /= qs.maxTermWeight
	}

	return score
}

// GetQueryTerms returns the query terms being highlighted.
func (qs *QueryScorer) GetQueryTerms() []string {
	result := make([]string, len(qs.terms))
	copy(result, qs.terms)
	return result
}

// GetWeightedTerms returns the terms with their weights.
func (qs *QueryScorer) GetWeightedTerms() map[string]float32 {
	result := make(map[string]float32)
	for term, weight := range qs.termWeights {
		result[term] = weight
	}
	return result
}

// AddTerm adds a term with the given weight.
func (qs *QueryScorer) AddTerm(term string, weight float32) {
	qs.terms = append(qs.terms, term)
	qs.termWeights[term] = weight

	if weight > qs.maxTermWeight {
		qs.maxTermWeight = weight
	}
}

// RemoveTerm removes a term.
func (qs *QueryScorer) RemoveTerm(term string) {
	delete(qs.termWeights, term)

	// Rebuild terms slice
	newTerms := make([]string, 0, len(qs.termWeights))
	for t := range qs.termWeights {
		newTerms = append(newTerms, t)
	}
	qs.terms = newTerms

	// Recalculate max weight
	qs.maxTermWeight = 0
	for _, w := range qs.termWeights {
		if w > qs.maxTermWeight {
			qs.maxTermWeight = w
		}
	}
}

// GetQuery returns the query being scored.
func (qs *QueryScorer) GetQuery() search.Query {
	return qs.query
}

// GetField returns the field being highlighted.
func (qs *QueryScorer) GetField() string {
	return qs.field
}

// IsTokenized returns true if the given text should be tokenized.
func (qs *QueryScorer) IsTokenized(text string) bool {
	// Simple heuristic: if text contains spaces, it's tokenized
	return strings.Contains(text, " ")
}

// Tokenize tokenizes the given text into terms.
func (qs *QueryScorer) Tokenize(text string) []string {
	// Simple tokenization by whitespace and punctuation
	var terms []string
	var current strings.Builder

	for _, r := range text {
		if isTokenChar(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				terms = append(terms, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		terms = append(terms, current.String())
	}

	return terms
}

// isTokenChar returns true if the rune is a valid token character.
func isTokenChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9')
}

// ScoreTerm scores a single term occurrence.
func (qs *QueryScorer) ScoreTerm(term string) float32 {
	if weight, ok := qs.termWeights[term]; ok {
		return weight
	}
	return 0
}

// QueryTermScorer scores individual query terms.
type QueryTermScorer struct {
	// term is the query term
	term string

	// weight is the term weight
	weight float32
}

// NewQueryTermScorer creates a new QueryTermScorer.
func NewQueryTermScorer(term string, weight float32) *QueryTermScorer {
	return &QueryTermScorer{
		term:   term,
		weight: weight,
	}
}

// GetTerm returns the term.
func (qts *QueryTermScorer) GetTerm() string {
	return qts.term
}

// GetWeight returns the weight.
func (qts *QueryTermScorer) GetWeight() float32 {
	return qts.weight
}

// ScoreFragment scores a fragment for this term.
func (qts *QueryTermScorer) ScoreFragment(fragment string) float32 {
	lowerFragment := strings.ToLower(fragment)
	lowerTerm := strings.ToLower(qts.term)
	count := strings.Count(lowerFragment, lowerTerm)
	return float32(count) * qts.weight
}

// ScoringResult represents the result of scoring a fragment.
type ScoringResult struct {
	// Fragment is the scored fragment
	Fragment string

	// Score is the fragment score
	Score float32

	// MatchedTerms are the terms that matched
	MatchedTerms []string
}

// NewScoringResult creates a new ScoringResult.
func NewScoringResult(fragment string, score float32) *ScoringResult {
	return &ScoringResult{
		Fragment:     fragment,
		Score:        score,
		MatchedTerms: make([]string, 0),
	}
}

// AddMatchedTerm adds a matched term.
func (sr *ScoringResult) AddMatchedTerm(term string) {
	sr.MatchedTerms = append(sr.MatchedTerms, term)
}

// GetMatchedTermCount returns the number of matched terms.
func (sr *ScoringResult) GetMatchedTermCount() int {
	return len(sr.MatchedTerms)
}
