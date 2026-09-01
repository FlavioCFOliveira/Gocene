package components

import (
	"github.com/FlavioCFOliveira/Gocene/luke/models/search"
)

// QueryParserTabOperator is the operator for the QueryParser tab.
type QueryParserTabOperator interface {
	ComponentOperator

	SetSearchableFields(searchableFields []string)
	SetRangeSearchableFields(rangeSearchableFields []string)
	GetConfig() *search.QueryParserConfig
	GetDefaultField() string
}
