package components

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/luke/models/search"
)

// MLTTabOperator is the operator for the MLT tab.
type MLTTabOperator interface {
	ComponentOperator

	SetAnalyzer(analyzer analysis.Analyzer)
	SetFields(fields []string)
	GetConfig() *search.MLTConfig
}
