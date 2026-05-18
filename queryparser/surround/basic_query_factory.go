package surround

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BasicQueryFactory hands out basic SpanTermQuery instances and tracks how
// many it has produced; once the configured limit is reached every subsequent
// request errors out with TooManyBasicQueries. Mirrors
// org.apache.lucene.queryparser.surround.query.BasicQueryFactory.
type BasicQueryFactory struct {
	maxBasicQueries  int
	queriesMadeCount int
}

// DefaultMaxBasicQueries is the limit Lucene applies when callers do not
// specify their own ceiling.
const DefaultMaxBasicQueries = 1024

// NewBasicQueryFactory builds a factory with the default query limit.
func NewBasicQueryFactory() *BasicQueryFactory {
	return NewBasicQueryFactoryWithLimit(DefaultMaxBasicQueries)
}

// NewBasicQueryFactoryWithLimit builds a factory with the supplied query limit.
func NewBasicQueryFactoryWithLimit(max int) *BasicQueryFactory {
	return &BasicQueryFactory{maxBasicQueries: max}
}

// GetMaxBasicQueries returns the configured query ceiling.
func (f *BasicQueryFactory) GetMaxBasicQueries() int { return f.maxBasicQueries }

// GetQueriesMade returns the number of queries handed out so far.
func (f *BasicQueryFactory) GetQueriesMade() int { return f.queriesMadeCount }

// MakeBasicTermQuery returns a search.TermQuery for the given field/text,
// counting against the basic-query budget.
func (f *BasicQueryFactory) MakeBasicTermQuery(field, text string) (search.Query, error) {
	if err := f.tickBasicQueryBudget(); err != nil {
		return nil, err
	}
	return search.NewTermQuery(index.NewTerm(field, text)), nil
}

// MakeSpanTermQuery returns a search.SpanTermQuery for the given field/text,
// counting against the basic-query budget.
func (f *BasicQueryFactory) MakeSpanTermQuery(field, text string) (*search.SpanTermQuery, error) {
	if err := f.tickBasicQueryBudget(); err != nil {
		return nil, err
	}
	return search.NewSpanTermQuery(index.NewTerm(field, text)), nil
}

func (f *BasicQueryFactory) tickBasicQueryBudget() error {
	f.queriesMadeCount++
	if f.queriesMadeCount > f.maxBasicQueries {
		return NewTooManyBasicQueries(f.maxBasicQueries)
	}
	return nil
}
