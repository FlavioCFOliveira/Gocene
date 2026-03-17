// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestSynonymFilter_BasicSingleWord tests basic single-word synonym expansion.
// Source: TestSynonymFilter.testSingleWordSynonyms()
// Purpose: Tests that single-word synonyms are correctly expanded.
func TestSynonymFilter_BasicSingleWord(t *testing.T) {
	// Create synonym map: "usa" -> "america", "united states"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("usa", "america", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	// Use StringToWords for multi-word output
	if err := builder.Add([]byte("usa"), StringToWords("united states"), false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("usa"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text            string
		positionIncrement int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.positionIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "usa" (posInc=1), "america" (posInc=0), "united" (posInc=0), "states" (posInc=1)
	expected := []tokenInfo{
		{text: "usa", positionIncrement: 1},
		{text: "america", positionIncrement: 0},
		{text: "united", positionIncrement: 0},
		{text: "states", positionIncrement: 1},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionIncrement != exp.positionIncrement {
			t.Errorf("Token %d: expected %+v, got %+v", i, exp, tokens[i])
		}
	}
}

// TestSynonymFilter_MultipleInputWords tests synonyms for multiple input words.
// Source: TestSynonymFilter.testMultiWordInput()
// Purpose: Tests that multi-word input synonyms are correctly matched.
func TestSynonymFilter_MultipleInputWords(t *testing.T) {
	// Create synonym map: "united states" -> "usa"
	builder := NewSynonymMapBuilder()
	if err := builder.Add([]byte("united\x00states"), []byte("usa"), false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("united states"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text            string
		positionIncrement int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.positionIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "united" (posInc=1), "states" (posInc=1), "usa" (posInc=0)
	expected := []tokenInfo{
		{text: "united", positionIncrement: 1},
		{text: "states", positionIncrement: 1},
		{text: "usa", positionIncrement: 0},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionIncrement != exp.positionIncrement {
			t.Errorf("Token %d: expected %+v, got %+v", i, exp, tokens[i])
		}
	}
}

// TestSynonymFilter_NoMatch tests that tokens without synonyms pass through.
// Source: TestSynonymFilter.testNoMatch()
// Purpose: Tests that tokens without synonyms are emitted unchanged.
func TestSynonymFilter_NoMatch(t *testing.T) {
	// Create synonym map: "usa" -> "america"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("usa", "america", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	var tokens []string
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
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestSynonymFilter_MixedMatch tests a mix of tokens with and without synonyms.
// Source: TestSynonymFilter.testMixedMatch()
// Purpose: Tests that synonyms and non-synonyms are handled correctly together.
func TestSynonymFilter_MixedMatch(t *testing.T) {
	// Create synonym map: "quick" -> "fast"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("quick", "fast", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("the quick brown fox"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text            string
		positionIncrement int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.positionIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "the" (posInc=1), "quick" (posInc=1), "fast" (posInc=0), "brown" (posInc=1), "fox" (posInc=1)
	expected := []tokenInfo{
		{text: "the", positionIncrement: 1},
		{text: "quick", positionIncrement: 1},
		{text: "fast", positionIncrement: 0},
		{text: "brown", positionIncrement: 1},
		{text: "fox", positionIncrement: 1},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionIncrement != exp.positionIncrement {
			t.Errorf("Token %d: expected %+v, got %+v", i, exp, tokens[i])
		}
	}
}

// TestSynonymFilter_EmptyInput tests empty input handling.
// Source: TestSynonymFilter.testEmptyInput()
// Purpose: Tests that empty input is handled correctly.
func TestSynonymFilter_EmptyInput(t *testing.T) {
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("test", "example", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(""))

	filter := NewSynonymFilter(tokenizer, synonymMap)
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

// TestSynonymFilter_MultipleSynonyms tests multiple synonyms for one input.
// Source: TestSynonymFilter.testMultipleSynonyms()
// Purpose: Tests that multiple synonyms are emitted for a single input.
func TestSynonymFilter_MultipleSynonyms(t *testing.T) {
	// Create synonym map: "tv" -> "television", "telly", "tv set"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("tv", "television", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	if err := builder.AddString("tv", "telly", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	if err := builder.Add([]byte("tv"), StringToWords("tv set"), false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("tv"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text            string
		positionIncrement int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.positionIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "tv" (posInc=1), "television" (posInc=0), "telly" (posInc=0), "tv" (posInc=0), "set" (posInc=1)
	expected := []tokenInfo{
		{text: "tv", positionIncrement: 1},
		{text: "television", positionIncrement: 0},
		{text: "telly", positionIncrement: 0},
		{text: "tv", positionIncrement: 0},
		{text: "set", positionIncrement: 1},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionIncrement != exp.positionIncrement {
			t.Errorf("Token %d: expected %+v, got %+v", i, exp, tokens[i])
		}
	}
}

// TestSynonymFilter_CaseInsensitive tests case-insensitive synonym matching.
// Source: TestSynonymFilter.testCaseInsensitive()
// Purpose: Tests that case-insensitive matching works correctly.
func TestSynonymFilter_CaseInsensitive(t *testing.T) {
	// Create synonym map: "USA" -> "America" (with case-insensitive storage)
	builder := NewSynonymMapBuilder()
	builder.SetIgnoreCase(true)
	if err := builder.AddString("USA", "America", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("usa USA Usa"))

	filter := NewSynonymFilterWithOptions(tokenizer, synonymMap, true)
	defer filter.Close()

	var tokens []string
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
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// All three "usa" variants should match and produce "America" synonym
	// Expected: "usa", "America", "USA", "America", "Usa", "America" (6 tokens)
	if len(tokens) != 6 {
		t.Errorf("Expected 6 tokens (3 inputs + 3 synonyms), got %d: %v", len(tokens), tokens)
	}
}

// TestSynonymFilter_EndMethod tests the End() method.
// Source: TestSynonymFilter.testEnd()
// Purpose: Tests that End() is properly propagated.
func TestSynonymFilter_EndMethod(t *testing.T) {
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("test", "example", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err = filter.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestSynonymFilter_Chaining tests chaining with other filters.
// Source: TestSynonymFilter.testChaining()
// Purpose: Tests that SynonymFilter works properly in filter chains.
func TestSynonymFilter_Chaining(t *testing.T) {
	// Create synonym map: "USA" -> "America"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("USA", "America", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	input := "USA is great"

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader(input))

	// Chain: WhitespaceTokenizer -> SynonymFilter -> LowerCaseFilter
	synonymFilter := NewSynonymFilter(tokenizer, synonymMap)
	defer synonymFilter.Close()

	lowerFilter := NewLowerCaseFilter(synonymFilter)
	defer lowerFilter.Close()

	var tokens []string
	for {
		hasToken, err := lowerFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := lowerFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Expected: "usa", "america", "is", "great"
	expected := []string{"usa", "america", "is", "great"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestSynonymFilter_Factory tests the SynonymFilterFactory.
// Source: TestSynonymFilter.testFactory()
// Purpose: Tests that the factory creates filters correctly.
func TestSynonymFilter_Factory(t *testing.T) {
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("test", "example", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	factory := NewSynonymFilterFactory(synonymMap)

	if factory.GetSynonymMap() != synonymMap {
		t.Error("Factory returned wrong SynonymMap")
	}

	if factory.IsIgnoreCase() {
		t.Error("Factory should not be case-insensitive by default")
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	filter := factory.Create(tokenizer)
	defer filter.Close()

	// Cast to *SynonymFilter to access GetAttributeSource
	synFilter, ok := filter.(*SynonymFilter)
	if !ok {
		t.Fatal("Factory did not return a *SynonymFilter")
	}

	var tokens []string
	for {
		hasToken, err := synFilter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}
		if attr := synFilter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	// Expected: "test", "example"
	expected := []string{"test", "example"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestSynonymFilter_Getters tests the getter methods.
// Source: TestSynonymFilter.testGetters()
// Purpose: Tests that getter methods return correct values.
func TestSynonymFilter_Getters(t *testing.T) {
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("test", "example", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	filter := NewSynonymFilterWithOptions(tokenizer, synonymMap, true)
	defer filter.Close()

	if filter.GetSynonymMap() != synonymMap {
		t.Error("GetSynonymMap() returned wrong value")
	}

	if !filter.IsIgnoreCase() {
		t.Error("IsIgnoreCase() should return true")
	}
}

// TestSynonymFilter_MultiWordOutput tests multi-word output synonyms.
// Source: TestSynonymFilter.testMultiWordOutput()
// Purpose: Tests that multi-word output synonyms are handled correctly.
func TestSynonymFilter_MultiWordOutput(t *testing.T) {
	// Create synonym map: "ny" -> "new york"
	builder := NewSynonymMapBuilder()
	if err := builder.Add([]byte("ny"), StringToWords("new york"), false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("ny"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text            string
		positionIncrement int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.positionIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "ny" (posInc=1), "new" (posInc=0), "york" (posInc=1)
	expected := []tokenInfo{
		{text: "ny", positionIncrement: 1},
		{text: "new", positionIncrement: 0},
		{text: "york", positionIncrement: 1},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionIncrement != exp.positionIncrement {
			t.Errorf("Token %d: expected %+v, got %+v", i, exp, tokens[i])
		}
	}
}

// TestSynonymFilter_ConsecutiveMatches tests consecutive synonym matches.
// Source: TestSynonymFilter.testConsecutiveMatches()
// Purpose: Tests that consecutive tokens with synonyms are handled correctly.
func TestSynonymFilter_ConsecutiveMatches(t *testing.T) {
	// Create synonym map: "a" -> "alpha", "b" -> "beta"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("a", "alpha", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	if err := builder.AddString("b", "beta", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("a b"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text            string
		positionIncrement int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.positionIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "a" (posInc=1), "alpha" (posInc=0), "b" (posInc=1), "beta" (posInc=0)
	expected := []tokenInfo{
		{text: "a", positionIncrement: 1},
		{text: "alpha", positionIncrement: 0},
		{text: "b", positionIncrement: 1},
		{text: "beta", positionIncrement: 0},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionIncrement != exp.positionIncrement {
			t.Errorf("Token %d: expected %+v, got %+v", i, exp, tokens[i])
		}
	}
}

// TestSynonymFilter_Offsets tests offset preservation.
// Source: TestSynonymFilter.testOffsets()
// Purpose: Tests that character offsets are preserved.
func TestSynonymFilter_Offsets(t *testing.T) {
	// Create synonym map: "test" -> "example"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("test", "example", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test word"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text        string
		startOffset int
		endOffset   int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				info.startOffset = offsetAttr.StartOffset()
				info.endOffset = offsetAttr.EndOffset()
			}
		}
		tokens = append(tokens, info)
	}

	if len(tokens) < 2 {
		t.Fatalf("Expected at least 2 tokens, got %d", len(tokens))
	}

	// First token "test" should have offsets [0, 4]
	if tokens[0].text != "test" || tokens[0].startOffset != 0 || tokens[0].endOffset != 4 {
		t.Errorf("First token: expected test [0,4], got %s [%d,%d]",
			tokens[0].text, tokens[0].startOffset, tokens[0].endOffset)
	}

	// Second token "word" should have offsets [5, 9]
	if tokens[2].text != "word" || tokens[2].startOffset != 5 || tokens[2].endOffset != 9 {
		t.Errorf("Third token: expected word [5,9], got %s [%d,%d]",
			tokens[2].text, tokens[2].startOffset, tokens[2].endOffset)
	}
}

// TestSynonymFilter_EmptySynonymMap tests behavior with empty synonym map.
// Source: TestSynonymFilter.testEmptySynonymMap()
// Purpose: Tests that an empty synonym map passes tokens through unchanged.
func TestSynonymFilter_EmptySynonymMap(t *testing.T) {
	synonymMap := NewSynonymMap()

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("hello world"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	var tokens []string
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
				tokens = append(tokens, termAttr.String())
			}
		}
	}

	expected := []string{"hello", "world"}
	if !reflect.DeepEqual(tokens, expected) {
		t.Errorf("Expected %v, got %v", expected, tokens)
	}
}

// TestSynonymFilter_LongerMatchPreferred tests that longer matches are preferred.
// Source: TestSynonymFilter.testLongerMatchPreferred()
// Purpose: Tests that the longest possible synonym match is used.
func TestSynonymFilter_LongerMatchPreferred(t *testing.T) {
	// Create synonym map with overlapping matches:
	// "new" -> "fresh"
	// "new york" -> "nyc"
	builder := NewSynonymMapBuilder()
	if err := builder.AddString("new", "fresh", false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	if err := builder.Add([]byte("new\x00york"), []byte("nyc"), false); err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}
	synonymMap, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("new york city"))

	filter := NewSynonymFilter(tokenizer, synonymMap)
	defer filter.Close()

	type tokenInfo struct {
		text            string
		positionIncrement int
	}

	var tokens []tokenInfo
	for {
		hasToken, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		var info tokenInfo
		if attr := filter.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				info.text = termAttr.String()
			}
		}
		if attr := filter.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				info.positionIncrement = posAttr.GetPositionIncrement()
			}
		}
		tokens = append(tokens, info)
	}

	// Expected: "new" (posInc=1), "york" (posInc=1), "nyc" (posInc=0), "city" (posInc=1)
	// The longer match "new york" should be preferred over just "new"
	expected := []tokenInfo{
		{text: "new", positionIncrement: 1},
		{text: "york", positionIncrement: 1},
		{text: "nyc", positionIncrement: 0},
		{text: "city", positionIncrement: 1},
	}

	if len(tokens) != len(expected) {
		t.Errorf("Expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
		return
	}

	for i, exp := range expected {
		if tokens[i].text != exp.text || tokens[i].positionIncrement != exp.positionIncrement {
			t.Errorf("Token %d: expected %+v, got %+v", i, exp, tokens[i])
		}
	}
}
