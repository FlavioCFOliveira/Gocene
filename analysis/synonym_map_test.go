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
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestSynonymMapBasic(t *testing.T) {
	builder := NewSynonymMapBuilder()

	// Add simple synonym
	err := builder.AddString("quick", "fast", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Test lookup
	ordinals := sm.LookupString("quick")
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal, got %d", len(ordinals))
	}

	output := sm.GetOutputString(ordinals[0])
	if output != "fast" {
		t.Errorf("Expected 'fast', got %q", output)
	}
}

func TestSynonymMapBidirectional(t *testing.T) {
	builder := NewSynonymMapBuilder()

	// Add bidirectional synonym
	err := builder.AddString("quick", "fast", true)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Test forward lookup
	ordinals := sm.LookupString("quick")
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal for 'quick', got %d", len(ordinals))
	}

	output := sm.GetOutputString(ordinals[0])
	if output != "fast" {
		t.Errorf("Expected 'fast', got %q", output)
	}

	// Test reverse lookup
	ordinals = sm.LookupString("fast")
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal for 'fast', got %d", len(ordinals))
	}

	output = sm.GetOutputString(ordinals[0])
	if output != "quick" {
		t.Errorf("Expected 'quick', got %q", output)
	}
}

func TestSynonymMapMultiWord(t *testing.T) {
	builder := NewSynonymMapBuilder()

	// Add multi-word synonym
	input := JoinWords([]string{"united", "states"})
	output := JoinWords([]string{"usa", "us"})

	err := builder.Add(input, output, false)
	if err != nil {
		t.Fatalf("Failed to add multi-word synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Test lookup
	ordinals := sm.Lookup(input)
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal, got %d", len(ordinals))
	}

	result := sm.GetOutput(ordinals[0])
	expected := []byte("usa")
	expected = append(expected, WORD_SEPARATOR)
	expected = append(expected, []byte("us")...)
	if !bytes.Equal(result.ValidBytes(), expected) {
		t.Errorf("Expected %q, got %q", expected, result.ValidBytes())
	}

	// Check max horizontal context
	if sm.GetMaxHorizontalContext() != 2 {
		t.Errorf("Expected maxHorizontalContext=2, got %d", sm.GetMaxHorizontalContext())
	}
}

func TestSynonymMapMultipleOutputs(t *testing.T) {
	builder := NewSynonymMapBuilder()

	// Add multiple outputs for same input
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

	// Test lookup
	ordinals := sm.LookupString("happy")
	if len(ordinals) != 2 {
		t.Errorf("Expected 2 ordinals, got %d", len(ordinals))
	}

	// Collect outputs
	outputs := make(map[string]bool)
	for _, ord := range ordinals {
		outputs[sm.GetOutputString(ord)] = true
	}

	if !outputs["joyful"] {
		t.Error("Expected 'joyful' in outputs")
	}
	if !outputs["cheerful"] {
		t.Error("Expected 'cheerful' in outputs")
	}
}

func TestSynonymMapDeduplication(t *testing.T) {
	// Test with dedup enabled (default)
	builder := NewSynonymMapBuilder()

	err := builder.AddString("test", "result", false)
	if err != nil {
		t.Fatalf("Failed to add first synonym: %v", err)
	}

	err = builder.AddString("test", "result", false) // Duplicate
	if err != nil {
		t.Fatalf("Failed to add duplicate synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	ordinals := sm.LookupString("test")
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal (dedup), got %d", len(ordinals))
	}

	// Test with dedup disabled
	builder2 := NewSynonymMapBuilderWithDedup(false)

	err = builder2.AddString("test2", "result2", false)
	if err != nil {
		t.Fatalf("Failed to add first synonym: %v", err)
	}

	err = builder2.AddString("test2", "result2", false) // Duplicate
	if err != nil {
		t.Fatalf("Failed to add duplicate synonym: %v", err)
	}

	sm2, err := builder2.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	ordinals = sm2.LookupString("test2")
	if len(ordinals) != 2 {
		t.Errorf("Expected 2 ordinals (no dedup), got %d", len(ordinals))
	}
}

func TestSynonymMapIgnoreCase(t *testing.T) {
	builder := NewSynonymMapBuilder()
	builder.SetIgnoreCase(true)

	err := builder.AddString("Quick", "fast", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// When ignoreCase is true during build, the input is lowercased when stored
	// So we need to lookup with lowercase
	ordinals := sm.LookupString("quick") // "Quick" was lowercased to "quick" during storage
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal for lowercase lookup, got %d", len(ordinals))
	}
}

func TestSynonymMapValidation(t *testing.T) {
	builder := NewSynonymMapBuilder()

	// Test empty input
	err := builder.Add([]byte{}, []byte("output"), false)
	if err == nil {
		t.Error("Expected error for empty input")
	}

	// Test empty output
	err = builder.Add([]byte("input"), []byte{}, false)
	if err == nil {
		t.Error("Expected error for empty output")
	}

	// Test input with holes
	err = builder.Add(
		[]byte{WORD_SEPARATOR, WORD_SEPARATOR},
		[]byte("output"),
		false,
	)
	if err == nil {
		t.Error("Expected error for input with holes")
	}

	// Test output with holes
	err = builder.Add(
		[]byte("input"),
		[]byte{WORD_SEPARATOR, WORD_SEPARATOR},
		false,
	)
	if err == nil {
		t.Error("Expected error for output with holes")
	}
}

func TestSynonymMapEmpty(t *testing.T) {
	sm := NewSynonymMap()

	if !sm.IsEmpty() {
		t.Error("Expected empty synonym map")
	}

	if sm.Size() != 0 {
		t.Errorf("Expected size 0, got %d", sm.Size())
	}

	ordinals := sm.LookupString("anything")
	if ordinals != nil {
		t.Error("Expected nil for empty map lookup")
	}
}

func TestJoinWords(t *testing.T) {
	tests := []struct {
		input    []string
		expected []byte
	}{
		{[]string{}, nil},
		{[]string{"hello"}, []byte("hello")},
		{[]string{"hello", "world"}, []byte("hello\x00world")},
		{[]string{"a", "b", "c"}, []byte("a\x00b\x00c")},
	}

	for _, test := range tests {
		result := JoinWords(test.input)
		if !bytes.Equal(result, test.expected) {
			t.Errorf("JoinWords(%v) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input    []byte
		expected []string
	}{
		{nil, nil},
		{[]byte{}, nil},
		{[]byte("hello"), []string{"hello"}},
		{[]byte("hello\x00world"), []string{"hello", "world"}},
		{[]byte("a\x00b\x00c"), []string{"a", "b", "c"}},
	}

	for _, test := range tests {
		result := SplitWords(test.input)
		if len(result) != len(test.expected) {
			t.Errorf("SplitWords(%q) = %v, expected %v", test.input, result, test.expected)
			continue
		}
		for i := range result {
			if result[i] != test.expected[i] {
				t.Errorf("SplitWords(%q)[%d] = %q, expected %q", test.input, i, result[i], test.expected[i])
			}
		}
	}
}

func TestWordsToString(t *testing.T) {
	input := []byte("hello\x00world")
	result := WordsToString(input)
	expected := "hello world"
	if result != expected {
		t.Errorf("WordsToString(%q) = %q, expected %q", input, result, expected)
	}
}

func TestStringToWords(t *testing.T) {
	input := "hello world"
	result := StringToWords(input)
	expected := []byte("hello\x00world")
	if !bytes.Equal(result, expected) {
		t.Errorf("StringToWords(%q) = %q, expected %q", input, result, expected)
	}
}

func TestHasHoles(t *testing.T) {
	tests := []struct {
		input    []byte
		hasHoles bool
	}{
		{[]byte{}, false},
		{[]byte("hello"), false},
		{[]byte{WORD_SEPARATOR}, false},
		{[]byte{WORD_SEPARATOR, WORD_SEPARATOR}, true},
		{[]byte("a\x00\x00b"), true},
		{[]byte("a\x00b\x00c"), false},
	}

	for _, test := range tests {
		result := hasHoles(test.input)
		if result != test.hasHoles {
			t.Errorf("hasHoles(%q) = %v, expected %v", test.input, result, test.hasHoles)
		}
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input    []byte
		expected int
	}{
		{[]byte{}, 0},
		{[]byte("hello"), 1},
		{[]byte{WORD_SEPARATOR}, 2},
		{[]byte("a\x00b"), 2},
		{[]byte("a\x00b\x00c"), 3},
	}

	for _, test := range tests {
		result := countWords(test.input)
		if result != test.expected {
			t.Errorf("countWords(%q) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestSynonymMapGetEntries(t *testing.T) {
	builder := NewSynonymMapBuilder()

	err := builder.AddString("a", "1", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	err = builder.AddString("b", "2", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	entries := sm.GetEntries()
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

func TestSynonymMapString(t *testing.T) {
	builder := NewSynonymMapBuilder()

	err := builder.AddString("test", "result", false)
	if err != nil {
		t.Fatalf("Failed to add synonym: %v", err)
	}

	sm, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	str := sm.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Should contain key information
	if !bytes.Contains([]byte(str), []byte("SynonymMap{")) {
		t.Error("String representation should start with 'SynonymMap{'")
	}
}

func TestFlatSynonymMap(t *testing.T) {
	fsm := NewFlatSynonymMap(false)

	fsm.Add("quick", []string{"fast", "rapid"})
	fsm.Add("happy", []string{"joyful"})

	// Test lookup
	results := fsm.Lookup("quick")
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Test case sensitivity
	results = fsm.Lookup("QUICK")
	if results != nil {
		t.Error("Expected nil for case-sensitive lookup")
	}

	// Test max words
	if fsm.GetMaxInputWords() != 1 {
		t.Errorf("Expected maxInputWords=1, got %d", fsm.GetMaxInputWords())
	}
	// Both "fast" and "rapid" are single words, so maxOutputWords should be 1
	if fsm.GetMaxOutputWords() != 1 {
		t.Errorf("Expected maxOutputWords=1, got %d", fsm.GetMaxOutputWords())
	}
}

func TestFlatSynonymMapIgnoreCase(t *testing.T) {
	fsm := NewFlatSynonymMap(true)

	fsm.Add("Quick", []string{"Fast"})

	results := fsm.Lookup("QUICK")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for case-insensitive lookup, got %d", len(results))
	}
}

func TestSynonymRules(t *testing.T) {
	rules := NewSynonymRules()

	rules.Add("quick", []string{"fast", "rapid"}, false)
	rules.AddBidirectional("happy", "joyful")

	if rules.Len() != 2 {
		t.Errorf("Expected 2 rules, got %d", rules.Len())
	}

	sm, err := rules.Build()
	if err != nil {
		t.Fatalf("Failed to build synonym map: %v", err)
	}

	// Test first rule
	ordinals := sm.LookupString("quick")
	if len(ordinals) != 2 {
		t.Errorf("Expected 2 ordinals for 'quick', got %d", len(ordinals))
	}

	// Test bidirectional rule
	ordinals = sm.LookupString("happy")
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal for 'happy', got %d", len(ordinals))
	}

	ordinals = sm.LookupString("joyful")
	if len(ordinals) != 1 {
		t.Errorf("Expected 1 ordinal for 'joyful', got %d", len(ordinals))
	}
}

func TestSynonymRulesSort(t *testing.T) {
	rules := NewSynonymRules()

	rules.Add("zebra", []string{"animal"}, false)
	rules.Add("apple", []string{"fruit"}, false)
	rules.Add("mango", []string{"fruit"}, false)

	rules.Sort()

	// After sorting, rules should be in alphabetical order
	if rules.rules[0].Input != "apple" {
		t.Errorf("Expected first rule to be 'apple', got %q", rules.rules[0].Input)
	}
	if rules.rules[1].Input != "mango" {
		t.Errorf("Expected second rule to be 'mango', got %q", rules.rules[1].Input)
	}
	if rules.rules[2].Input != "zebra" {
		t.Errorf("Expected third rule to be 'zebra', got %q", rules.rules[2].Input)
	}
}

func TestBytesRefHash(t *testing.T) {
	hash := NewBytesRefHash()

	br1 := NewBytesRefHashEntry([]byte("hello"))
	br2 := NewBytesRefHashEntry([]byte("world"))
	br3 := NewBytesRefHashEntry([]byte("hello")) // Duplicate

	ord1, err := hash.Add(br1)
	if err != nil {
		t.Fatalf("Failed to add first entry: %v", err)
	}
	if ord1 < 0 {
		t.Error("Expected new ordinal for first entry")
	}

	ord2, err := hash.Add(br2)
	if err != nil {
		t.Fatalf("Failed to add second entry: %v", err)
	}
	if ord2 < 0 {
		t.Error("Expected new ordinal for second entry")
	}

	ord3, err := hash.Add(br3)
	if err != nil {
		t.Fatalf("Failed to add third entry: %v", err)
	}
	if ord3 >= 0 {
		t.Error("Expected negative ordinal for duplicate entry")
	}

	// Size should be 2 (unique entries)
	if hash.Size() != 2 {
		t.Errorf("Expected size 2, got %d", hash.Size())
	}

	// Test Get
	ref := &util.BytesRef{}
	result := hash.Get(0, ref)
	if result == nil {
		t.Error("Expected non-nil result for valid ordinal")
	}

	// Test Find
	found := hash.Find(br1)
	if found < 0 {
		t.Error("Expected to find existing entry")
	}
}

func TestNormalizeInput(t *testing.T) {
	tests := []struct {
		input       []byte
		expected    []byte
		shouldError bool
	}{
		{[]byte("hello"), []byte("hello"), false},
		{[]byte("\x00hello"), []byte("hello"), false},
		{[]byte("hello\x00"), []byte("hello"), false},
		{[]byte("\x00hello\x00"), []byte("hello"), false},
		{[]byte("\x00\x00"), nil, true},
		{[]byte{}, nil, true},
	}

	for _, test := range tests {
		result, err := NormalizeInput(test.input)
		if test.shouldError {
			if err == nil {
				t.Errorf("NormalizeInput(%q) expected error, got nil", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("NormalizeInput(%q) unexpected error: %v", test.input, err)
			} else if !bytes.Equal(result, test.expected) {
				t.Errorf("NormalizeInput(%q) = %q, expected %q", test.input, result, test.expected)
			}
		}
	}
}

func TestValidateUTF8(t *testing.T) {
	// Valid UTF-8
	if err := ValidateUTF8([]byte("hello")); err != nil {
		t.Errorf("Unexpected error for valid UTF-8: %v", err)
	}

	// Invalid UTF-8 (invalid continuation byte)
	invalid := []byte{0xFF, 0xFE}
	if err := ValidateUTF8(invalid); err == nil {
		t.Error("Expected error for invalid UTF-8")
	}
}

func TestUTF8UTF16Conversion(t *testing.T) {
	input := []byte("hello world")

	// UTF8 to UTF16
	utf16 := UTF8ToUTF16(input)
	if utf16 != "hello world" {
		t.Errorf("UTF8ToUTF16 failed: got %q", utf16)
	}

	// UTF16 to UTF8
	utf8 := UTF16ToUTF8(utf16)
	if !bytes.Equal(utf8, input) {
		t.Errorf("UTF16ToUTF8 failed: got %q", utf8)
	}
}

// Helper function to create BytesRef for testing
func NewBytesRefHashEntry(data []byte) *util.BytesRef {
	return util.NewBytesRef(data)
}
