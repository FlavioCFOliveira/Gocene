package components

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/luke/models/search"
)

// MLTPane implements MLTTabOperator.
type MLTPane struct {
	analyzer analysis.Analyzer
	fields   []string
}

func NewMLTPane() *MLTPane {
	return &MLTPane{}
}

func (p *MLTPane) SetAnalyzer(analyzer analysis.Analyzer) {
	p.analyzer = analyzer
}

func (p *MLTPane) SetFields(fields []string) {
	p.fields = fields
}

func (p *MLTPane) GetConfig() *search.MLTConfig {
	// Return config based on internal state
	return &search.MLTConfig{
		// fields: p.fields,
		// analyzer: p.analyzer,
	}
}
