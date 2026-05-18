// Package pt hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.pt.
package pt

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// PortugueseMinimalStemFilterFactory mirrors org.apache.lucene.analysis.pt.PortugueseMinimalStemFilterFactory.
type PortugueseMinimalStemFilterFactory struct{}

// NewPortugueseMinimalStemFilterFactory builds a PortugueseMinimalStemFilterFactory.
func NewPortugueseMinimalStemFilterFactory() *PortugueseMinimalStemFilterFactory { return &PortugueseMinimalStemFilterFactory{} }

// PortugueseStemFilterFactory mirrors org.apache.lucene.analysis.pt.PortugueseStemFilterFactory.
type PortugueseStemFilterFactory struct{}

// NewPortugueseStemFilterFactory builds a PortugueseStemFilterFactory.
func NewPortugueseStemFilterFactory() *PortugueseStemFilterFactory { return &PortugueseStemFilterFactory{} }

// RSLPStemmerBase mirrors org.apache.lucene.analysis.pt.RSLPStemmerBase.
type RSLPStemmerBase struct{}

// NewRSLPStemmerBase builds a RSLPStemmerBase.
func NewRSLPStemmerBase() *RSLPStemmerBase { return &RSLPStemmerBase{} }

// PortugueseMinimalStemFilter mirrors org.apache.lucene.analysis.pt.PortugueseMinimalStemFilter.
type PortugueseMinimalStemFilter struct{}

// NewPortugueseMinimalStemFilter builds a PortugueseMinimalStemFilter.
func NewPortugueseMinimalStemFilter() *PortugueseMinimalStemFilter { return &PortugueseMinimalStemFilter{} }

// PortugueseStemFilter mirrors org.apache.lucene.analysis.pt.PortugueseStemFilter.
type PortugueseStemFilter struct{}

// NewPortugueseStemFilter builds a PortugueseStemFilter.
func NewPortugueseStemFilter() *PortugueseStemFilter { return &PortugueseStemFilter{} }

