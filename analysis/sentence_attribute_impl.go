// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// SentenceAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.SentenceAttributeImpl.
//
// The current implementation is coincidentally identical to
// [FlagsAttribute] in shape (a single int), but the Lucene reference
// keeps it separate because this attribute is not an implied bitmap
// and may carry other sentence-specific data in the future.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/SentenceAttributeImpl.java
type SentenceAttributeImpl struct {
	index int
}

// Compile-time assertions to lock in the contracts this impl
// participates in.
var (
	_ AttributeImpl        = (*SentenceAttributeImpl)(nil)
	_ SentenceAttribute    = (*SentenceAttributeImpl)(nil)
	_ AttributeReflectable = (*SentenceAttributeImpl)(nil)
)

// NewSentenceAttributeImpl initialises this attribute with the default
// sentence index of 0, matching the Lucene no-arg constructor.
func NewSentenceAttributeImpl() *SentenceAttributeImpl {
	return &SentenceAttributeImpl{}
}

// GetSentenceIndex returns the current sentence index.
func (s *SentenceAttributeImpl) GetSentenceIndex() int {
	return s.index
}

// SetSentenceIndex sets the current sentence index. Lucene imposes no
// validation here.
func (s *SentenceAttributeImpl) SetSentenceIndex(sentence int) {
	s.index = sentence
}

// Clear resets the index to 0.
func (s *SentenceAttributeImpl) Clear() {
	s.index = 0
}

// CopyTo copies the sentence index onto target, which must satisfy
// [SentenceAttribute]; a panic with an explanatory message is raised
// otherwise (Lucene cast contract).
func (s *SentenceAttributeImpl) CopyTo(target AttributeImpl) {
	other, ok := target.(SentenceAttribute)
	if !ok {
		panic("SentenceAttributeImpl.CopyTo: target must implement SentenceAttribute")
	}
	other.SetSentenceIndex(s.index)
}

// Copy returns a deep clone of this impl.
func (s *SentenceAttributeImpl) Copy() AttributeImpl {
	return &SentenceAttributeImpl{index: s.index}
}

// ReflectWith pushes the single (SentenceAttribute, "sentences", index)
// triple through reflector, matching the Lucene reference exactly
// (including the unusual plural key "sentences").
func (s *SentenceAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf((*SentenceAttribute)(nil)).Elem(), "sentences", s.index)
}

// Equals returns true if other is a [SentenceAttributeImpl] with the
// same index, matching Lucene's {@code equals(Object)}.
func (s *SentenceAttributeImpl) Equals(other any) bool {
	if s == other {
		return true
	}
	o, ok := other.(*SentenceAttributeImpl)
	if !ok {
		return false
	}
	return s.index == o.index
}

// HashCode returns the index, matching Lucene's {@code hashCode()}.
func (s *SentenceAttributeImpl) HashCode() int {
	return s.index
}
