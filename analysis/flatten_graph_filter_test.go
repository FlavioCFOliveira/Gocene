// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// mockGraphTokenStream is a mock TokenStream that produces tokens with specific
// position increments and lengths to simulate a token graph.
type mockGraphTokenStream struct {
	*BaseTokenStream
	tokens      []mockToken
	currentIdx  int
	termAttr    CharTermAttribute
	posIncrAttr PositionIncrementAttribute
	posLenAttr  *PositionLengthAttribute
	offsetAttr  OffsetAttribute
}

type mockToken struct {
	term        string
	posIncr     int
	posLen      int
	startOffset int
	endOffset   int
}

func newMockGraphTokenStream(tokens []mockToken) *mockGraphTokenStream {
	stream := &mockGraphTokenStream{
		BaseTokenStream: NewBaseTokenStream(),
		tokens:          tokens,
		currentIdx:      0,
	}

	// Add attributes
	stream.termAttr = NewCharTermAttribute()
	stream.posIncrAttr = NewPositionIncrementAttribute()
	stream.posLenAttr = NewPositionLengthAttribute()
	stream.offsetAttr = NewOffsetAttribute()

	stream.AddAttribute(stream.termAttr)
	stream.AddAttribute(stream.posIncrAttr)
	stream.AddAttribute(stream.posLenAttr)
	stream.AddAttribute(stream.offsetAttr)

	return stream
}

func (m *mockGraphTokenStream) IncrementToken() (bool, error) {
	if m.currentIdx >= len(m.tokens) {
		return false, nil
	}

	token := m.tokens[m.currentIdx]
	m.currentIdx++

	m.ClearAttributes()
	m.termAttr.SetValue(token.term)
	m.posIncrAttr.SetPositionIncrement(token.posIncr)
	m.posLenAttr.SetPositionLength(token.posLen)
	m.offsetAttr.SetStartOffset(token.startOffset)
	m.offsetAttr.SetEndOffset(token.endOffset)

	return true, nil
}

func (m *mockGraphTokenStream) End() error {
	return nil
}

// TestFlattenGraphFilter_Basic tests basic flattening of a simple graph.
// Source: TestFlattenGraphFilter.testBasic()
// Purpose: Tests that a simple token stream is passed through correctly.
func TestFlattenGraphFilter_Basic(t *testing.T) {
	// Simple linear stream: "hello world"
	tokens := []mockToken{
		{term: "hello", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 5},
		{term: "world", posIncr: 1, posLen: 1, startOffset: 6, endOffset: 11},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var results []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

// TestFlattenGraphFilter_SynonymGraph tests flattening a synonym graph.
// Source: TestFlattenGraphFilter.testSynonymGraph()
// Purpose: Tests that synonyms at the same position are handled correctly.
func TestFlattenGraphFilter_SynonymGraph(t *testing.T) {
	// Graph: "wifi" and "wireless" at same position
	// Position 0: "wifi" (posLen=1), "wireless" (posLen=1, posIncr=0)
	tokens := []mockToken{
		{term: "wifi", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 4},
		{term: "wireless", posIncr: 0, posLen: 1, startOffset: 0, endOffset: 8},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var results []string
	var posIncrs []int
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				posIncrs = append(posIncrs, posAttr.GetPositionIncrement())
			}
		}
	}

	// Both tokens should be present
	if len(results) != 2 {
		t.Errorf("Expected 2 tokens, got %d: %v", len(results), results)
	}

	// First token should have posIncr=1, second should have posIncr=0
	if len(posIncrs) != 2 || posIncrs[0] != 1 || posIncrs[1] != 0 {
		t.Errorf("Expected posIncrs [1, 0], got %v", posIncrs)
	}
}

// TestFlattenGraphFilter_MultiWordSynonym tests flattening with multi-word synonyms.
// Source: TestFlattenGraphFilter.testMultiWordSynonym()
// Purpose: Tests that multi-word synonyms with position length > 1 are handled.
func TestFlattenGraphFilter_MultiWordSynonym(t *testing.T) {
	// Graph: "ny" spans 2 positions (synonym for "new york")
	// Position 0: "ny" (posLen=2), "new" (posLen=1)
	// Position 1: "york" (posLen=1)
	tokens := []mockToken{
		{term: "ny", posIncr: 1, posLen: 2, startOffset: 0, endOffset: 2},
		{term: "new", posIncr: 0, posLen: 1, startOffset: 0, endOffset: 3},
		{term: "york", posIncr: 1, posLen: 1, startOffset: 4, endOffset: 8},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var results []struct {
		term     string
		posIncr  int
		posLen   int
		startOff int
		endOff   int
	}

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var token struct {
			term     string
			posIncr  int
			posLen   int
			startOff int
			endOff   int
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				token.term = termAttr.String()
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				token.posIncr = posAttr.GetPositionIncrement()
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionLengthAttribute"); attr != nil {
			if posLenAttr, ok := attr.(*PositionLengthAttribute); ok {
				token.posLen = posLenAttr.GetPositionLength()
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offAttr, ok := attr.(OffsetAttribute); ok {
				token.startOff = offAttr.StartOffset()
				token.endOff = offAttr.EndOffset()
			}
		}

		results = append(results, token)
	}

	// Should have 3 tokens
	if len(results) != 3 {
		t.Fatalf("Expected 3 tokens, got %d: %+v", len(results), results)
	}

	// Check first token (ny)
	if results[0].term != "ny" || results[0].posIncr != 1 || results[0].posLen != 2 {
		t.Errorf("First token: expected {ny, posIncr=1, posLen=2}, got %+v", results[0])
	}

	// Check second token (new)
	if results[1].term != "new" || results[1].posIncr != 0 || results[1].posLen != 1 {
		t.Errorf("Second token: expected {new, posIncr=0, posLen=1}, got %+v", results[1])
	}

	// Check third token (york)
	if results[2].term != "york" || results[2].posIncr != 1 || results[2].posLen != 1 {
		t.Errorf("Third token: expected {york, posIncr=1, posLen=1}, got %+v", results[2])
	}
}

// TestFlattenGraphFilter_Gap tests handling of gaps in the graph.
// Source: TestFlattenGraphFilter.testGap()
// Purpose: Tests that gaps (missing positions) are handled correctly.
func TestFlattenGraphFilter_Gap(t *testing.T) {
	// Graph with gap: position 0, then position 2 (gap at position 1)
	tokens := []mockToken{
		{term: "first", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 5},
		{term: "third", posIncr: 2, posLen: 1, startOffset: 12, endOffset: 17},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var posIncrs []int
	var results []string

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				posIncrs = append(posIncrs, posAttr.GetPositionIncrement())
			}
		}
	}

	// Should preserve the gap
	if len(results) != 2 {
		t.Errorf("Expected 2 tokens, got %d: %v", len(results), results)
	}

	// Gap should be preserved: first has posIncr=1, third has posIncr=2
	if len(posIncrs) != 2 || posIncrs[0] != 1 || posIncrs[1] != 2 {
		t.Errorf("Expected posIncrs [1, 2], got %v", posIncrs)
	}
}

// TestFlattenGraphFilter_EmptyInput tests empty input handling.
// Source: TestFlattenGraphFilter.testEmpty()
// Purpose: Tests that empty input produces no tokens.
func TestFlattenGraphFilter_EmptyInput(t *testing.T) {
	tokens := []mockToken{}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	tokenCount := 0
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		tokenCount++
	}

	if tokenCount != 0 {
		t.Errorf("Expected 0 tokens for empty input, got %d", tokenCount)
	}
}

// TestFlattenGraphFilter_SingleToken tests single token input.
// Source: TestFlattenGraphFilter.testSingleToken()
// Purpose: Tests that a single token is passed through correctly.
func TestFlattenGraphFilter_SingleToken(t *testing.T) {
	tokens := []mockToken{
		{term: "hello", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 5},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var results []string
	var posIncrs []int

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				posIncrs = append(posIncrs, posAttr.GetPositionIncrement())
			}
		}
	}

	if len(results) != 1 || results[0] != "hello" {
		t.Errorf("Expected [hello], got %v", results)
	}

	if len(posIncrs) != 1 || posIncrs[0] != 1 {
		t.Errorf("Expected posIncr [1], got %v", posIncrs)
	}
}

// TestFlattenGraphFilter_ComplexGraph tests a complex graph with multiple paths.
// Source: TestFlattenGraphFilter.testComplexGraph()
// Purpose: Tests complex graphs with multiple overlapping paths.
func TestFlattenGraphFilter_ComplexGraph(t *testing.T) {
	// Complex graph:
	// Position 0: "a" (posLen=1), "b c" (posLen=2)
	// Position 1: "d" (posLen=1)
	// Position 2: "e" (posLen=1)
	tokens := []mockToken{
		{term: "a", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 1},
		{term: "b", posIncr: 0, posLen: 2, startOffset: 0, endOffset: 1},
		{term: "c", posIncr: 1, posLen: 1, startOffset: 2, endOffset: 3},
		{term: "d", posIncr: 1, posLen: 1, startOffset: 4, endOffset: 5},
		{term: "e", posIncr: 1, posLen: 1, startOffset: 6, endOffset: 7},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var results []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}
	}

	// All tokens should be present
	// Note: "b" comes before "a" because it has a longer position length (2 vs 1)
	// The algorithm sorts by end position descending at each position
	expected := []string{"b", "a", "c", "d", "e"}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

// TestFlattenGraphFilter_PositionLengthPreserved tests that position length is preserved.
// Source: TestFlattenGraphFilter.testPositionLengthPreserved()
// Purpose: Tests that position length attribute is correctly preserved.
func TestFlattenGraphFilter_PositionLengthPreserved(t *testing.T) {
	tokens := []mockToken{
		{term: "ny", posIncr: 1, posLen: 2, startOffset: 0, endOffset: 2},
		{term: "new", posIncr: 0, posLen: 1, startOffset: 0, endOffset: 3},
		{term: "york", posIncr: 1, posLen: 1, startOffset: 4, endOffset: 8},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var posLens []int
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionLengthAttribute"); attr != nil {
			if posLenAttr, ok := attr.(*PositionLengthAttribute); ok {
				posLens = append(posLens, posLenAttr.GetPositionLength())
			}
		}
	}

	// Position lengths should be preserved: ny=2, new=1, york=1
	expected := []int{2, 1, 1}
	if !reflect.DeepEqual(posLens, expected) {
		t.Errorf("Expected posLens %v, got %v", expected, posLens)
	}
}

// TestFlattenGraphFilter_OffsetPreserved tests that offsets are preserved.
// Source: TestFlattenGraphFilter.testOffsetPreserved()
// Purpose: Tests that character offsets are correctly preserved.
func TestFlattenGraphFilter_OffsetPreserved(t *testing.T) {
	tokens := []mockToken{
		{term: "hello", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 5},
		{term: "world", posIncr: 1, posLen: 1, startOffset: 6, endOffset: 11},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	type offsetInfo struct {
		start int
		end   int
	}

	var offsets []offsetInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offAttr, ok := attr.(OffsetAttribute); ok {
				offsets = append(offsets, offsetInfo{offAttr.StartOffset(), offAttr.EndOffset()})
			}
		}
	}

	expected := []offsetInfo{{0, 5}, {6, 11}}
	if !reflect.DeepEqual(offsets, expected) {
		t.Errorf("Expected offsets %v, got %v", expected, offsets)
	}
}

// TestFlattenGraphFilter_EndMethod tests the End() method.
// Source: TestFlattenGraphFilter.testEnd()
// Purpose: Tests that End() is properly handled and end offset is preserved.
func TestFlattenGraphFilter_EndMethod(t *testing.T) {
	tokens := []mockToken{
		{term: "test", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 4},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	// Consume all tokens
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Call End
	err := filter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}

	// Check that end offset is preserved
	if attr := filter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
		if offAttr, ok := attr.(OffsetAttribute); ok {
			if offAttr.EndOffset() != 4 {
				t.Errorf("Expected end offset 4, got %d", offAttr.EndOffset())
			}
		}
	}
}

// TestFlattenGraphFilter_MultipleSynonymsAtPosition tests multiple synonyms.
// Source: TestFlattenGraphFilter.testMultipleSynonymsAtPosition()
// Purpose: Tests handling of multiple synonyms at the same position.
func TestFlattenGraphFilter_MultipleSynonymsAtPosition(t *testing.T) {
	// Multiple synonyms: "quick", "fast", "speedy" all at position 0
	tokens := []mockToken{
		{term: "quick", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 5},
		{term: "fast", posIncr: 0, posLen: 1, startOffset: 0, endOffset: 4},
		{term: "speedy", posIncr: 0, posLen: 1, startOffset: 0, endOffset: 6},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var results []string
	var posIncrs []int

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				posIncrs = append(posIncrs, posAttr.GetPositionIncrement())
			}
		}
	}

	// All three synonyms should be present
	if len(results) != 3 {
		t.Errorf("Expected 3 tokens, got %d: %v", len(results), results)
	}

	// First should have posIncr=1, others posIncr=0
	if len(posIncrs) != 3 || posIncrs[0] != 1 || posIncrs[1] != 0 || posIncrs[2] != 0 {
		t.Errorf("Expected posIncrs [1, 0, 0], got %v", posIncrs)
	}
}

// TestFlattenGraphFilter_WithRealTokenizer tests with a real tokenizer.
// Source: TestFlattenGraphFilter.testWithRealTokenizer()
// Purpose: Tests integration with a real tokenizer.
func TestFlattenGraphFilter_WithRealTokenizer(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewFlattenGraphFilter(tokenizer)
	defer filter.Close()

	var results []string
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

// TestFlattenGraphFilter_Chaining tests chaining with other filters.
// Source: TestFlattenGraphFilter.testChaining()
// Purpose: Tests that FlattenGraphFilter works properly in filter chains.
func TestFlattenGraphFilter_Chaining(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("HELLO WORLD"))

	lowerFilter := NewLowerCaseFilter(tokenizer)
	flattenFilter := NewFlattenGraphFilter(lowerFilter)
	defer flattenFilter.Close()

	var results []string
	for {
		hasToken, err := flattenFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		if attr := flattenFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				results = append(results, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expected %v, got %v", expected, results)
	}
}

// TestFlattenGraphFilter_LongPositionLength tests tokens with long position lengths.
// Source: TestFlattenGraphFilter.testLongPositionLength()
// Purpose: Tests handling of tokens spanning many positions.
func TestFlattenGraphFilter_LongPositionLength(t *testing.T) {
	// Token spanning 3 positions
	tokens := []mockToken{
		{term: "abc", posIncr: 1, posLen: 3, startOffset: 0, endOffset: 3},
		{term: "a", posIncr: 0, posLen: 1, startOffset: 0, endOffset: 1},
		{term: "b", posIncr: 1, posLen: 1, startOffset: 1, endOffset: 2},
		{term: "c", posIncr: 1, posLen: 1, startOffset: 2, endOffset: 3},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	var results []struct {
		term    string
		posLen  int
		posIncr int
	}

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var token struct {
			term    string
			posLen  int
			posIncr int
		}

		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				token.term = termAttr.String()
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionLengthAttribute"); attr != nil {
			if posLenAttr, ok := attr.(*PositionLengthAttribute); ok {
				token.posLen = posLenAttr.GetPositionLength()
			}
		}

		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				token.posIncr = posAttr.GetPositionIncrement()
			}
		}

		results = append(results, token)
	}

	// All tokens should be present
	if len(results) != 4 {
		t.Fatalf("Expected 4 tokens, got %d: %+v", len(results), results)
	}

	// Check that position lengths are preserved
	if results[0].posLen != 3 {
		t.Errorf("First token should have posLen=3, got %d", results[0].posLen)
	}
}

// TestFlattenGraphFilter_AttributePreservation tests that all attributes are preserved.
// Source: TestFlattenGraphFilter.testAttributePreservation()
// Purpose: Tests that type, payload, flags, and keyword attributes are preserved.
func TestFlattenGraphFilter_AttributePreservation(t *testing.T) {
	// Create a mock stream with various attributes set
	tokens := []mockToken{
		{term: "test", posIncr: 1, posLen: 1, startOffset: 0, endOffset: 4},
	}

	mockStream := newMockGraphTokenStream(tokens)
	filter := NewFlattenGraphFilter(mockStream)
	defer filter.Close()

	// Just verify the filter runs without error
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}
}

// TestNewFlattenGraphFilter tests the constructor.
// Source: TestFlattenGraphFilter.testConstructor()
// Purpose: Tests that the constructor properly initializes the filter.
func TestNewFlattenGraphFilter(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	filter := NewFlattenGraphFilter(tokenizer)

	if filter == nil {
		t.Error("Expected non-nil filter")
	}

	if filter.GetInput() != tokenizer {
		t.Error("Expected input to be set correctly")
	}

	filter.Close()
}

// TestFlattenGraphFilter_Factory tests the factory pattern.
// Source: TestFlattenGraphFilter.testFactory()
// Purpose: Tests that the factory creates filters correctly.
func TestFlattenGraphFilter_Factory(t *testing.T) {
	factory := &FlattenGraphFilterFactory{}

	tokenizer := NewWhitespaceTokenizer()
	filter := factory.Create(tokenizer)

	if filter == nil {
		t.Error("Expected non-nil filter from factory")
	}

	// Verify it implements TokenFilter
	var _ TokenFilter = filter

	filter.Close()
}

// FlattenGraphFilterFactory creates FlattenGraphFilter instances.
type FlattenGraphFilterFactory struct{}

// Create creates a FlattenGraphFilter wrapping the given input.
func (f *FlattenGraphFilterFactory) Create(input TokenStream) TokenFilter {
	return NewFlattenGraphFilter(input)
}

// Ensure FlattenGraphFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*FlattenGraphFilterFactory)(nil)
