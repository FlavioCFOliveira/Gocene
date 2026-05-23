// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/java/org/apache/lucene/collation/tokenattributes/CollatedTermAttributeImpl.java

// Package tokenattributes provides collation-aware token attributes.
package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Collator maps a string term to its collation key bytes.
//
// This is the Go abstraction for java.text.Collator. Implementations
// must be safe for concurrent use (the Java side clones the Collator in
// the constructor for the same reason).
type Collator interface {
	CollationKey(s string) []byte
}

// CollatedTermAttributeImpl is the Go port of
// org.apache.lucene.collation.tokenattributes.CollatedTermAttributeImpl.
//
// It extends CharTermAttributeImpl and overrides GetBytesRef to return
// the collation-key bytes of the current term instead of raw UTF-8.
type CollatedTermAttributeImpl struct {
	analysis.CharTermAttributeImpl
	collator Collator
}

// Compile-time assertion.
var _ util.AttributeImpl = (*CollatedTermAttributeImpl)(nil)

// NewCollatedTermAttributeImpl creates a CollatedTermAttributeImpl that
// encodes terms via the supplied Collator.
func NewCollatedTermAttributeImpl(collator Collator) *CollatedTermAttributeImpl {
	return &CollatedTermAttributeImpl{
		CharTermAttributeImpl: *analysis.NewCharTermAttributeImpl(),
		collator:              collator,
	}
}

// AttributeInterfaces satisfies util.AttributeInterfaceProvider and
// exposes the same attribute interfaces as CharTermAttributeImpl.
func (c *CollatedTermAttributeImpl) AttributeInterfaces() []reflect.Type {
	return c.CharTermAttributeImpl.AttributeInterfaces()
}

// GetBytesRef overrides CharTermAttributeImpl.GetBytesRef to return the
// collation-key bytes produced by the configured Collator rather than
// raw UTF-8. The returned BytesRef is owned by this attribute and is
// valid until the next mutation.
func (c *CollatedTermAttributeImpl) GetBytesRef() *util.BytesRef {
	key := c.collator.CollationKey(c.CharTermAttributeImpl.String())
	return &util.BytesRef{Bytes: key, Offset: 0, Length: len(key)}
}
