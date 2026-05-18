package surround

import "fmt"

// TooManyBasicQueries is returned by BasicQueryFactory when expanding a
// surround query would produce more basic Lucene queries than the configured
// maximum. It mirrors org.apache.lucene.queryparser.surround.query.TooManyBasicQueries.
type TooManyBasicQueries struct {
	MaxBasicQueries int
}

func (e *TooManyBasicQueries) Error() string {
	return fmt.Sprintf("Exceeded maximum of %d basic queries.", e.MaxBasicQueries)
}

// NewTooManyBasicQueries builds the limit-exceeded error.
func NewTooManyBasicQueries(maxBasicQueries int) *TooManyBasicQueries {
	return &TooManyBasicQueries{MaxBasicQueries: maxBasicQueries}
}
