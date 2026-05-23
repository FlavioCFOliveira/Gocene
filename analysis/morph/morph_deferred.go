// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package morph provides morphological analysis base types used by Japanese
// (Kuromoji) and Korean (Nori) analyzers.
package morph

import "sort"

// TokenType reflects the original source of a morphological token.
//
// This is the Go port of org.apache.lucene.analysis.morph.TokenType from
// Apache Lucene 10.4.0.
type TokenType int

const (
	// TokenTypeKnown denotes words from the system dictionary.
	TokenTypeKnown TokenType = iota
	// TokenTypeUnknown denotes heuristically segmented unknown words.
	TokenTypeUnknown
	// TokenTypeUser denotes words from the user dictionary.
	TokenTypeUser
)

// String returns the name of the token type.
func (t TokenType) String() string {
	switch t {
	case TokenTypeKnown:
		return "KNOWN"
	case TokenTypeUnknown:
		return "UNKNOWN"
	case TokenTypeUser:
		return "USER"
	default:
		return "UNKNOWN"
	}
}

// MorphData is the high-level interface that exposes morphological information
// stored in a binary dictionary.
//
// This is the Go port of org.apache.lucene.analysis.morph.MorphData from
// Apache Lucene 10.4.0.
type MorphData interface {
	// LeftID returns the left connection ID of the morpheme at morphID.
	LeftID(morphID int) int
	// RightID returns the right connection ID of the morpheme at morphID.
	RightID(morphID int) int
	// WordCost returns the word cost of the morpheme at morphID.
	WordCost(morphID int) int
}

// ConnectionCosts holds the connection cost matrix used by the Viterbi
// decoder. Costs are indexed by [forwardID][backwardID].
//
// This is the Go port of org.apache.lucene.analysis.morph.ConnectionCosts from
// Apache Lucene 10.4.0.
type ConnectionCosts struct {
	matrix      []int16
	forwardSize int
}

// NewConnectionCosts creates a ConnectionCosts with the given matrix dimensions.
// matrix must have forwardSize*backwardSize entries in row-major order.
func NewConnectionCosts(matrix []int16, forwardSize int) *ConnectionCosts {
	return &ConnectionCosts{matrix: matrix, forwardSize: forwardSize}
}

// Get returns the connection cost between forwardID and backwardID.
func (c *ConnectionCosts) Get(forwardID, backwardID int) int {
	if c.matrix == nil {
		return 0
	}
	idx := backwardID*c.forwardSize + forwardID
	if idx < 0 || idx >= len(c.matrix) {
		return 0
	}
	return int(c.matrix[idx])
}

// CharacterDefinition stores character category data used by the Viterbi
// morphological decoder.
//
// This is the Go port of org.apache.lucene.analysis.morph.CharacterDefinition
// from Apache Lucene 10.4.0.
type CharacterDefinition struct {
	categoryMap [0x10000]byte
	invokeMap   []bool
	groupMap    []bool
}

// NewCharacterDefinition creates an empty CharacterDefinition.
func NewCharacterDefinition() *CharacterDefinition { return &CharacterDefinition{} }

// CharacterClass returns the character category for c.
func (cd *CharacterDefinition) CharacterClass(c rune) byte {
	if c < 0x10000 {
		return cd.categoryMap[c]
	}
	return 0
}

// IsInvoke reports whether invoke processing applies to character c.
func (cd *CharacterDefinition) IsInvoke(c rune) bool {
	cat := int(cd.CharacterClass(c))
	if cat < len(cd.invokeMap) {
		return cd.invokeMap[cat]
	}
	return false
}

// IsGroup reports whether character c should be grouped with adjacent characters.
func (cd *CharacterDefinition) IsGroup(c rune) bool {
	cat := int(cd.CharacterClass(c))
	if cat < len(cd.groupMap) {
		return cd.groupMap[cat]
	}
	return false
}

// CharacterDefinitionWriter builds a CharacterDefinition.
//
// This is the Go port of org.apache.lucene.analysis.morph.CharacterDefinitionWriter
// from Apache Lucene 10.4.0.
type CharacterDefinitionWriter struct {
	categoryMap [0x10000]byte
	invokeMap   []bool
	groupMap    []bool
}

// NewCharacterDefinitionWriter creates a writer for the given class count.
func NewCharacterDefinitionWriter(classCount int) *CharacterDefinitionWriter {
	return &CharacterDefinitionWriter{
		invokeMap: make([]bool, classCount),
		groupMap:  make([]bool, classCount),
	}
}

// SetCharacterCategory assigns the category byte for character c.
func (w *CharacterDefinitionWriter) SetCharacterCategory(c rune, category byte) {
	if c < 0x10000 {
		w.categoryMap[c] = category
	}
}

// SetInvokeAndGroupAttributes sets invoke and group flags for classID.
func (w *CharacterDefinitionWriter) SetInvokeAndGroupAttributes(classID int, invoke, group bool) {
	if classID < len(w.invokeMap) {
		w.invokeMap[classID] = invoke
		w.groupMap[classID] = group
	}
}

// Build finalises and returns the CharacterDefinition.
func (w *CharacterDefinitionWriter) Build() *CharacterDefinition {
	cd := &CharacterDefinition{
		invokeMap: w.invokeMap,
		groupMap:  w.groupMap,
	}
	cd.categoryMap = w.categoryMap
	return cd
}

// TokenInfoFST wraps an FST that maps byte sequences to integer word IDs.
//
// This is the Go port of org.apache.lucene.analysis.morph.TokenInfoFST from
// Apache Lucene 10.4.0.
//
// Deviation: the Lucene reference wraps org.apache.lucene.util.fst.FST<Long>.
// This Go port provides a map-backed placeholder; concrete FST wrapping is
// deferred to the kuromoji/nori packages.
type TokenInfoFST struct {
	outputs map[string]int64
}

// NewTokenInfoFST creates an empty TokenInfoFST.
func NewTokenInfoFST() *TokenInfoFST { return &TokenInfoFST{outputs: make(map[string]int64)} }

// Put stores an output value for a byte sequence.
func (f *TokenInfoFST) Put(seq []byte, value int64) { f.outputs[string(seq)] = value }

// Lookup returns the output value for seq, or -1 if not found.
func (f *TokenInfoFST) Lookup(seq []byte) int64 {
	if v, ok := f.outputs[string(seq)]; ok {
		return v
	}
	return -1
}

// BinaryDictionary is the base type for morphological binary dictionaries.
//
// This is the Go port of org.apache.lucene.analysis.morph.BinaryDictionary from
// Apache Lucene 10.4.0.
//
// Deviation: the Lucene reference reads from a pre-built binary resource file
// via codec headers. This Go port provides the struct; concrete subclasses load
// binary data in their own packages (kuromoji, nori).
type BinaryDictionary struct {
	buffer       []byte
	targetMap    []int
	targetMapOff []int
}

// NewBinaryDictionary creates an empty BinaryDictionary.
func NewBinaryDictionary() *BinaryDictionary { return &BinaryDictionary{} }

// Buffer returns the raw dictionary byte buffer.
func (d *BinaryDictionary) Buffer() []byte { return d.buffer }

// Lookup returns the target-map slice for the given word ID.
func (d *BinaryDictionary) Lookup(wordID int) []int {
	if wordID < 0 || wordID >= len(d.targetMapOff) {
		return nil
	}
	start := d.targetMapOff[wordID]
	end := len(d.targetMap)
	if wordID+1 < len(d.targetMapOff) {
		end = d.targetMapOff[wordID+1]
	}
	if start >= end {
		return nil
	}
	return d.targetMap[start:end]
}

// BinaryDictionaryWriter builds and serialises a BinaryDictionary.
//
// This is the Go port of org.apache.lucene.analysis.morph.BinaryDictionaryWriter
// from Apache Lucene 10.4.0.
type BinaryDictionaryWriter struct {
	entries [][]byte
}

// NewBinaryDictionaryWriter creates a new BinaryDictionaryWriter.
func NewBinaryDictionaryWriter() *BinaryDictionaryWriter { return &BinaryDictionaryWriter{} }

// AddEntry appends a raw entry byte slice and returns its word ID.
func (w *BinaryDictionaryWriter) AddEntry(data []byte) int {
	id := len(w.entries)
	w.entries = append(w.entries, data)
	return id
}

// Build assembles all entries into a BinaryDictionary.
func (w *BinaryDictionaryWriter) Build() *BinaryDictionary {
	d := &BinaryDictionary{}
	for _, e := range w.entries {
		d.targetMapOff = append(d.targetMapOff, len(d.buffer))
		d.targetMap = append(d.targetMap, len(d.buffer))
		d.buffer = append(d.buffer, e...)
	}
	return d
}

// ConnectionCostsWriter builds a ConnectionCosts matrix.
//
// This is the Go port of org.apache.lucene.analysis.morph.ConnectionCostsWriter
// from Apache Lucene 10.4.0.
type ConnectionCostsWriter struct {
	forwardSize  int
	backwardSize int
	matrix       []int16
}

// NewConnectionCostsWriter creates a writer for a matrix of the given dimensions.
func NewConnectionCostsWriter(forwardSize, backwardSize int) *ConnectionCostsWriter {
	return &ConnectionCostsWriter{
		forwardSize:  forwardSize,
		backwardSize: backwardSize,
		matrix:       make([]int16, forwardSize*backwardSize),
	}
}

// SetCost stores the connection cost at [forwardID][backwardID].
func (w *ConnectionCostsWriter) SetCost(forwardID, backwardID int, cost int16) {
	if idx := backwardID*w.forwardSize + forwardID; idx >= 0 && idx < len(w.matrix) {
		w.matrix[idx] = cost
	}
}

// Build returns a ConnectionCosts from the written data.
func (w *ConnectionCostsWriter) Build() *ConnectionCosts {
	return NewConnectionCosts(w.matrix, w.forwardSize)
}

// DictionaryEntryWriter writes morphological dictionary entries.
//
// This is the Go port of org.apache.lucene.analysis.morph.DictionaryEntryWriter
// from Apache Lucene 10.4.0.
type DictionaryEntryWriter struct {
	entries []string
}

// NewDictionaryEntryWriter creates a new DictionaryEntryWriter.
func NewDictionaryEntryWriter() *DictionaryEntryWriter { return &DictionaryEntryWriter{} }

// AddEntry records a CSV-formatted dictionary entry.
func (w *DictionaryEntryWriter) AddEntry(entry string) { w.entries = append(w.entries, entry) }

// Entries returns all recorded entries.
func (w *DictionaryEntryWriter) Entries() []string { return w.entries }

// GraphvizFormatter formats a morphological lattice for Graphviz visualisation.
//
// This is the Go port of org.apache.lucene.analysis.morph.GraphvizFormatter from
// Apache Lucene 10.4.0.
type GraphvizFormatter struct {
	lines []string
}

// NewGraphvizFormatter creates a new GraphvizFormatter.
func NewGraphvizFormatter() *GraphvizFormatter { return &GraphvizFormatter{} }

// AddEdge records a directed edge between two node labels.
func (f *GraphvizFormatter) AddEdge(from, to, label string) {
	f.lines = append(f.lines, `  "`+from+`" -> "`+to+`" [label="`+label+`"];`)
}

// String returns the Graphviz DOT representation.
func (f *GraphvizFormatter) String() string {
	s := "digraph {\n"
	for _, l := range f.lines {
		s += l + "\n"
	}
	return s + "}\n"
}

// ViterbiPath represents a single decoded path with its accumulated cost.
type ViterbiPath struct {
	// Positions holds the rune-index boundaries of each segment.
	Positions []int
	// Cost is the accumulated Viterbi cost for this path.
	Cost int
}

// ViterbiNBest stores the N-best lattice results from the Viterbi decoder.
//
// This is the Go port of org.apache.lucene.analysis.morph.ViterbiNBest from
// Apache Lucene 10.4.0.
type ViterbiNBest struct {
	paths []ViterbiPath
}

// NewViterbiNBest creates an empty N-best result holder.
func NewViterbiNBest() *ViterbiNBest { return &ViterbiNBest{} }

// Add appends a decoded path.
func (v *ViterbiNBest) Add(path ViterbiPath) { v.paths = append(v.paths, path) }

// Paths returns all decoded paths in ascending cost order.
func (v *ViterbiNBest) Paths() []ViterbiPath {
	sort.Slice(v.paths, func(i, j int) bool { return v.paths[i].Cost < v.paths[j].Cost })
	return v.paths
}

// Viterbi implements the Viterbi dynamic-programming decoder for morphological
// analysis.
//
// This is the Go port of org.apache.lucene.analysis.morph.Viterbi from
// Apache Lucene 10.4.0.
//
// Deviation: the Lucene reference is a generic class parameterised on MorphData.
// This Go port is concrete; kuromoji and nori embed it with their data types.
// The Decode method is a structural placeholder — full decoding requires the
// complete binary dictionary and TokenInfoFST loaded from language resources.
type Viterbi struct {
	costs   *ConnectionCosts
	charDef *CharacterDefinition
}

// NewViterbi creates a new Viterbi decoder with the given cost matrix and
// character definition tables.
func NewViterbi(costs *ConnectionCosts, charDef *CharacterDefinition) *Viterbi {
	return &Viterbi{costs: costs, charDef: charDef}
}

// Decode runs the Viterbi algorithm on text and returns the best segmentation
// as a slice of (start, end) rune-index pairs.
//
// This structural implementation returns a single span covering the whole
// input; full decoding is provided by the kuromoji/nori codec packages that
// supply a populated BinaryDictionary and TokenInfoFST.
func (v *Viterbi) Decode(text []rune) [][2]int {
	if len(text) == 0 {
		return nil
	}
	return [][2]int{{0, len(text)}}
}

// Token represents a morphological token produced by the Viterbi decoder.
//
// This is the Go port of org.apache.lucene.analysis.morph.Token from
// Apache Lucene 10.4.0.
type Token struct {
	// Surface is the surface form of the token.
	Surface string
	// Start is the start rune index in the original text.
	Start int
	// Length is the number of runes in the token.
	Length int
	// Type is the morphological token type.
	Type TokenType
}

// NewToken creates a new Token.
func NewToken(surface string, start, length int, tokenType TokenType) *Token {
	return &Token{Surface: surface, Start: start, Length: length, Type: tokenType}
}

// Dictionary is the interface for morphological dictionaries.
//
// This is the Go port of org.apache.lucene.analysis.morph.Dictionary from
// Apache Lucene 10.4.0.
type Dictionary interface {
	// Lookup returns the target IDs for the given word ID.
	Lookup(wordID int) []int
}

// Ensure BinaryDictionary implements Dictionary.
var _ Dictionary = (*BinaryDictionary)(nil)
