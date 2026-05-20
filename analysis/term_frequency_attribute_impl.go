// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/TermFrequencyAttributeImpl.java

package analysis

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermFrequencyAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.TermFrequencyAttributeImpl.
//
// It is the exported concrete implementation of [TermFrequencyAttribute].
// The default term frequency is 1, matching the Lucene default.
// Unlike other attributes, End() resets to 1 (same as Clear) rather
// than being a no-op, mirroring the Lucene override.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/TermFrequencyAttributeImpl.java
type TermFrequencyAttributeImpl struct {
	termFrequency int
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*TermFrequencyAttributeImpl)(nil)
	_ TermFrequencyAttribute          = (*TermFrequencyAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*TermFrequencyAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (tf *TermFrequencyAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{TermFrequencyAttributeType}
}

// NewTermFrequencyAttributeImpl initialises this attribute with term
// frequency 1, matching the Lucene no-arg constructor.
func NewTermFrequencyAttributeImpl() *TermFrequencyAttributeImpl {
	return &TermFrequencyAttributeImpl{termFrequency: 1}
}

// GetTermFrequency returns the term frequency.
func (tf *TermFrequencyAttributeImpl) GetTermFrequency() int { return tf.termFrequency }

// SetTermFrequency replaces the term frequency without validation.
func (tf *TermFrequencyAttributeImpl) SetTermFrequency(freq int) { tf.termFrequency = freq }

// SetTermFrequencyValidated panics when freq < 1, mirroring Lucene's
// {@code TermFrequencyAttributeImpl#setTermFrequency(int)}.
func (tf *TermFrequencyAttributeImpl) SetTermFrequencyValidated(freq int) {
	if freq < 1 {
		panic(fmt.Sprintf(
			"TermFrequencyAttributeImpl.SetTermFrequencyValidated: term frequency must be 1 or greater; got %d",
			freq))
	}
	tf.termFrequency = freq
}

// Clear resets the term frequency to 1, matching
// {@code TermFrequencyAttributeImpl#clear()}.
func (tf *TermFrequencyAttributeImpl) Clear() { tf.termFrequency = 1 }

// End mirrors {@code TermFrequencyAttributeImpl#end()}: resets to 1.
// Lucene overrides end() here (same value as clear) to document the
// intent explicitly; the Go port follows suit.
func (tf *TermFrequencyAttributeImpl) End() { tf.termFrequency = 1 }

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (tf *TermFrequencyAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &TermFrequencyAttributeImpl{termFrequency: tf.termFrequency}
}

// CopyTo copies the term frequency onto target, which must implement
// [TermFrequencyAttribute]; a panic is raised otherwise.
func (tf *TermFrequencyAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(TermFrequencyAttribute)
	if !ok {
		panic("TermFrequencyAttributeImpl.CopyTo: target must implement TermFrequencyAttribute")
	}
	t.SetTermFrequency(tf.termFrequency)
}

// ReflectWith pushes the (TermFrequencyAttribute, "termFrequency",
// value) triple through reflector, matching the Lucene reference.
func (tf *TermFrequencyAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(TermFrequencyAttributeType, "termFrequency", tf.termFrequency)
}

// Equals returns true if other is a [TermFrequencyAttributeImpl] with
// the same value, matching Lucene's {@code equals(Object)}.
func (tf *TermFrequencyAttributeImpl) Equals(other any) bool {
	if tf == other {
		return true
	}
	o, ok := other.(*TermFrequencyAttributeImpl)
	if !ok {
		return false
	}
	return tf.termFrequency == o.termFrequency
}

// HashCode mirrors {@code Integer.hashCode(termFrequency)}.
func (tf *TermFrequencyAttributeImpl) HashCode() int { return tf.termFrequency }
