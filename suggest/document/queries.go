package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

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

// CompletionQuery is the base completion query. Mirrors
// org.apache.lucene.search.suggest.document.CompletionQuery.
type CompletionQuery struct {
	Term         string
	Filter       search.Query
	DefaultBoost float32
}

func NewCompletionQuery(term string, filter search.Query) *CompletionQuery {
	return &CompletionQuery{
		Term:         term,
		Filter:       filter,
		DefaultBoost: 1.0,
	}
}

func (q *CompletionQuery) String() string {
	return fmt.Sprintf("CompletionQuery(term=%s, filter=%v)", q.Term, q.Filter)
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
	Inner    *CompletionQuery
	Contexts map[string]ContextMetaData
	MatchAll bool
}

type ContextMetaData struct {
	Boost float32
	Exact bool
}

// NewContextQuery builds the query.
func NewContextQuery(inner *CompletionQuery, contexts ...string) *ContextQuery {
	ctxMap := make(map[string]ContextMetaData)
	for _, c := range contexts {
		ctxMap[c] = ContextMetaData{Boost: 1.0, Exact: true}
	}
	return &ContextQuery{
		Inner:    inner,
		Contexts: ctxMap,
		MatchAll: false,
	}
}

func (q *ContextQuery) AddContext(context string, boost float32, exact bool) {
	q.Contexts[context] = ContextMetaData{Boost: boost, Exact: exact}
}

func (q *ContextQuery) AddAllContexts() {
	q.MatchAll = true
}

// CreateWeight produces the weight for the context query.
func (q *ContextQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*ContextCompletionWeight, error) {
	innerWeight := &CompletionWeight{
		Query: q.Inner,
		Boost: boost,
	}

	ctxAutomaton := q.toContextAutomaton()

	return &ContextCompletionWeight{
		Query:          q,
		Automaton:      ctxAutomaton,
		InnerWeight:    innerWeight,
		ContextMap:     q.Contexts,
		ContextLengths: q.getContextLengths(),
		CurrentBoost:   0,
		CurrentContext: "",
	}, nil
}

// toContextAutomaton is the port of the private static
// ContextQuery.toContextAutomaton(Map<IntsRef, ContextMetaData>, boolean) in
// Apache Lucene 10.5.0. Java reaches the Operations / Automata statics through
// their class names; in Gocene those statics are plain package-level functions
// in util/automaton, so they are spelled automaton.Repeat, automaton.MakeChar
// and so on.
func (q *ContextQuery) toContextAutomaton() *automaton.Automaton {
	matchAllAutomaton := automaton.Repeat(automaton.MakeAnyString())
	sep := automaton.MakeChar(CONTEXT_SEPARATOR)
	if q.MatchAll || len(q.Contexts) == 0 {
		return automaton.Concatenate([]*automaton.Automaton{matchAllAutomaton, sep})
	}

	var automataList []*automaton.Automaton
	for ctx, meta := range q.Contexts {
		contextAutomaton := []*automaton.Automaton{automaton.MakeString(ctx)}
		if !meta.Exact {
			contextAutomaton = append(contextAutomaton, matchAllAutomaton)
		}
		contextAutomaton = append(contextAutomaton, sep)
		automataList = append(automataList, automaton.Concatenate(contextAutomaton))
	}
	return automaton.Union(automataList)
}

func (q *ContextQuery) getContextLengths() []int {
	lengths := make([]int, 0, len(q.Contexts))
	for ctx := range q.Contexts {
		lengths = append(lengths, len(ctx))
	}
	for i := 0; i < len(lengths); i++ {
		for j := i + 1; j < len(lengths); j++ {
			if lengths[i] < lengths[j] {
				lengths[i], lengths[j] = lengths[j], lengths[i]
			}
		}
	}
	return lengths
}

// ContextCompletionWeight is the weight for the context query.
type ContextCompletionWeight struct {
	Query          *ContextQuery
	Automaton      *automaton.Automaton
	InnerWeight    *CompletionWeight
	ContextMap     map[string]ContextMetaData
	ContextLengths []int
	CurrentBoost   float32
	CurrentContext string
}

func (w *ContextCompletionWeight) SetNextMatch(pathPrefix []int) {
	for _, length := range w.ContextLengths {
		if length > len(pathPrefix) {
			continue
		}
		ctx := string(pathPrefix[:length])
		if meta, ok := w.ContextMap[ctx]; ok {
			w.CurrentBoost = meta.Boost
			w.CurrentContext = ctx
			return
		}
	}
	w.CurrentBoost = 0
	w.CurrentContext = ""
}

func (w *ContextCompletionWeight) Boost() float32 {
	return w.CurrentBoost + w.InnerWeight.Boost
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
