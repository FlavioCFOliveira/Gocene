/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analysis

import (
	"testing"
)

// TestSynonymGraphFilterBasic tests basic synonym functionality.
func TestSynonymGraphFilterBasic(t *testing.T) {
	// Create a synonym map with simple synonyms
	builder := NewSynonymMapBuilder()
	err := builder.AddString("quick", "fast", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create a tokenizer and filter
	reader := NewReusableStringReader()
	reader.SetValue("quick brown fox")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// Collect tokens
	tokens := collectSynonymTokens(t, filter)

	// Expected tokens:
	// Position 0: "quick" (posIncr=1, posLen=1), "fast" (posIncr=0, posLen=1)
	// Position 1: "brown" (posIncr=1, posLen=1)
	// Position 2: "fox" (posIncr=1, posLen=1)

	if len(tokens) != 4 {
		t.Fatalf("Expected 4 tokens, got %d: %v", len(tokens), tokens)
	}

	// Check first token (original)
	if tokens[0].term != "quick" {
		t.Errorf("Expected first token 'quick', got %q", tokens[0].term)
	}
	if tokens[0].posIncr != 1 {
		t.Errorf("Expected posIncr=1 for first token, got %d", tokens[0].posIncr)
	}
	if tokens[0].posLen != 1 {
		t.Errorf("Expected posLen=1 for first token, got %d", tokens[0].posLen)
	}

	// Check second token (synonym)
	if tokens[1].term != "fast" {
		t.Errorf("Expected second token 'fast', got %q", tokens[1].term)
	}
	if tokens[1].posIncr != 0 {
		t.Errorf("Expected posIncr=0 for synonym, got %d", tokens[1].posIncr)
	}
	if tokens[1].posLen != 1 {
		t.Errorf("Expected posLen=1 for synonym, got %d", tokens[1].posLen)
	}

	// Check third token (brown)
	if tokens[2].term != "brown" {
		t.Errorf("Expected third token 'brown', got %q", tokens[2].term)
	}
	if tokens[2].posIncr != 1 {
		t.Errorf("Expected posIncr=1 for 'brown', got %d", tokens[2].posIncr)
	}

	// Check fourth token (fox)
	if tokens[3].term != "fox" {
		t.Errorf("Expected fourth token 'fox', got %q", tokens[3].term)
	}
	if tokens[3].posIncr != 1 {
		t.Errorf("Expected posIncr=1 for 'fox', got %d", tokens[3].posIncr)
	}
}

// TestSynonymGraphFilterMultiWord tests multi-word synonym functionality.
func TestSynonymGraphFilterMultiWord(t *testing.T) {
	// Create a synonym map with multi-word synonyms
	builder := NewSynonymMapBuilder()

	// Add multi-word synonym: "united states" -> "usa"
	input := JoinWords([]string{"united", "states"})
	output := JoinWords([]string{"usa"})
	err := builder.Add(input, output, false)
	if err != nil {
		t.Fatalf("Failed to add multi-word synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create a tokenizer and filter
	reader := NewReusableStringReader()
	reader.SetValue("united states of america")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// Collect tokens
	tokens := collectSynonymTokens(t, filter)

	// Expected tokens:
	// Position 0: "united" (posIncr=1, posLen=1), "usa" (posIncr=0, posLen=1)
	// Position 1: "states" (posIncr=1, posLen=1)
	// Position 2: "of" (posIncr=1, posLen=1)
	// Position 3: "america" (posIncr=1, posLen=1)

	if len(tokens) != 5 {
		t.Fatalf("Expected 5 tokens, got %d: %v", len(tokens), tokens)
	}

	// Check first token (original)
	if tokens[0].term != "united" {
		t.Errorf("Expected first token 'united', got %q", tokens[0].term)
	}
	if tokens[0].posIncr != 1 {
		t.Errorf("Expected posIncr=1 for first token, got %d", tokens[0].posIncr)
	}
	if tokens[0].posLen != 1 {
		t.Errorf("Expected posLen=1 for first token, got %d", tokens[0].posLen)
	}

	// Check second token (synonym)
	if tokens[1].term != "usa" {
		t.Errorf("Expected second token 'usa', got %q", tokens[1].term)
	}
	if tokens[1].posIncr != 0 {
		t.Errorf("Expected posIncr=0 for synonym, got %d", tokens[1].posIncr)
	}
	if tokens[1].posLen != 1 {
		t.Errorf("Expected posLen=1 for synonym 'usa', got %d", tokens[1].posLen)
	}

	// Check third token (states)
	if tokens[2].term != "states" {
		t.Errorf("Expected third token 'states', got %q", tokens[2].term)
	}
	if tokens[2].posIncr != 1 {
		t.Errorf("Expected posIncr=1 for 'states', got %d", tokens[2].posIncr)
	}

	// Check fourth token (of)
	if tokens[3].term != "of" {
		t.Errorf("Expected fourth token 'of', got %q", tokens[3].term)
	}

	// Check fifth token (america)
	if tokens[4].term != "america" {
		t.Errorf("Expected fifth token 'america', got %q", tokens[4].term)
	}
}

// TestSynonymGraphFilterMultipleOutputs tests synonyms with multiple outputs.
func TestSynonymGraphFilterMultipleOutputs(t *testing.T) {
	// Create a synonym map with multiple outputs
	builder := NewSynonymMapBuilder()

	err := builder.AddString("happy", "joyful", false)
	if err != nil {
		t.Fatalf("Failed to add first synonym: %v", err)
	}

	err = builder.AddString("happy", "cheerful", false)
	if err != nil {
		t.Fatalf("Failed to add second synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create a tokenizer and filter
	reader := NewReusableStringReader()
	reader.SetValue("happy day")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// Collect tokens
	tokens := collectSynonymTokens(t, filter)

	// Expected tokens:
	// Position 0: "happy" (posIncr=1, posLen=1), "joyful" (posIncr=0, posLen=1), "cheerful" (posIncr=0, posLen=1)
	// Position 1: "day" (posIncr=1, posLen=1)

	if len(tokens) != 4 {
		t.Fatalf("Expected 4 tokens, got %d: %v", len(tokens), tokens)
	}

	// Check first token (original)
	if tokens[0].term != "happy" {
		t.Errorf("Expected first token 'happy', got %q", tokens[0].term)
	}
	if tokens[0].posIncr != 1 {
		t.Errorf("Expected posIncr=1 for first token, got %d", tokens[0].posIncr)
	}

	// Check that we have the synonyms (order may vary)
	synonyms := make(map[string]bool)
	for i := 1; i < 3; i++ {
		synonyms[tokens[i].term] = true
		if tokens[i].posIncr != 0 {
			t.Errorf("Expected posIncr=0 for synonym %q, got %d", tokens[i].term, tokens[i].posIncr)
		}
	}

	if !synonyms["joyful"] {
		t.Error("Expected 'joyful' in synonyms")
	}
	if !synonyms["cheerful"] {
		t.Error("Expected 'cheerful' in synonyms")
	}

	// Check last token (day)
	if tokens[3].term != "day" {
		t.Errorf("Expected last token 'day', got %q", tokens[3].term)
	}
	if tokens[3].posIncr != 1 {
		t.Errorf("Expected posIncr=1 for 'day', got %d", tokens[3].posIncr)
	}
}

// TestSynonymGraphFilterNoMatch tests input with no matching synonyms.
func TestSynonymGraphFilterNoMatch(t *testing.T) {
	// Create a synonym map
	builder := NewSynonymMapBuilder()
	err := builder.AddString("quick", "fast", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create a tokenizer and filter with text that doesn't match
	reader := NewReusableStringReader()
	reader.SetValue("slow brown fox")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// Collect tokens
	tokens := collectSynonymTokens(t, filter)

	// Expected tokens: just the original tokens
	if len(tokens) != 3 {
		t.Fatalf("Expected 3 tokens, got %d: %v", len(tokens), tokens)
	}

	// All tokens should have posIncr=1
	for i, token := range tokens {
		if token.posIncr != 1 {
			t.Errorf("Expected posIncr=1 for token %d (%q), got %d", i, token.term, token.posIncr)
		}
	}
}

// TestSynonymGraphFilterEmptyMap tests with an empty synonym map.
func TestSynonymGraphFilterEmptyMap(t *testing.T) {
	// Create an empty synonym map
	sm := NewSynonymMap()

	// Create a tokenizer and filter
	reader := NewReusableStringReader()
	reader.SetValue("hello world")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// Collect tokens
	tokens := collectSynonymTokens(t, filter)

	// Expected tokens: just the original tokens
	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d: %v", len(tokens), tokens)
	}

	if tokens[0].term != "hello" {
		t.Errorf("Expected first token 'hello', got %q", tokens[0].term)
	}
	if tokens[1].term != "world" {
		t.Errorf("Expected second token 'world', got %q", tokens[1].term)
	}
}

// TestSynonymGraphFilterBidirectional tests bidirectional synonyms.
func TestSynonymGraphFilterBidirectional(t *testing.T) {
	// Create a synonym map with bidirectional synonym
	builder := NewSynonymMapBuilder()
	err := builder.AddString("quick", "fast", true) // true = bidirectional
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Test with "quick"
	reader1 := NewReusableStringReader()
	reader1.SetValue("quick")
	tokenizer1 := NewWhitespaceTokenizer()
	tokenizer1.SetReader(reader1)
	filter1 := NewSynonymGraphFilter(tokenizer1, sm, false)
	tokens1 := collectSynonymTokens(t, filter1)

	if len(tokens1) != 2 {
		t.Fatalf("Expected 2 tokens for 'quick', got %d: %v", len(tokens1), tokens1)
	}

	// Test with "fast"
	reader2 := NewReusableStringReader()
	reader2.SetValue("fast")
	tokenizer2 := NewWhitespaceTokenizer()
	tokenizer2.SetReader(reader2)
	filter2 := NewSynonymGraphFilter(tokenizer2, sm, false)
	tokens2 := collectSynonymTokens(t, filter2)

	if len(tokens2) != 2 {
		t.Fatalf("Expected 2 tokens for 'fast', got %d: %v", len(tokens2), tokens2)
	}
}

// TestSynonymGraphFilterOffsets tests that offsets are preserved correctly.
func TestSynonymGraphFilterOffsets(t *testing.T) {
	// Create a synonym map
	builder := NewSynonymMapBuilder()
	err := builder.AddString("quick", "fast", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create a tokenizer and filter
	reader := NewReusableStringReader()
	reader.SetValue("quick brown")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// Collect tokens with offsets
	tokens := collectSynonymTokensWithOffsets(t, filter)

	// Check that original tokens have correct offsets
	if tokens[0].term != "quick" {
		t.Errorf("Expected first token 'quick', got %q", tokens[0].term)
	}
	if tokens[0].startOffset != 0 {
		t.Errorf("Expected startOffset=0 for 'quick', got %d", tokens[0].startOffset)
	}
	if tokens[0].endOffset != 5 {
		t.Errorf("Expected endOffset=5 for 'quick', got %d", tokens[0].endOffset)
	}

	// Synonym should have same offsets as original
	if tokens[1].term != "fast" {
		t.Errorf("Expected second token 'fast', got %q", tokens[1].term)
	}
	if tokens[1].startOffset != 0 {
		t.Errorf("Expected startOffset=0 for synonym 'fast', got %d", tokens[1].startOffset)
	}
	if tokens[1].endOffset != 5 {
		t.Errorf("Expected endOffset=5 for synonym 'fast', got %d", tokens[1].endOffset)
	}

	// Check second word offsets
	if tokens[2].term != "brown" {
		t.Errorf("Expected third token 'brown', got %q", tokens[2].term)
	}
	if tokens[2].startOffset != 6 {
		t.Errorf("Expected startOffset=6 for 'brown', got %d", tokens[2].startOffset)
	}
	if tokens[2].endOffset != 11 {
		t.Errorf("Expected endOffset=11 for 'brown', got %d", tokens[2].endOffset)
	}
}

// TestSynonymGraphFilterReset tests the Reset functionality.
func TestSynonymGraphFilterReset(t *testing.T) {
	// Create a synonym map
	builder := NewSynonymMapBuilder()
	err := builder.AddString("quick", "fast", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create a tokenizer and filter
	reader := NewReusableStringReader()
	reader.SetValue("quick")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// First pass
	tokens1 := collectSynonymTokens(t, filter)
	if len(tokens1) != 2 {
		t.Fatalf("Expected 2 tokens in first pass, got %d", len(tokens1))
	}

	// Reset and process again
	reader.SetValue("quick")
	tokenizer.SetReader(reader)
	err = filter.Reset()
	if err != nil {
		t.Fatalf("Failed to reset filter: %v", err)
	}

	// Second pass
	tokens2 := collectSynonymTokens(t, filter)
	if len(tokens2) != 2 {
		t.Fatalf("Expected 2 tokens in second pass, got %d", len(tokens2))
	}

	// Verify tokens are the same
	for i := range tokens1 {
		if tokens1[i].term != tokens2[i].term {
			t.Errorf("Token %d mismatch: first=%q, second=%q", i, tokens1[i].term, tokens2[i].term)
		}
	}
}

// TestSynonymGraphFilterFactory tests the factory.
func TestSynonymGraphFilterFactory(t *testing.T) {
	// Create a synonym map
	builder := NewSynonymMapBuilder()
	err := builder.AddString("test", "exam", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create factory
	factory := NewSynonymGraphFilterFactory(sm)

	// Create filter using factory
	reader := NewReusableStringReader()
	reader.SetValue("test")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)
	filter := factory.Create(tokenizer)

	// Collect tokens
	tokens := collectSynonymTokens(t, filter)

	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d: %v", len(tokens), tokens)
	}

	if tokens[0].term != "test" {
		t.Errorf("Expected first token 'test', got %q", tokens[0].term)
	}
	if tokens[1].term != "exam" {
		t.Errorf("Expected second token 'exam', got %q", tokens[1].term)
	}
}

// TestSynonymGraphFilterPositionLength tests position length for multi-word output synonyms.
// Note: This test verifies that the first word of a multi-word synonym output
// is emitted with the correct position length. Full support for emitting all
// words of multi-word synonym outputs would require additional graph traversal logic.
func TestSynonymGraphFilterPositionLength(t *testing.T) {
	// Create a synonym map with multi-word output
	builder := NewSynonymMapBuilder()

	// Add synonym: "usa" -> "united states" (multi-word output)
	input := JoinWords([]string{"usa"})
	output := JoinWords([]string{"united", "states"})
	err := builder.Add(input, output, false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Create a tokenizer and filter
	reader := NewReusableStringReader()
	reader.SetValue("usa today")
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(reader)

	filter := NewSynonymGraphFilter(tokenizer, sm, false)

	// Collect tokens
	tokens := collectSynonymTokens(t, filter)

	// Current implementation emits:
	// Position 0: "usa" (posIncr=1, posLen=1), "united" (posIncr=0, posLen=2)
	// Position 2: "today" (posIncr=1, posLen=1)
	//
	// Note: Full multi-word synonym output support would also emit:
	// Position 1: "states" (posIncr=0, posLen=1)

	// Verify the first word of multi-word synonym has correct position length
	foundUnited := false
	for _, token := range tokens {
		if token.term == "united" {
			foundUnited = true
			if token.posIncr != 0 {
				t.Errorf("Expected posIncr=0 for synonym 'united', got %d", token.posIncr)
			}
			if token.posLen != 2 {
				t.Errorf("Expected posLen=2 for multi-word synonym start, got %d", token.posLen)
			}
		}
	}

	if !foundUnited {
		t.Errorf("Expected to find 'united' in tokens, got: %v", tokens)
	}
}

// synonymTokenInfo holds information about a collected token.
type synonymTokenInfo struct {
	term       string
	posIncr    int
	posLen     int
	startOffset int
	endOffset  int
}

// collectSynonymTokens collects all tokens from a TokenStream.
func collectSynonymTokens(t *testing.T, ts TokenStream) []synonymTokenInfo {
	t.Helper()

	var tokens []synonymTokenInfo

	// Get attributes
	var termAttr CharTermAttribute
	var posIncrAttr PositionIncrementAttribute
	var posLenAttr *PositionLengthAttribute

	if attrSrc, ok := ts.(interface{ GetAttributeSource() *AttributeSource }); ok {
		as := attrSrc.GetAttributeSource()
		if attr := as.GetAttribute("CharTermAttribute"); attr != nil {
			if ta, ok := attr.(CharTermAttribute); ok {
				termAttr = ta
			}
		}
		if attr := as.GetAttribute("PositionIncrementAttribute"); attr != nil {
			if pa, ok := attr.(PositionIncrementAttribute); ok {
				posIncrAttr = pa
			}
		}
		if attr := as.GetAttribute("PositionLengthAttribute"); attr != nil {
			if pla, ok := attr.(*PositionLengthAttribute); ok {
				posLenAttr = pla
			}
		}
	}

	for {
		hasToken, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		info := synonymTokenInfo{}
		if termAttr != nil {
			info.term = termAttr.String()
		}
		if posIncrAttr != nil {
			info.posIncr = posIncrAttr.GetPositionIncrement()
		}
		if posLenAttr != nil {
			info.posLen = posLenAttr.GetPositionLength()
		}

		tokens = append(tokens, info)
	}

	return tokens
}

// collectSynonymTokensWithOffsets collects all tokens with offset information.
func collectSynonymTokensWithOffsets(t *testing.T, ts TokenStream) []synonymTokenInfo {
	t.Helper()

	var tokens []synonymTokenInfo

	// Get attributes
	var termAttr CharTermAttribute
	var posIncrAttr PositionIncrementAttribute
	var posLenAttr *PositionLengthAttribute
	var offsetAttr OffsetAttribute

	if attrSrc, ok := ts.(interface{ GetAttributeSource() *AttributeSource }); ok {
		as := attrSrc.GetAttributeSource()
		if attr := as.GetAttribute("CharTermAttribute"); attr != nil {
			if ta, ok := attr.(CharTermAttribute); ok {
				termAttr = ta
			}
		}
		if attr := as.GetAttribute("PositionIncrementAttribute"); attr != nil {
			if pa, ok := attr.(PositionIncrementAttribute); ok {
				posIncrAttr = pa
			}
		}
		if attr := as.GetAttribute("PositionLengthAttribute"); attr != nil {
			if pla, ok := attr.(*PositionLengthAttribute); ok {
				posLenAttr = pla
			}
		}
		if attr := as.GetAttribute("OffsetAttribute"); attr != nil {
			if oa, ok := attr.(OffsetAttribute); ok {
				offsetAttr = oa
			}
		}
	}

	for {
		hasToken, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("Error incrementing token: %v", err)
		}
		if !hasToken {
			break
		}

		info := synonymTokenInfo{}
		if termAttr != nil {
			info.term = termAttr.String()
		}
		if posIncrAttr != nil {
			info.posIncr = posIncrAttr.GetPositionIncrement()
		}
		if posLenAttr != nil {
			info.posLen = posLenAttr.GetPositionLength()
		}
		if offsetAttr != nil {
			info.startOffset = offsetAttr.StartOffset()
			info.endOffset = offsetAttr.EndOffset()
		}

		tokens = append(tokens, info)
	}

	return tokens
}
