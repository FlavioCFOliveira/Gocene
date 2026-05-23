// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/icu/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ICUCollationKeyAnalyzer configures a KeywordTokenizer with an
// ICUCollationAttributeFactory so each token is encoded as a binary
// Unicode collation key.
//
// Go port of org.apache.lucene.analysis.icu.ICUCollationKeyAnalyzer
// (Apache Lucene 10.4.0).
//
// Deviation: The Java original extends Analyzer and overrides
// createComponents() to return a KeywordTokenizer constructed with the
// ICUCollationAttributeFactory. In Go, Analyzer is an interface; this
// implementation provides a simple TokenStream method that creates a
// pipeline for each call.
type ICUCollationKeyAnalyzer struct {
	factory *ICUCollationAttributeFactory
}

// NewICUCollationKeyAnalyzer creates an analyser that encodes each term
// as a collation key using the given Collator.
func NewICUCollationKeyAnalyzer(collator tokenattributes.Collator) *ICUCollationKeyAnalyzer {
	return &ICUCollationKeyAnalyzer{
		factory: NewICUCollationAttributeFactory(collator),
	}
}

// TokenStream creates a TokenStream that emits a single token containing
// the collation key for the entire input string.
func (a *ICUCollationKeyAnalyzer) TokenStream(_ string, reader io.Reader) (analysis.TokenStream, error) {
	// Read all input and build a single collation-key token.
	src, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return &collationKeyTokenStream{
		factory: a.factory,
		input:   string(src),
	}, nil
}

// Close is a no-op; ICUCollationKeyAnalyzer holds no closeable resources.
func (a *ICUCollationKeyAnalyzer) Close() error { return nil }

// Ensure compile-time interface satisfaction.
var _ analysis.Analyzer = (*ICUCollationKeyAnalyzer)(nil)

// CollationKeyTokenStream is a single-token TokenStream backed by a
// collation key for a given string.
//
// Exported so that callers can retrieve the collation key via GetCollationKey.
type CollationKeyTokenStream = collationKeyTokenStream

// collationKeyTokenStream is the internal implementation.
type collationKeyTokenStream struct {
	factory   *ICUCollationAttributeFactory
	input     string
	emitted   bool
	attrSrc   *util.AttributeSource
	termAttr  *tokenattributes.ICUCollatedTermAttributeImpl
}

func (s *collationKeyTokenStream) init() {
	if s.attrSrc != nil {
		return
	}
	s.attrSrc = util.NewAttributeSourceWithFactory(s.factory)
	rawImpl := s.factory.CreateAttributeInstance(analysis.CharTermAttributeType)
	s.termAttr = rawImpl.(*tokenattributes.ICUCollatedTermAttributeImpl)
}

// IncrementToken emits a single token with the collation key bytes as the
// term text.
func (s *collationKeyTokenStream) IncrementToken() (bool, error) {
	if s.emitted {
		return false, nil
	}
	s.init()
	s.termAttr.SetValue(s.input)
	s.emitted = true
	return true, nil
}

// End is a no-op.
func (s *collationKeyTokenStream) End() error { return nil }

// Close is a no-op.
func (s *collationKeyTokenStream) Close() error { return nil }

// GetAttributeSource returns the AttributeSource so callers can extract
// attributes (e.g. BytesRef for the collation key).
func (s *collationKeyTokenStream) GetAttributeSource() *util.AttributeSource {
	s.init()
	return s.attrSrc
}

// GetCollationKey returns the binary collation key for the current term.
func (s *collationKeyTokenStream) GetCollationKey() *util.BytesRef {
	s.init()
	return s.termAttr.GetBytesRef()
}

// CollationKeyAnalyzerHelper is a convenience helper that encodes a string
// directly to its collation key bytes without creating an analyser pipeline.
func CollationKeyBytes(collator tokenattributes.Collator, s string) []byte {
	return collator.GetRawCollationKey(s)
}

// newReaderFromString returns an io.Reader for a string (helper).
func newReaderFromString(s string) io.Reader {
	return strings.NewReader(s)
}
