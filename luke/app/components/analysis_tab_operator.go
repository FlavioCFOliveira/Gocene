package components

import (
	analysismodels "github.com/FlavioCFOliveira/Gocene/luke/models/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// AnalysisTabOperator is the operator for the Analysis tab.
type AnalysisTabOperator interface {
	ComponentOperator

	SetAnalyzerByType(analyzerType string)
	SetAnalyzerByCustomConfiguration(config *analysismodels.CustomAnalyzerConfig)
	GetCurrentAnalyzer() analysis.Analyzer
}
