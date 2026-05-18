// Package hyphenation hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.compound.hyphenation.
package hyphenation

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// ByteVector mirrors org.apache.lucene.analysis.compound.hyphenation.ByteVector.
type ByteVector struct{}

// NewByteVector builds a ByteVector.
func NewByteVector() *ByteVector { return &ByteVector{} }

// CharVector mirrors org.apache.lucene.analysis.compound.hyphenation.CharVector.
type CharVector struct{}

// NewCharVector builds a CharVector.
func NewCharVector() *CharVector { return &CharVector{} }

// Hyphen mirrors org.apache.lucene.analysis.compound.hyphenation.Hyphen.
type Hyphen struct{}

// NewHyphen builds a Hyphen.
func NewHyphen() *Hyphen { return &Hyphen{} }

// Hyphenation mirrors org.apache.lucene.analysis.compound.hyphenation.Hyphenation.
type Hyphenation struct{}

// NewHyphenation builds a Hyphenation.
func NewHyphenation() *Hyphenation { return &Hyphenation{} }

// HyphenationTree mirrors org.apache.lucene.analysis.compound.hyphenation.HyphenationTree.
type HyphenationTree struct{}

// NewHyphenationTree builds a HyphenationTree.
func NewHyphenationTree() *HyphenationTree { return &HyphenationTree{} }

// PatternConsumer mirrors org.apache.lucene.analysis.compound.hyphenation.PatternConsumer.
type PatternConsumer struct{}

// NewPatternConsumer builds a PatternConsumer.
func NewPatternConsumer() *PatternConsumer { return &PatternConsumer{} }

// PatternParser mirrors org.apache.lucene.analysis.compound.hyphenation.PatternParser.
type PatternParser struct{}

// NewPatternParser builds a PatternParser.
func NewPatternParser() *PatternParser { return &PatternParser{} }

// TernaryTree mirrors org.apache.lucene.analysis.compound.hyphenation.TernaryTree.
type TernaryTree struct{}

// NewTernaryTree builds a TernaryTree.
func NewTernaryTree() *TernaryTree { return &TernaryTree{} }
