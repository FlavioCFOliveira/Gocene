// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenManagerAdvanced provides advanced token management capabilities.
// This extends the basic QueryParserTokenManager with additional features.
type TokenManagerAdvanced struct {
	*QueryParserTokenManager
	charStream CharStream
	tokens     []Token
	position   int
	lookahead  int
}

// NewTokenManagerAdvanced creates a new TokenManagerAdvanced.
func NewTokenManagerAdvanced(input string) *TokenManagerAdvanced {
	return &TokenManagerAdvanced{
		QueryParserTokenManager: NewQueryParserTokenManager(input),
		charStream:              NewFastCharStream(input),
		tokens:                  make([]Token, 0),
		position:                0,
		lookahead:               1,
	}
}

// NewTokenManagerAdvancedWithStream creates a new TokenManagerAdvanced with a CharStream.
func NewTokenManagerAdvancedWithStream(stream CharStream) *TokenManagerAdvanced {
	return &TokenManagerAdvanced{
		QueryParserTokenManager: NewQueryParserTokenManager(stream.GetImage()),
		charStream:              stream,
		tokens:                  make([]Token, 0),
		position:                0,
		lookahead:               1,
	}
}

// GetNextToken returns the next token, using lookahead buffer if available.
func (tma *TokenManagerAdvanced) GetNextToken() Token {
	// Check if we have tokens in the buffer
	if tma.position < len(tma.tokens) {
		token := tma.tokens[tma.position]
		tma.position++
		return token
	}

	// Get next token from base manager
	token := tma.QueryParserTokenManager.NextToken()
	tma.tokens = append(tma.tokens, token)
	tma.position++
	return token
}

// GetToken returns a token at a specific position relative to current.
func (tma *TokenManagerAdvanced) GetToken(offset int) Token {
	pos := tma.position + offset - 1
	if pos < 0 || pos >= len(tma.tokens) {
		return Token{Type: TokenTypeEOF, Value: "", Pos: -1}
	}
	return tma.tokens[pos]
}

// Backup backs up the token stream by the specified number of tokens.
func (tma *TokenManagerAdvanced) Backup(amount int) {
	tma.position -= amount
	if tma.position < 0 {
		tma.position = 0
	}
}

// GetCurrentPosition returns the current token position.
func (tma *TokenManagerAdvanced) GetCurrentPosition() int {
	return tma.position
}

// SetCurrentPosition sets the current token position.
func (tma *TokenManagerAdvanced) SetCurrentPosition(pos int) {
	if pos >= 0 && pos <= len(tma.tokens) {
		tma.position = pos
	}
}

// ClearBuffer clears the token buffer.
func (tma *TokenManagerAdvanced) ClearBuffer() {
	tma.tokens = make([]Token, 0)
	tma.position = 0
}

// GetTokenCount returns the number of tokens in the buffer.
func (tma *TokenManagerAdvanced) GetTokenCount() int {
	return len(tma.tokens)
}

// GetLookahead returns the current lookahead setting.
func (tma *TokenManagerAdvanced) GetLookahead() int {
	return tma.lookahead
}

// SetLookahead sets the lookahead value.
func (tma *TokenManagerAdvanced) SetLookahead(lookahead int) {
	if lookahead > 0 {
		tma.lookahead = lookahead
	}
}

// TokenizeAll tokenizes the entire input and returns all tokens.
func (tma *TokenManagerAdvanced) TokenizeAll() []Token {
	var allTokens []Token
	for {
		token := tma.GetNextToken()
		allTokens = append(allTokens, token)
		if token.Type == TokenTypeEOF {
			break
		}
	}
	return allTokens
}

// PeekToken peeks at the next token without consuming it.
func (tma *TokenManagerAdvanced) PeekToken() Token {
	return tma.GetToken(1)
}

// PeekTokenAt peeks at a token at a specific offset.
func (tma *TokenManagerAdvanced) PeekTokenAt(offset int) Token {
	return tma.GetToken(offset)
}

// Match checks if the next token matches the expected type.
func (tma *TokenManagerAdvanced) Match(expected TokenType) bool {
	token := tma.PeekToken()
	return token.Type == expected
}

// Consume consumes the next token if it matches the expected type.
func (tma *TokenManagerAdvanced) Consume(expected TokenType) (Token, error) {
	token := tma.GetNextToken()
	if token.Type != expected {
		return Token{}, fmt.Errorf("expected token %v but got %v", expected, token.Type)
	}
	return token, nil
}

// SkipUntil skips tokens until the specified token type is found.
func (tma *TokenManagerAdvanced) SkipUntil(tokenType TokenType) Token {
	for {
		token := tma.GetNextToken()
		if token.Type == tokenType || token.Type == TokenTypeEOF {
			return token
		}
	}
}

// GetTokenString returns a string representation of tokens in a range.
func (tma *TokenManagerAdvanced) GetTokenString(start, end int) string {
	if start < 0 || end > len(tma.tokens) || start >= end {
		return ""
	}

	var parts []string
	for i := start; i < end && i < len(tma.tokens); i++ {
		parts = append(parts, tma.tokens[i].Value)
	}
	return strings.Join(parts, " ")
}

// TokenQueue provides a queue-based token management.
type TokenQueue struct {
	tokens []Token
	head   int
	tail   int
	size   int
}

// NewTokenQueue creates a new TokenQueue with the specified capacity.
func NewTokenQueue(capacity int) *TokenQueue {
	return &TokenQueue{
		tokens: make([]Token, capacity),
		head:   0,
		tail:   0,
		size:   0,
	}
}

// Enqueue adds a token to the queue.
func (tq *TokenQueue) Enqueue(token Token) error {
	if tq.size >= len(tq.tokens) {
		return fmt.Errorf("token queue is full")
	}
	tq.tokens[tq.tail] = token
	tq.tail = (tq.tail + 1) % len(tq.tokens)
	tq.size++
	return nil
}

// Dequeue removes and returns the next token from the queue.
func (tq *TokenQueue) Dequeue() (Token, error) {
	if tq.size == 0 {
		return Token{}, fmt.Errorf("token queue is empty")
	}
	token := tq.tokens[tq.head]
	tq.head = (tq.head + 1) % len(tq.tokens)
	tq.size--
	return token, nil
}

// Peek returns the next token without removing it.
func (tq *TokenQueue) Peek() (Token, error) {
	if tq.size == 0 {
		return Token{}, fmt.Errorf("token queue is empty")
	}
	return tq.tokens[tq.head], nil
}

// IsEmpty returns whether the queue is empty.
func (tq *TokenQueue) IsEmpty() bool {
	return tq.size == 0
}

// IsFull returns whether the queue is full.
func (tq *TokenQueue) IsFull() bool {
	return tq.size >= len(tq.tokens)
}

// Size returns the current size of the queue.
func (tq *TokenQueue) Size() int {
	return tq.size
}

// Capacity returns the capacity of the queue.
func (tq *TokenQueue) Capacity() int {
	return len(tq.tokens)
}

// Clear clears the queue.
func (tq *TokenQueue) Clear() {
	tq.head = 0
	tq.tail = 0
	tq.size = 0
}

// TokenFilter provides token filtering capabilities.
type TokenFilter struct {
	filterFunc func(Token) bool
}

// NewTokenFilter creates a new TokenFilter with the specified filter function.
func NewTokenFilter(filterFunc func(Token) bool) *TokenFilter {
	return &TokenFilter{filterFunc: filterFunc}
}

// Filter filters a slice of tokens.
func (tf *TokenFilter) Filter(tokens []Token) []Token {
	var filtered []Token
	for _, token := range tokens {
		if tf.filterFunc(token) {
			filtered = append(filtered, token)
		}
	}
	return filtered
}

// Common token filters

// NewEOFTokenFilter creates a filter that removes EOF tokens.
func NewEOFTokenFilter() *TokenFilter {
	return NewTokenFilter(func(t Token) bool {
		return t.Type != TokenTypeEOF
	})
}

// NewWhitespaceTokenFilter creates a filter that removes whitespace tokens.
func NewWhitespaceTokenFilter() *TokenFilter {
	return NewTokenFilter(func(t Token) bool {
		return !unicode.IsSpace(rune(t.Value[0])) || len(t.Value) > 1
	})
}

// NewOperatorTokenFilter creates a filter that keeps only operator tokens.
func NewOperatorTokenFilter() *TokenFilter {
	return NewTokenFilter(func(t Token) bool {
		return t.Type == TokenTypeAND || t.Type == TokenTypeOR || t.Type == TokenTypeNOT
	})
}

// TokenMatcher provides token matching capabilities.
type TokenMatcher struct {
	matchFunc func(Token, Token) bool
}

// NewTokenMatcher creates a new TokenMatcher with the specified match function.
func NewTokenMatcher(matchFunc func(Token, Token) bool) *TokenMatcher {
	return &TokenMatcher{matchFunc: matchFunc}
}

// Match checks if two tokens match.
func (tm *TokenMatcher) Match(t1, t2 Token) bool {
	return tm.matchFunc(t1, t2)
}

// MatchSequence checks if a sequence of tokens matches.
func (tm *TokenMatcher) MatchSequence(tokens1, tokens2 []Token) bool {
	if len(tokens1) != len(tokens2) {
		return false
	}
	for i := range tokens1 {
		if !tm.matchFunc(tokens1[i], tokens2[i]) {
			return false
		}
	}
	return true
}

// Common token matchers

// NewExactTokenMatcher creates a matcher that matches tokens exactly.
func NewExactTokenMatcher() *TokenMatcher {
	return NewTokenMatcher(func(t1, t2 Token) bool {
		return t1.Type == t2.Type && t1.Value == t2.Value
	})
}

// NewTypeTokenMatcher creates a matcher that matches only by type.
func NewTypeTokenMatcher() *TokenMatcher {
	return NewTokenMatcher(func(t1, t2 Token) bool {
		return t1.Type == t2.Type
	})
}

// NewValueTokenMatcher creates a matcher that matches only by value.
func NewValueTokenMatcher() *TokenMatcher {
	return NewTokenMatcher(func(t1, t2 Token) bool {
		return t1.Value == t2.Value
	})
}
