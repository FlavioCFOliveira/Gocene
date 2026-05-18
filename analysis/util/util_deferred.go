// Package util hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.util.
package util

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CharArrayIterator mirrors org.apache.lucene.analysis.util.CharArrayIterator.
type CharArrayIterator struct{}

// NewCharArrayIterator builds a CharArrayIterator.
func NewCharArrayIterator() *CharArrayIterator { return &CharArrayIterator{} }

// FilesystemResourceLoader mirrors org.apache.lucene.analysis.util.FilesystemResourceLoader.
type FilesystemResourceLoader struct{}

// NewFilesystemResourceLoader builds a FilesystemResourceLoader.
func NewFilesystemResourceLoader() *FilesystemResourceLoader { return &FilesystemResourceLoader{} }

// OpenStringBuilder mirrors org.apache.lucene.analysis.util.OpenStringBuilder.
type OpenStringBuilder struct{}

// NewOpenStringBuilder builds a OpenStringBuilder.
func NewOpenStringBuilder() *OpenStringBuilder { return &OpenStringBuilder{} }

// RollingCharBuffer mirrors org.apache.lucene.analysis.util.RollingCharBuffer.
type RollingCharBuffer struct{}

// NewRollingCharBuffer builds a RollingCharBuffer.
func NewRollingCharBuffer() *RollingCharBuffer { return &RollingCharBuffer{} }

// SegmentingTokenizerBase mirrors org.apache.lucene.analysis.util.SegmentingTokenizerBase.
type SegmentingTokenizerBase struct{}

// NewSegmentingTokenizerBase builds a SegmentingTokenizerBase.
func NewSegmentingTokenizerBase() *SegmentingTokenizerBase { return &SegmentingTokenizerBase{} }

// UnicodeProps mirrors org.apache.lucene.analysis.util.UnicodeProps.
type UnicodeProps struct{}

// NewUnicodeProps builds a UnicodeProps.
func NewUnicodeProps() *UnicodeProps { return &UnicodeProps{} }

// CharTokenizer mirrors org.apache.lucene.analysis.util.CharTokenizer.
type CharTokenizer struct{}

// NewCharTokenizer builds a CharTokenizer.
func NewCharTokenizer() *CharTokenizer { return &CharTokenizer{} }

