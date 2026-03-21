// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
)

// AnalyzerUtils provides utility methods for analysis operations.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.AnalyzerUtil.
//
// AnalyzerUtils provides helper methods for common analysis tasks such as
// tokenizing text, creating token streams, and managing token positions.
type AnalyzerUtils struct{}

// NewAnalyzerUtils creates a new AnalyzerUtils instance.
func NewAnalyzerUtils() *AnalyzerUtils {
	return &AnalyzerUtils{}
}

// Tokenize tokenizes the given text using the provided Tokenizer.
// Returns a slice of token strings.
func Tokenize(tokenizer Tokenizer, text string) ([]string, error) {
	return tokenizeInternal(tokenizer, text)
}

// tokenizeInternal performs the actual tokenization work.
// Split from Tokenize to enable inlining of the public function.
func tokenizeInternal(tokenizer Tokenizer, text string) ([]string, error) {
	tokenizer.SetReader(strings.NewReader(text))

	var tokens []string
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			tokenizer.Close()
			return nil, err
		}
		if !hasToken {
			break
		}

		// Try to get term from the tokenizer if it's a BaseTokenStream
		if baseTs, ok := tokenizer.(interface{ GetAttribute(string) AttributeImpl }); ok {
			if attr := baseTs.GetAttribute("CharTermAttribute"); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					tokens = append(tokens, termAttr.String())
				}
			}
		}
	}

	// Explicit close instead of defer for better inlining
	err := tokenizer.Close()
	return tokens, err
}

// TokenizeWithAnalyzer tokenizes text using the given Analyzer.
// Returns a slice of token strings.
func TokenizeWithAnalyzer(analyzer Analyzer, fieldName, text string) ([]string, error) {
	return tokenizeWithAnalyzerInternal(analyzer, fieldName, text)
}

// tokenizeWithAnalyzerInternal performs the actual tokenization work.
// Split from TokenizeWithAnalyzer to enable inlining of the public function.
func tokenizeWithAnalyzerInternal(analyzer Analyzer, fieldName, text string) ([]string, error) {
	tokenStream, err := analyzer.TokenStream(fieldName, strings.NewReader(text))
	if err != nil {
		return nil, err
	}

	var tokens []string
	for {
		hasToken, err := tokenStream.IncrementToken()
		if err != nil {
			tokenStream.Close()
			return nil, err
		}
		if !hasToken {
			break
		}

		// Try to get term from the token stream if it's a BaseTokenStream
		if baseTs, ok := tokenStream.(interface{ GetAttribute(string) AttributeImpl }); ok {
			if attr := baseTs.GetAttribute("CharTermAttribute"); attr != nil {
				if termAttr, ok := attr.(CharTermAttribute); ok {
					tokens = append(tokens, termAttr.String())
				}
			}
		}
	}

	// Explicit close instead of defer for better inlining
	err = tokenStream.Close()
	return tokens, err
}

// GetTokenPositions returns the positions of tokens in the stream.
func GetTokenPositions(tokenStream TokenStream) ([]int, error) {
	var positions []int
	position := 0

	for {
		hasToken, err := tokenStream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !hasToken {
			break
		}

		// Try to get position increment from the token stream
		if baseTs, ok := tokenStream.(interface{ GetAttribute(string) AttributeImpl }); ok {
			if attr := baseTs.GetAttribute("PositionIncrementAttribute"); attr != nil {
				if posAttr, ok := attr.(PositionIncrementAttribute); ok {
					position += posAttr.GetPositionIncrement()
					positions = append(positions, position)
					continue
				}
			}
		}
		position++
		positions = append(positions, position)
	}

	return positions, nil
}

// GetTokenOffsets returns the start and end offsets of tokens.
func GetTokenOffsets(tokenStream TokenStream) ([][2]int, error) {
	var offsets [][2]int

	for {
		hasToken, err := tokenStream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !hasToken {
			break
		}

		// Try to get offsets from the token stream
		if baseTs, ok := tokenStream.(interface{ GetAttribute(string) AttributeImpl }); ok {
			if attr := baseTs.GetAttribute("OffsetAttribute"); attr != nil {
				if offsetAttr, ok := attr.(OffsetAttribute); ok {
					offsets = append(offsets, [2]int{offsetAttr.StartOffset(), offsetAttr.EndOffset()})
				}
			}
		}
	}

	return offsets, nil
}

// CreateTokenStream creates a TokenStream from text using the given components.
func CreateTokenStream(tokenizer Tokenizer, filters []TokenFilter, text string) (TokenStream, error) {
	tokenizer.SetReader(strings.NewReader(text))

	// Build the pipeline
	var stream TokenStream = tokenizer
	for _, filter := range filters {
		// Re-wrap the filter with the current stream
		if baseFilter, ok := filter.(*BaseTokenFilter); ok {
			baseFilter.SetInput(stream)
		}
		stream = filter
	}

	return stream, nil
}

// SetInput sets the input for a TokenStream (helper for filter chaining).
func SetInput(filter TokenFilter, input TokenStream) {
	if baseFilter, ok := filter.(*BaseTokenFilter); ok {
		baseFilter.input = input
	}
}

// ClearAttributes clears all attributes in the given AttributeSource.
func ClearAttributes(source *AttributeSource) {
	source.ClearAttributes()
}

// HasAttribute checks if the given attribute type exists in the source.
func HasAttribute(source *AttributeSource, attrType string) bool {
	return source.GetAttribute(attrType) != nil
}

// IsEmpty checks if a token stream has no tokens.
func IsEmpty(tokenStream TokenStream) (bool, error) {
	hasToken, err := tokenStream.IncrementToken()
	if err != nil {
		return false, err
	}
	return !hasToken, nil
}

// CountTokens counts the number of tokens in the stream.
func CountTokens(tokenStream TokenStream) (int, error) {
	count := 0
	for {
		hasToken, err := tokenStream.IncrementToken()
		if err != nil {
			return 0, err
		}
		if !hasToken {
			break
		}
		count++
	}
	return count, nil
}

// Ensure BaseTokenFilter has SetInput method
func (f *BaseTokenFilter) SetInput(input TokenStream) {
	f.input = input
	// Update AttributeSource
	if hasAttrSrc, ok := input.(interface{ GetAttributeSource() *AttributeSource }); ok {
		f.attributes = hasAttrSrc.GetAttributeSource()
	}
}
