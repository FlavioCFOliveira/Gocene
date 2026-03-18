// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// AnalyzerIntegration provides integration between the query parser and analysis components.
// This enables proper tokenization and analysis of query text.
type AnalyzerIntegration struct {
	analyzer           analysis.Analyzer
	defaultField       string
	positionIncrements bool
}

// NewAnalyzerIntegration creates a new AnalyzerIntegration.
func NewAnalyzerIntegration(analyzer analysis.Analyzer) *AnalyzerIntegration {
	return &AnalyzerIntegration{
		analyzer:           analyzer,
		defaultField:       "default",
		positionIncrements: true,
	}
}

// NewAnalyzerIntegrationWithField creates a new AnalyzerIntegration with a default field.
func NewAnalyzerIntegrationWithField(analyzer analysis.Analyzer, defaultField string) *AnalyzerIntegration {
	ai := NewAnalyzerIntegration(analyzer)
	ai.defaultField = defaultField
	return ai
}

// GetAnalyzer returns the analyzer.
func (ai *AnalyzerIntegration) GetAnalyzer() analysis.Analyzer {
	return ai.analyzer
}

// SetAnalyzer sets the analyzer.
func (ai *AnalyzerIntegration) SetAnalyzer(analyzer analysis.Analyzer) {
	ai.analyzer = analyzer
}

// GetDefaultField returns the default field.
func (ai *AnalyzerIntegration) GetDefaultField() string {
	return ai.defaultField
}

// SetDefaultField sets the default field.
func (ai *AnalyzerIntegration) SetDefaultField(field string) {
	ai.defaultField = field
}

// GetPositionIncrements returns whether position increments are enabled.
func (ai *AnalyzerIntegration) GetPositionIncrements() bool {
	return ai.positionIncrements
}

// SetPositionIncrements sets whether position increments are enabled.
func (ai *AnalyzerIntegration) SetPositionIncrements(enabled bool) {
	ai.positionIncrements = enabled
}

// Tokenize tokenizes the given text using the analyzer.
func (ai *AnalyzerIntegration) Tokenize(field, text string) ([]TokenInfo, error) {
	if ai.analyzer == nil {
		return []TokenInfo{{Term: text, Position: 0}}, nil
	}

	if field == "" {
		field = ai.defaultField
	}

	reader := strings.NewReader(text)
	tokenStream, err := ai.analyzer.TokenStream(field, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create token stream: %w", err)
	}
	defer tokenStream.Close()

	var tokens []TokenInfo
	position := 0

	for {
		hasToken, err := tokenStream.IncrementToken()
		if err != nil {
			return nil, fmt.Errorf("incrementing token: %w", err)
		}
		if !hasToken {
			break
		}

		// Get the term attribute using the attribute source
		if baseTs, ok := tokenStream.(*analysis.BaseTokenStream); ok {
			attrSource := baseTs.GetAttributeSource()

			// Get CharTermAttribute
			if termAttr := attrSource.GetAttribute("CharTermAttribute"); termAttr != nil {
				if cta, ok := termAttr.(analysis.CharTermAttribute); ok {
					term := cta.String()
					if term == "" {
						continue
					}

					// Get position increment if available
					if posIncAttr := attrSource.GetAttribute("PositionIncrementAttribute"); posIncAttr != nil {
						if pia, ok := posIncAttr.(analysis.PositionIncrementAttribute); ok && ai.positionIncrements {
							position += pia.GetPositionIncrement()
						} else {
							position++
						}
					} else {
						position++
					}

					tokens = append(tokens, TokenInfo{
						Term:     term,
						Position: position,
					})
				}
			}
		}
	}

	return tokens, nil
}

// CreateQuery creates a query from analyzed tokens.
func (ai *AnalyzerIntegration) CreateQuery(field string, tokens []TokenInfo) search.Query {
	if len(tokens) == 0 {
		return search.NewMatchNoDocsQuery()
	}

	if len(tokens) == 1 {
		return search.NewTermQuery(index.NewTerm(field, tokens[0].Term))
	}

	// Create a phrase query for multiple tokens
	terms := make([]*index.Term, len(tokens))
	for i, token := range tokens {
		terms[i] = index.NewTerm(field, token.Term)
	}

	return search.NewPhraseQuery(field, terms...)
}

// AnalyzeText analyzes text and creates appropriate queries.
func (ai *AnalyzerIntegration) AnalyzeText(field, text string) (search.Query, error) {
	tokens, err := ai.Tokenize(field, text)
	if err != nil {
		return nil, fmt.Errorf("tokenizing text: %w", err)
	}

	return ai.CreateQuery(field, tokens), nil
}

// GetAnalyzedTerms returns the analyzed terms for a given text.
func (ai *AnalyzerIntegration) GetAnalyzedTerms(field, text string) ([]string, error) {
	tokens, err := ai.Tokenize(field, text)
	if err != nil {
		return nil, err
	}

	terms := make([]string, len(tokens))
	for i, token := range tokens {
		terms[i] = token.Term
	}

	return terms, nil
}

// IsMultiTerm checks if the analyzed text produces multiple terms.
func (ai *AnalyzerIntegration) IsMultiTerm(field, text string) bool {
	terms, err := ai.GetAnalyzedTerms(field, text)
	if err != nil {
		return false
	}
	return len(terms) > 1
}

// TokenInfo holds information about a token.
type TokenInfo struct {
	Term        string
	Position    int
	StartOffset int
	EndOffset   int
	Type        string
}

// QueryAnalyzer provides high-level query analysis functionality.
type QueryAnalyzer struct {
	integration *AnalyzerIntegration
}

// NewQueryAnalyzer creates a new QueryAnalyzer.
func NewQueryAnalyzer(analyzer analysis.Analyzer) *QueryAnalyzer {
	return &QueryAnalyzer{
		integration: NewAnalyzerIntegration(analyzer),
	}
}

// AnalyzeQueryString analyzes a query string and returns the analyzed terms.
func (qa *QueryAnalyzer) AnalyzeQueryString(field, queryString string) (*AnalyzedQuery, error) {
	if field == "" {
		field = qa.integration.GetDefaultField()
	}

	// Split the query string into parts (simple approach)
	parts := strings.Fields(queryString)

	analyzed := &AnalyzedQuery{
		Original: queryString,
		Field:    field,
		Parts:    make([]AnalyzedPart, 0, len(parts)),
	}

	for _, part := range parts {
		// Skip boolean operators
		upperPart := strings.ToUpper(part)
		if upperPart == "AND" || upperPart == "OR" || upperPart == "NOT" {
			analyzed.Parts = append(analyzed.Parts, AnalyzedPart{
				Original: part,
				Type:     PartTypeOperator,
			})
			continue
		}

		// Analyze the part
		terms, err := qa.integration.GetAnalyzedTerms(field, part)
		if err != nil {
			return nil, fmt.Errorf("analyzing part %q: %w", part, err)
		}

		analyzed.Parts = append(analyzed.Parts, AnalyzedPart{
			Original: part,
			Terms:    terms,
			Type:     PartTypeTerm,
		})
	}

	return analyzed, nil
}

// AnalyzedQuery represents an analyzed query.
type AnalyzedQuery struct {
	Original string
	Field    string
	Parts    []AnalyzedPart
}

// AnalyzedPart represents a part of an analyzed query.
type AnalyzedPart struct {
	Original string
	Terms    []string
	Type     PartType
}

// PartType represents the type of a query part.
type PartType int

const (
	// PartTypeTerm is a term part.
	PartTypeTerm PartType = iota
	// PartTypeOperator is a boolean operator.
	PartTypeOperator
	// PartTypePhrase is a phrase.
	PartTypePhrase
	// PartTypeRange is a range.
	PartTypeRange
)

// String returns the string representation of the part type.
func (pt PartType) String() string {
	switch pt {
	case PartTypeTerm:
		return "TERM"
	case PartTypeOperator:
		return "OPERATOR"
	case PartTypePhrase:
		return "PHRASE"
	case PartTypeRange:
		return "RANGE"
	default:
		return "UNKNOWN"
	}
}
