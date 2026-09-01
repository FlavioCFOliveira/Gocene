package analysis

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

type analysisImpl struct {
	analyzer analysis.Analyzer
}

// NewAnalysisImpl creates a new instance of Analysis.
func NewAnalysisImpl() Analysis {
	return &analysisImpl{
		analyzer: analysis.NewStandardAnalyzer(),
	}
}

func (a *analysisImpl) GetAvailableCharFilters() []string {
	// In Lucene, this calls CharFilterFactory.availableCharFilters().
	// Gocene doesn't have a global registry yet.
	return []string{}
}

func (a *analysisImpl) GetAvailableTokenizers() []string {
	// In Lucene, this calls TokenizerFactory.availableTokenizers().
	return []string{}
}

func (a *analysisImpl) GetAvailableTokenFilters() []string {
	// In Lucene, this calls TokenFilterFactory.availableTokenFilters().
	return []string{}
}

func (a *analysisImpl) CreateAnalyzerFromClassName(analyzerType string) (analysis.Analyzer, error) {
	// In Lucene, this uses reflection to instantiate the class.
	// Gocene does not support dynamic instantiation by class name.
	return nil, fmt.Errorf("dynamic analyzer instantiation by class name not supported in Go port: %s", analyzerType)
}

func (a *analysisImpl) BuildCustomAnalyzer(config CustomAnalyzerConfig) (analysis.Analyzer, error) {
	// In Lucene, this uses CustomAnalyzer.Builder.
	// CustomAnalyzer is not yet ported to Gocene.
	return nil, errors.New("CustomAnalyzer is not yet implemented in Gocene")
}

func (a *analysisImpl) Analyze(text string) ([]Token, error) {
	if a.analyzer == nil {
		return nil, errors.New("analyzer is not set")
	}

	reader := strings.NewReader(text)
	stream, err := a.analyzer.TokenStream("field", reader)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var result []Token
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}

		// In Gocene, attributes are accessed via the stream.
		// We need to extract the term and all attributes.
		// For now, we assume the stream supports GetCharTermAttribute.
		var term string
		if ha, ok := stream.(interface{ GetCharTermAttribute() analysis.CharTermAttribute }); ok {
			term = ha.GetCharTermAttribute().String()
		}

		// Attribute extraction is complex in Gocene because there's no reflection-based visit.
		// For now, we only collect the term.
		result = append(result, Token{
			term:       term,
			attributes: []TokenAttribute{},
		})
	}
	_ = stream.End()

	return result, nil
}

func (a *analysisImpl) CurrentAnalyzer() (analysis.Analyzer, error) {
	if a.analyzer == nil {
		return nil, errors.New("analyzer is not set")
	}
	return a.analyzer, nil
}

func (a *analysisImpl) AddExternalJars(jarFiles []string) error {
	// In Lucene, this uses URLClassLoader.
	// Not supported in Go.
	return errors.New("loading external JARs is not supported in the Go port")
}

func (a *analysisImpl) AnalyzeStepByStep(text string) (*StepByStepResult, error) {
	// This requires CustomAnalyzer and a way to iterate factories.
	// Not implemented yet.
	return nil, errors.New("analyzeStepByStep is not yet implemented (requires CustomAnalyzer)")
}
