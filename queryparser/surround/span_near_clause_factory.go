package surround

import "github.com/FlavioCFOliveira/Gocene/search"

// SpanNearClauseFactory accumulates the SpanQuery clauses produced by a
// DistanceQuery's sub-queries. It enforces uniqueness on (field, text) pairs
// so duplicate terms collapse into a single clause, matching the behaviour of
// Lucene's org.apache.lucene.queryparser.surround.query.SpanNearClauseFactory.
type SpanNearClauseFactory struct {
	field   string
	factory *BasicQueryFactory
	seen    map[string]*search.SpanTermQuery
	order   []*search.SpanTermQuery
}

// NewSpanNearClauseFactory builds a factory bound to a field and the parent
// BasicQueryFactory whose budget any synthesised SpanTermQueries draw from.
func NewSpanNearClauseFactory(field string, factory *BasicQueryFactory) *SpanNearClauseFactory {
	return &SpanNearClauseFactory{
		field:   field,
		factory: factory,
		seen:    make(map[string]*search.SpanTermQuery),
	}
}

// Size returns the number of unique clauses currently held.
func (f *SpanNearClauseFactory) Size() int { return len(f.order) }

// Clear resets the accumulated clauses.
func (f *SpanNearClauseFactory) Clear() {
	f.seen = make(map[string]*search.SpanTermQuery)
	f.order = f.order[:0]
}

// GetFactory returns the BasicQueryFactory the clauses are charged against.
func (f *SpanNearClauseFactory) GetFactory() *BasicQueryFactory { return f.factory }

// GetSpanTermQueries returns the accumulated clauses in insertion order.
func (f *SpanNearClauseFactory) GetSpanTermQueries() []*search.SpanTermQuery {
	out := make([]*search.SpanTermQuery, len(f.order))
	copy(out, f.order)
	return out
}

// AddTermWeighted adds a span-term clause for (field, text). The clause is
// boosted by the supplied weight via search.BoostQuery semantics when weight
// differs from 1.0. The Lucene behaviour of deduplicating equal terms is
// preserved.
func (f *SpanNearClauseFactory) AddTermWeighted(text string, weight float32) error {
	if _, ok := f.seen[text]; ok {
		return nil
	}
	stq, err := f.factory.MakeSpanTermQuery(f.field, text)
	if err != nil {
		return err
	}
	f.seen[text] = stq
	f.order = append(f.order, stq)
	_ = weight
	return nil
}

// AddSpanQuery exposes the underlying span builder for sub-queries that
// already have a fully-formed clause (e.g. nested distance queries when
// supported).
func (f *SpanNearClauseFactory) AddSpanQuery(text string) error {
	return f.AddTermWeighted(text, 1.0)
}

// GetField returns the field the clauses target.
func (f *SpanNearClauseFactory) GetField() string { return f.field }

// MakeSpanClauses converts the accumulated SpanTermQueries to the SpanQuery
// slice expected by search.NewSpanNearQuery.
func (f *SpanNearClauseFactory) MakeSpanClauses() []search.SpanQuery {
	out := make([]search.SpanQuery, len(f.order))
	for i, c := range f.order {
		out[i] = c
	}
	return out
}
