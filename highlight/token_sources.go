// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TokenSources provides utility methods for obtaining TokenStreams.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.TokenSources.
type TokenSources struct{}

// NewTokenSources creates a new TokenSources.
func NewTokenSources() *TokenSources {
	return &TokenSources{}
}

// GetTokenStream returns a TokenStream for the given field from the document.
// This uses the analyzer to tokenize the stored field value.
func (ts *TokenSources) GetTokenStream(field string, fieldValue string, analyzer analysis.Analyzer) (analysis.TokenStream, error) {
	if fieldValue == "" {
		return nil, nil
	}

	// Create a TokenStream by tokenizing the value
	return analyzer.TokenStream(field, strings.NewReader(fieldValue))
}

// TokenGroup represents a group of tokens for scoring.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.TokenGroup.
type TokenGroup struct {
	// tokens contains the tokens in this group
	tokens []*TokenScore

	// totalScore is the total score of this group
	totalScore float32

	// numTokens is the number of tokens in this group
	numTokens int
}

// TokenScore represents a token and its score.
type TokenScore struct {
	// Token is the token text
	Token string

	// Score is the score for this token
	Score float32

	// StartOffset is the start offset in the original text
	StartOffset int

	// EndOffset is the end offset in the original text
	EndOffset int
}

// NewTokenGroup creates a new TokenGroup.
func NewTokenGroup() *TokenGroup {
	return &TokenGroup{
		tokens: make([]*TokenScore, 0),
	}
}

// AddToken adds a token to this group.
func (tg *TokenGroup) AddToken(token string, score float32, startOffset, endOffset int) {
	tg.tokens = append(tg.tokens, &TokenScore{
		Token:       token,
		Score:       score,
		StartOffset: startOffset,
		EndOffset:   endOffset,
	})
	tg.totalScore += score
	tg.numTokens++
}

// GetScore returns the total score of this group.
func (tg *TokenGroup) GetScore() float32 {
	return tg.totalScore
}

// GetNumTokens returns the number of tokens in this group.
func (tg *TokenGroup) GetNumTokens() int {
	return tg.numTokens
}

// GetTokens returns the tokens in this group.
func (tg *TokenGroup) GetTokens() []*TokenScore {
	return tg.tokens
}

// GetTokenScore returns the score for a specific token.
func (tg *TokenGroup) GetTokenScore(index int) float32 {
	if index >= 0 && index < len(tg.tokens) {
		return tg.tokens[index].Score
	}
	return 0
}

// Clear clears this token group.
func (tg *TokenGroup) Clear() {
	tg.tokens = tg.tokens[:0]
	tg.totalScore = 0
	tg.numTokens = 0
}

// IsDistinct returns true if the given offset is distinct from the current tokens.
func (tg *TokenGroup) IsDistinct(startOffset, endOffset int) bool {
	for _, token := range tg.tokens {
		if startOffset < token.EndOffset && endOffset > token.StartOffset {
			return false
		}
	}
	return true
}
