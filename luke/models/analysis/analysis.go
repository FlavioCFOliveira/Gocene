package analysis

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// Token is a holder for a token.
type Token struct {
	term       string
	attributes []TokenAttribute
}

func (t *Token) Term() string {
	return t.term
}

func (t *Token) Attributes() []TokenAttribute {
	// Return a copy to prevent modification
	attrs := make([]TokenAttribute, len(t.attributes))
	copy(attrs, t.attributes)
	return attrs
}

// TokenAttribute is a holder for a token attribute.
type TokenAttribute struct {
	attClass  string
	attValues map[string]string
}

func (ta *TokenAttribute) AttClass() string {
	return ta.attClass
}

func (ta *TokenAttribute) AttValues() map[string]string {
	// Return a copy to prevent modification
	values := make(map[string]string, len(ta.attValues))
	for k, v := range ta.attValues {
		values[k] = v
	}
	return values
}

// NamedObject is a base for named objects.
type NamedObject struct {
	name string
}

func (no *NamedObject) Name() string {
	return no.name
}

// NamedTokens is a holder for a pair tokenizer/filter and token list.
type NamedTokens struct {
	NamedObject
	tokens []Token
}

func (nt *NamedTokens) Tokens() []Token {
	return nt.tokens
}

// CharfilteredText is a holder for a charfilter name and text that output by the charfilter.
type CharfilteredText struct {
	NamedObject
	text string
}

func (ct *CharfilteredText) Text() string {
	return ct.text
}

// StepByStepResult is a step-by-step analysis result holder.
type StepByStepResult struct {
	charfilteredTexts []CharfilteredText
	namedTokens       []NamedTokens
}

func (s *StepByStepResult) CharfilteredTexts() []CharfilteredText {
	return s.charfilteredTexts
}

func (s *StepByStepResult) NamedTokens() []NamedTokens {
	return s.namedTokens
}

// Analysis is a dedicated interface for Luke's Analysis tab.
type Analysis interface {
	GetAvailableCharFilters() []string
	GetAvailableTokenizers() []string
	GetAvailableTokenFilters() []string
	CreateAnalyzerFromClassName(analyzerType string) (analysis.Analyzer, error)
	BuildCustomAnalyzer(config CustomAnalyzerConfig) (analysis.Analyzer, error)
	Analyze(text string) ([]Token, error)
	CurrentAnalyzer() (analysis.Analyzer, error)
	AddExternalJars(jarFiles []string) error
	AnalyzeStepByStep(text string) (*StepByStepResult, error)
}

// CustomAnalyzerConfig is a configuration for building a custom analyzer.
type CustomAnalyzerConfig struct {
	ConfigDir        *string
	TokenizerConfig  ComponentConfig
	CharFilterConfigs []ComponentConfig
	TokenFilterConfigs []ComponentConfig
}

// ComponentConfig is a configuration for a specific analysis component.
type ComponentConfig struct {
	Name   string
	Params map[string]string
}
