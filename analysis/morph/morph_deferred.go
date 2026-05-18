// Package morph hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.morph.
package morph

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// BinaryDictionary mirrors org.apache.lucene.analysis.morph.BinaryDictionary.
type BinaryDictionary struct{}

// NewBinaryDictionary builds a BinaryDictionary.
func NewBinaryDictionary() *BinaryDictionary { return &BinaryDictionary{} }

// BinaryDictionaryWriter mirrors org.apache.lucene.analysis.morph.BinaryDictionaryWriter.
type BinaryDictionaryWriter struct{}

// NewBinaryDictionaryWriter builds a BinaryDictionaryWriter.
func NewBinaryDictionaryWriter() *BinaryDictionaryWriter { return &BinaryDictionaryWriter{} }

// CharacterDefinition mirrors org.apache.lucene.analysis.morph.CharacterDefinition.
type CharacterDefinition struct{}

// NewCharacterDefinition builds a CharacterDefinition.
func NewCharacterDefinition() *CharacterDefinition { return &CharacterDefinition{} }

// CharacterDefinitionWriter mirrors org.apache.lucene.analysis.morph.CharacterDefinitionWriter.
type CharacterDefinitionWriter struct{}

// NewCharacterDefinitionWriter builds a CharacterDefinitionWriter.
func NewCharacterDefinitionWriter() *CharacterDefinitionWriter { return &CharacterDefinitionWriter{} }

// ConnectionCosts mirrors org.apache.lucene.analysis.morph.ConnectionCosts.
type ConnectionCosts struct{}

// NewConnectionCosts builds a ConnectionCosts.
func NewConnectionCosts() *ConnectionCosts { return &ConnectionCosts{} }

// ConnectionCostsWriter mirrors org.apache.lucene.analysis.morph.ConnectionCostsWriter.
type ConnectionCostsWriter struct{}

// NewConnectionCostsWriter builds a ConnectionCostsWriter.
func NewConnectionCostsWriter() *ConnectionCostsWriter { return &ConnectionCostsWriter{} }

// Dictionary mirrors org.apache.lucene.analysis.morph.Dictionary.
type Dictionary struct{}

// NewDictionary builds a Dictionary.
func NewDictionary() *Dictionary { return &Dictionary{} }

// DictionaryEntryWriter mirrors org.apache.lucene.analysis.morph.DictionaryEntryWriter.
type DictionaryEntryWriter struct{}

// NewDictionaryEntryWriter builds a DictionaryEntryWriter.
func NewDictionaryEntryWriter() *DictionaryEntryWriter { return &DictionaryEntryWriter{} }

// GraphvizFormatter mirrors org.apache.lucene.analysis.morph.GraphvizFormatter.
type GraphvizFormatter struct{}

// NewGraphvizFormatter builds a GraphvizFormatter.
func NewGraphvizFormatter() *GraphvizFormatter { return &GraphvizFormatter{} }

// MorphData mirrors org.apache.lucene.analysis.morph.MorphData.
type MorphData struct{}

// NewMorphData builds a MorphData.
func NewMorphData() *MorphData { return &MorphData{} }

// Token mirrors org.apache.lucene.analysis.morph.Token.
type Token struct{}

// NewToken builds a Token.
func NewToken() *Token { return &Token{} }

// TokenInfoFST mirrors org.apache.lucene.analysis.morph.TokenInfoFST.
type TokenInfoFST struct{}

// NewTokenInfoFST builds a TokenInfoFST.
func NewTokenInfoFST() *TokenInfoFST { return &TokenInfoFST{} }

// TokenType mirrors org.apache.lucene.analysis.morph.TokenType.
type TokenType struct{}

// NewTokenType builds a TokenType.
func NewTokenType() *TokenType { return &TokenType{} }

// ViterbiNBest mirrors org.apache.lucene.analysis.morph.ViterbiNBest.
type ViterbiNBest struct{}

// NewViterbiNBest builds a ViterbiNBest.
func NewViterbiNBest() *ViterbiNBest { return &ViterbiNBest{} }

// Viterbi mirrors org.apache.lucene.analysis.morph.Viterbi.
type Viterbi struct{}

// NewViterbi builds a Viterbi.
func NewViterbi() *Viterbi { return &Viterbi{} }

