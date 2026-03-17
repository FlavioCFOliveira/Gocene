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
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// WORD_SEPARATOR is the character used to separate words in multi-word synonyms.
// This is the same as Lucene's WORD_SEPARATOR.
const WORD_SEPARATOR = 0

// SynonymMap is an immutable map for storing phrase-to-phrase synonyms.
// It maps input phrases to output phrases, supporting multi-word synonyms
// and bidirectional mappings.
//
// This is a port of Lucene's org.apache.lucene.analysis.synonym.SynonymMap.
type SynonymMap struct {
	// fst maps input words to a list of ordinals (as BytesRef).
	// For now, we use a map-based approach until FST is implemented.
	fst map[string][]int

	// words maps ordinals to output words (BytesRef).
	words *BytesRefHash

	// maxHorizontalContext is the maximum context needed on the token stream.
	// This is the maximum number of tokens in any input phrase.
	maxHorizontalContext int

	// maxOutputWords is the maximum number of words in any output phrase.
	maxOutputWords int
}

// NewSynonymMap creates a new empty SynonymMap.
func NewSynonymMap() *SynonymMap {
	return &SynonymMap{
		fst:                  make(map[string][]int),
		words:                NewBytesRefHash(),
		maxHorizontalContext: 0,
		maxOutputWords:       0,
	}
}

// GetMaxHorizontalContext returns the maximum context needed on the token stream.
// This is the maximum number of tokens in any input phrase.
func (sm *SynonymMap) GetMaxHorizontalContext() int {
	return sm.maxHorizontalContext
}

// GetMaxOutputWords returns the maximum number of words in any output phrase.
func (sm *SynonymMap) GetMaxOutputWords() int {
	return sm.maxOutputWords
}

// Lookup looks up the given input text and returns the ordinals of matching outputs.
// The input should be a UTF-8 encoded byte slice with words separated by WORD_SEPARATOR.
func (sm *SynonymMap) Lookup(input []byte) []int {
	if sm.fst == nil {
		return nil
	}
	key := string(input)
	ordinals, ok := sm.fst[key]
	if !ok {
		return nil
	}
	// Return a copy to prevent modification
	result := make([]int, len(ordinals))
	copy(result, ordinals)
	return result
}

// LookupString looks up the given input string and returns the ordinals of matching outputs.
func (sm *SynonymMap) LookupString(input string) []int {
	return sm.Lookup([]byte(input))
}

// GetOutput returns the output BytesRef for the given ordinal.
func (sm *SynonymMap) GetOutput(ordinal int) *util.BytesRef {
	ref := &util.BytesRef{}
	return sm.words.Get(ordinal, ref)
}

// GetOutputString returns the output string for the given ordinal.
func (sm *SynonymMap) GetOutputString(ordinal int) string {
	br := sm.GetOutput(ordinal)
	if br == nil {
		return ""
	}
	return br.String()
}

// WordsSize returns the number of unique output words.
func (sm *SynonymMap) WordsSize() int {
	return sm.words.Size()
}

// IsEmpty returns true if this synonym map has no entries.
func (sm *SynonymMap) IsEmpty() bool {
	return len(sm.fst) == 0
}

// Size returns the number of input entries in this synonym map.
func (sm *SynonymMap) Size() int {
	return len(sm.fst)
}

// SynonymEntry represents a single synonym mapping.
type SynonymEntry struct {
	Input   string
	Outputs []string
}

// GetEntries returns all synonym entries in this map.
// Note: This reconstructs the entries from the internal representation.
func (sm *SynonymMap) GetEntries() []SynonymEntry {
	entries := make([]SynonymEntry, 0, len(sm.fst))
	for input, ordinals := range sm.fst {
		outputs := make([]string, len(ordinals))
		for i, ord := range ordinals {
			outputs[i] = sm.GetOutputString(ord)
		}
		entries = append(entries, SynonymEntry{
			Input:   input,
			Outputs: outputs,
		})
	}
	return entries
}

// Builder is used to construct a SynonymMap.
//
// This is a port of Lucene's SynonymMap.Builder.
type Builder struct {
	// workingSet stores the mappings during building.
	// Key is input (words separated by WORD_SEPARATOR), value is list of output ordinals.
	workingSet map[string][]int

	// words stores the output words and maps them to ordinals.
	words *BytesRefHash

	// maxHorizontalContext tracks the maximum number of words in any input.
	maxHorizontalContext int

	// maxOutputWords tracks the maximum number of words in any output.
	maxOutputWords int

	// dedup if true, identical rules (same input, same output) will be added only once.
	dedup bool

	// ignoreCase if true, input matching is case-insensitive.
	ignoreCase bool
}

// NewSynonymMapBuilder creates a new Builder for constructing a SynonymMap.
func NewSynonymMapBuilder() *Builder {
	return &Builder{
		workingSet:           make(map[string][]int),
		words:                NewBytesRefHash(),
		maxHorizontalContext: 0,
		maxOutputWords:       0,
		dedup:                true,
		ignoreCase:           false,
	}
}

// NewSynonymMapBuilderWithDedup creates a new Builder with deduplication control.
func NewSynonymMapBuilderWithDedup(dedup bool) *Builder {
	b := NewSynonymMapBuilder()
	b.dedup = dedup
	return b
}

// SetIgnoreCase sets whether input matching should be case-insensitive.
func (b *Builder) SetIgnoreCase(ignoreCase bool) *Builder {
	b.ignoreCase = ignoreCase
	return b
}

// Add adds a synonym mapping from input to output.
// If includeOriginal is true, the input is also added as an output (bidirectional).
// Returns an error if the input or output contains holes (consecutive word separators).
func (b *Builder) Add(input, output []byte, includeOriginal bool) error {
	// Validate inputs - check for holes
	if hasHoles(input) {
		return fmt.Errorf("input contains holes: %q", string(input))
	}
	if hasHoles(output) {
		return fmt.Errorf("output contains holes: %q", string(output))
	}

	// Count words in input and output
	inputWordCount := countWords(input)
	outputWordCount := countWords(output)

	if inputWordCount == 0 {
		return errors.New("input must contain at least one word")
	}
	if outputWordCount == 0 {
		return errors.New("output must contain at least one word")
	}

	// Update max horizontal context
	if inputWordCount > b.maxHorizontalContext {
		b.maxHorizontalContext = inputWordCount
	}
	if outputWordCount > b.maxOutputWords {
		b.maxOutputWords = outputWordCount
	}

	// Prepare input key
	inputKey := string(input)
	if b.ignoreCase {
		inputKey = strings.ToLower(inputKey)
	}

	// Add output to words hash and get ordinal
	outputRef := util.NewBytesRef(output)
	ord, err := b.words.Add(outputRef)
	if err != nil {
		return err
	}

	// Check if this rule already exists (if dedup is enabled)
	// Note: ord < 0 means the entry already existed
	isNewRule := ord >= 0
	if ord < 0 {
		ord = -ord - 1
	}

	if b.dedup && !isNewRule {
		if existing, ok := b.workingSet[inputKey]; ok {
			for _, existingOrd := range existing {
				if existingOrd == ord {
					// Duplicate rule, skip
					return nil
				}
			}
		}
	}

	// Add mapping
	b.workingSet[inputKey] = append(b.workingSet[inputKey], ord)

	// If bidirectional, add reverse mapping
	if includeOriginal {
		// Use output as input and input as output
		reverseKey := string(output)
		if b.ignoreCase {
			reverseKey = strings.ToLower(reverseKey)
		}

		// Add input to words hash
		inputRef := util.NewBytesRef(input)
		inputOrd, err := b.words.Add(inputRef)
		if err != nil {
			return err
		}

		isNewInputRule := inputOrd >= 0
		if inputOrd < 0 {
			inputOrd = -inputOrd - 1
		}

		// Check for duplicate
		if b.dedup && !isNewInputRule {
			if existing, ok := b.workingSet[reverseKey]; ok {
				for _, existingOrd := range existing {
					if existingOrd == inputOrd {
						return nil
					}
				}
			}
		}

		b.workingSet[reverseKey] = append(b.workingSet[reverseKey], inputOrd)
	}

	return nil
}

// AddString adds a synonym mapping from input string to output string.
func (b *Builder) AddString(input, output string, includeOriginal bool) error {
	return b.Add([]byte(input), []byte(output), includeOriginal)
}

// AddMulti adds multiple synonym mappings with the same input.
func (b *Builder) AddMulti(input []byte, outputs [][]byte, includeOriginal bool) error {
	for _, output := range outputs {
		if err := b.Add(input, output, includeOriginal); err != nil {
			return err
		}
	}
	return nil
}

// AddMultiString adds multiple synonym mappings with the same input string.
func (b *Builder) AddMultiString(input string, outputs []string, includeOriginal bool) error {
	for _, output := range outputs {
		if err := b.AddString(input, output, includeOriginal); err != nil {
			return err
		}
	}
	return nil
}

// Build constructs and returns the SynonymMap.
func (b *Builder) Build() (*SynonymMap, error) {
	sm := &SynonymMap{
		fst:                  make(map[string][]int, len(b.workingSet)),
		words:                b.words,
		maxHorizontalContext: b.maxHorizontalContext,
		maxOutputWords:       b.maxOutputWords,
	}

	// Copy working set to the final map
	for k, v := range b.workingSet {
		ordinals := make([]int, len(v))
		copy(ordinals, v)
		sm.fst[k] = ordinals
	}

	return sm, nil
}

// hasHoles checks if the byte slice has consecutive word separators.
func hasHoles(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	prevWasSeparator := false
	for _, b := range data {
		if b == WORD_SEPARATOR {
			if prevWasSeparator {
				return true
			}
			prevWasSeparator = true
		} else {
			prevWasSeparator = false
		}
	}
	return false
}

// countWords counts the number of words in a byte slice.
// Words are separated by WORD_SEPARATOR.
func countWords(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := 1
	for _, b := range data {
		if b == WORD_SEPARATOR {
			count++
		}
	}
	return count
}

// JoinWords joins multiple words with WORD_SEPARATOR.
func JoinWords(words []string) []byte {
	if len(words) == 0 {
		return nil
	}
	if len(words) == 1 {
		return []byte(words[0])
	}

	var buf bytes.Buffer
	for i, word := range words {
		if i > 0 {
			buf.WriteByte(WORD_SEPARATOR)
		}
		buf.WriteString(word)
	}
	return buf.Bytes()
}

// SplitWords splits a byte slice into words using WORD_SEPARATOR.
func SplitWords(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	// Count words first
	wordCount := 1
	for _, b := range data {
		if b == WORD_SEPARATOR {
			wordCount++
		}
	}

	words := make([]string, 0, wordCount)
	start := 0
	for i, b := range data {
		if b == WORD_SEPARATOR {
			words = append(words, string(data[start:i]))
			start = i + 1
		}
	}
	// Add last word
	if start < len(data) {
		words = append(words, string(data[start:]))
	}

	return words
}

// WordsToString converts a word slice with WORD_SEPARATOR to a human-readable string.
func WordsToString(data []byte) string {
	words := SplitWords(data)
	return strings.Join(words, " ")
}

// StringToWords converts a space-separated string to WORD_SEPARATOR format.
func StringToWords(s string) []byte {
	words := strings.Fields(s)
	return JoinWords(words)
}

// Parser is an abstract base class for parsing synonym files.
// It extends Builder and provides analyze functionality.
//
// This is a port of Lucene's SynonymMap.Parser.
type Parser struct {
	*Builder
	analyzer Analyzer
}

// NewParser creates a new Parser with the given analyzer.
func NewParser(analyzer Analyzer) *Parser {
	return &Parser{
		Builder:  NewSynonymMapBuilder(),
		analyzer: analyzer,
	}
}

// NewParserWithDedup creates a new Parser with deduplication control.
func NewParserWithDedup(analyzer Analyzer, dedup bool) *Parser {
	return &Parser{
		Builder:  NewSynonymMapBuilderWithDedup(dedup),
		analyzer: analyzer,
	}
}

// Analyze analyzes the given text using the configured analyzer.
// Returns the analyzed text with words separated by WORD_SEPARATOR.
func (p *Parser) Analyze(text string) ([]byte, error) {
	if p.analyzer == nil {
		return StringToWords(text), nil
	}

	// Create a reusable string reader
	reader := NewReusableStringReader()
	reader.SetValue(text)

	// Get token stream
	ts, err := p.analyzer.TokenStream("", reader)
	if err != nil {
		return nil, err
	}
	defer ts.Close()

	// Try to get attributes from the token stream
	var termAttr CharTermAttribute
	var posIncAttr PositionIncrementAttribute

	if attrSrc, ok := ts.(interface{ GetAttributeSource() *AttributeSource }); ok {
		as := attrSrc.GetAttributeSource()
		if attr := as.GetAttribute("CharTermAttribute"); attr != nil {
			if ta, ok := attr.(CharTermAttribute); ok {
				termAttr = ta
			}
		}
		if attr := as.GetAttribute("PositionIncrementAttribute"); attr != nil {
			if pa, ok := attr.(PositionIncrementAttribute); ok {
				posIncAttr = pa
			}
		}
	}

	if termAttr == nil {
		return nil, errors.New("CharTermAttribute not available")
	}
	if posIncAttr == nil {
		return nil, errors.New("PositionIncrementAttribute not available")
	}

	var result bytes.Buffer
	first := true

	for {
		hasToken, err := ts.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !hasToken {
			break
		}

		// Check position increment - must be 1 for synonym parsing
		if posIncAttr.GetPositionIncrement() != 1 {
			return nil, fmt.Errorf("synonym input contains a hole: %q", text)
		}

		if !first {
			result.WriteByte(WORD_SEPARATOR)
		}
		first = false

		result.WriteString(termAttr.String())
	}

	return result.Bytes(), nil
}

// ParseLine parses a single line and adds the synonym mapping.
// This should be overridden by concrete parser implementations.
func (p *Parser) ParseLine(line string) error {
	return errors.New("ParseLine must be implemented by concrete parser")
}

// FlatSynonymMap is a simple in-memory synonym map for quick lookups.
// This is a simplified version that doesn't use FST but provides similar functionality.
type FlatSynonymMap struct {
	// inputToOutputs maps input strings to their output strings.
	inputToOutputs map[string][]string

	// maxInputWords is the maximum number of words in any input.
	maxInputWords int

	// maxOutputWords is the maximum number of words in any output.
	maxOutputWords int

	// ignoreCase determines if lookups are case-insensitive.
	ignoreCase bool
}

// NewFlatSynonymMap creates a new FlatSynonymMap.
func NewFlatSynonymMap(ignoreCase bool) *FlatSynonymMap {
	return &FlatSynonymMap{
		inputToOutputs: make(map[string][]string),
		ignoreCase:     ignoreCase,
	}
}

// Add adds a synonym mapping.
func (fsm *FlatSynonymMap) Add(input string, outputs []string) {
	if fsm.ignoreCase {
		input = strings.ToLower(input)
	}

	// Update max counts
	inputWords := len(strings.Fields(input))
	if inputWords > fsm.maxInputWords {
		fsm.maxInputWords = inputWords
	}

	for _, output := range outputs {
		outputWords := len(strings.Fields(output))
		if outputWords > fsm.maxOutputWords {
			fsm.maxOutputWords = outputWords
		}
	}

	fsm.inputToOutputs[input] = outputs
}

// Lookup looks up synonyms for the given input.
func (fsm *FlatSynonymMap) Lookup(input string) []string {
	if fsm.ignoreCase {
		input = strings.ToLower(input)
	}
	outputs, ok := fsm.inputToOutputs[input]
	if !ok {
		return nil
	}
	result := make([]string, len(outputs))
	copy(result, outputs)
	return result
}

// GetMaxInputWords returns the maximum number of words in any input.
func (fsm *FlatSynonymMap) GetMaxInputWords() int {
	return fsm.maxInputWords
}

// GetMaxOutputWords returns the maximum number of words in any output.
func (fsm *FlatSynonymMap) GetMaxOutputWords() int {
	return fsm.maxOutputWords
}

// IsEmpty returns true if this map has no entries.
func (fsm *FlatSynonymMap) IsEmpty() bool {
	return len(fsm.inputToOutputs) == 0
}

// Size returns the number of entries in this map.
func (fsm *FlatSynonymMap) Size() int {
	return len(fsm.inputToOutputs)
}

// String returns a string representation of the SynonymMap.
func (sm *SynonymMap) String() string {
	var buf bytes.Buffer
	buf.WriteString("SynonymMap{")
	buf.WriteString(fmt.Sprintf("size=%d, ", sm.Size()))
	buf.WriteString(fmt.Sprintf("words=%d, ", sm.WordsSize()))
	buf.WriteString(fmt.Sprintf("maxHorizontalContext=%d, ", sm.maxHorizontalContext))
	buf.WriteString("entries=[")

	first := true
	for input, ordinals := range sm.fst {
		if !first {
			buf.WriteString(", ")
		}
		first = false
		buf.WriteString(fmt.Sprintf("%q->[", input))
		for i, ord := range ordinals {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("%q", sm.GetOutputString(ord)))
		}
		buf.WriteString("]")
	}
	buf.WriteString("]}")
	return buf.String()
}

// BytesRefHash is a hash table for BytesRef that maps to ordinals.
// This is a simple implementation until the full util.BytesRefHash is available.
type BytesRefHash struct {
	// entries maps ordinals to BytesRef values.
	entries []*util.BytesRef

	// ordMap maps hash codes to ordinals for quick lookup.
	ordMap map[string]int
}

// NewBytesRefHash creates a new BytesRefHash.
func NewBytesRefHash() *BytesRefHash {
	return &BytesRefHash{
		entries: make([]*util.BytesRef, 0),
		ordMap:  make(map[string]int),
	}
}

// Add adds a BytesRef to the hash and returns its ordinal.
// If the BytesRef already exists, returns the existing ordinal.
func (brh *BytesRefHash) Add(br *util.BytesRef) (int, error) {
	key := string(br.ValidBytes())
	if ord, ok := brh.ordMap[key]; ok {
		return -(ord + 1), nil // Return negative to indicate existing
	}
	ord := len(brh.entries)
	brh.entries = append(brh.entries, br.Clone())
	brh.ordMap[key] = ord
	return ord, nil
}

// Get returns the BytesRef for the given ordinal.
func (brh *BytesRefHash) Get(ordinal int, ref *util.BytesRef) *util.BytesRef {
	if ordinal < 0 || ordinal >= len(brh.entries) {
		return nil
	}
	ref.Copy(brh.entries[ordinal])
	return ref
}

// Size returns the number of entries in the hash.
func (brh *BytesRefHash) Size() int {
	return len(brh.entries)
}

// Find returns the ordinal of the given BytesRef, or -1 if not found.
func (brh *BytesRefHash) Find(br *util.BytesRef) int {
	key := string(br.ValidBytes())
	if ord, ok := brh.ordMap[key]; ok {
		return ord
	}
	return -1
}

// Ensure SynonymMap and related types implement expected interfaces.
var (
	_ fmt.Stringer = (*SynonymMap)(nil)
)

// UTF8ToUTF16 converts a UTF-8 byte slice to a UTF-16 string.
// This is used for compatibility with Lucene's UTF-16 based processing.
func UTF8ToUTF16(data []byte) string {
	return string(data)
}

// UTF16ToUTF8 converts a UTF-16 string to a UTF-8 byte slice.
func UTF16ToUTF8(s string) []byte {
	return []byte(s)
}

// ValidateUTF8 validates that the given byte slice is valid UTF-8.
func ValidateUTF8(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("invalid UTF-8 sequence")
	}
	return nil
}

// NormalizeInput normalizes the input for consistent processing.
// It validates UTF-8 and removes leading/trailing word separators.
func NormalizeInput(data []byte) ([]byte, error) {
	if err := ValidateUTF8(data); err != nil {
		return nil, err
	}

	// Remove leading word separators
	start := 0
	for start < len(data) && data[start] == WORD_SEPARATOR {
		start++
	}

	// Remove trailing word separators
	end := len(data)
	for end > start && data[end-1] == WORD_SEPARATOR {
		end--
	}

	if start >= end {
		return nil, errors.New("empty input after normalization")
	}

	return data[start:end], nil
}

// SynonymRule represents a single synonym rule with input and outputs.
type SynonymRule struct {
	Input           string
	Outputs         []string
	IncludeOriginal bool
}

// SynonymRules is a collection of synonym rules that can be applied to build a SynonymMap.
type SynonymRules struct {
	rules []SynonymRule
}

// NewSynonymRules creates a new SynonymRules collection.
func NewSynonymRules() *SynonymRules {
	return &SynonymRules{
		rules: make([]SynonymRule, 0),
	}
}

// Add adds a synonym rule.
func (sr *SynonymRules) Add(input string, outputs []string, includeOriginal bool) {
	sr.rules = append(sr.rules, SynonymRule{
		Input:           input,
		Outputs:         outputs,
		IncludeOriginal: includeOriginal,
	})
}

// AddBidirectional adds a bidirectional synonym rule (A->B and B->A).
func (sr *SynonymRules) AddBidirectional(term1, term2 string) {
	sr.rules = append(sr.rules, SynonymRule{
		Input:           term1,
		Outputs:         []string{term2},
		IncludeOriginal: true,
	})
}

// Build builds a SynonymMap from the collected rules.
func (sr *SynonymRules) Build() (*SynonymMap, error) {
	builder := NewSynonymMapBuilder()
	for _, rule := range sr.rules {
		input := StringToWords(rule.Input)
		for _, output := range rule.Outputs {
			outputBytes := StringToWords(output)
			if err := builder.Add(input, outputBytes, rule.IncludeOriginal); err != nil {
				return nil, err
			}
		}
	}
	return builder.Build()
}

// Len returns the number of rules.
func (sr *SynonymRules) Len() int {
	return len(sr.rules)
}

// Sort sorts the rules by input for consistent ordering.
func (sr *SynonymRules) Sort() {
	sort.Slice(sr.rules, func(i, j int) bool {
		return sr.rules[i].Input < sr.rules[j].Input
	})
}
