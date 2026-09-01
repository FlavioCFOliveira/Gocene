package components

import (
	"github.com/FlavioCFOliveira/Gocene/luke/models/search"
)

// QueryParserPane implements QueryParserTabOperator.
type QueryParserPane struct {
	searchableFields      []string
	rangeSearchableFields []string
	defaultField          string
}

func NewQueryParserPane() *QueryParserPane {
	return &QueryParserPane{}
}

func (p *QueryParserPane) SetSearchableFields(fields []string) {
	p.searchableFields = fields
}

func (p *QueryParserPane) SetRangeSearchableFields(fields []string) {
	p.rangeSearchableFields = fields
}

func (p *QueryParserPane) GetConfig() *search.QueryParserConfig {
	return &search.QueryParserConfig{
		// searchableFields: p.searchableFields,
		// rangeSearchableFields: p.rangeSearchableFields,
	}
}

func (p *QueryParserPane) GetDefaultField() string {
	return p.defaultField
}
