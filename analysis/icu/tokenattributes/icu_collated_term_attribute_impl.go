// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Collator generates binary collation keys for Unicode strings.
//
// This is the Go equivalent of com.ibm.icu.text.Collator. Callers must
// supply a concrete implementation; the icu package does not bundle an ICU
// collation engine.
//
// Deviation: In Java, Collator is the abstract ICU4J class. In Go we define
// a minimal interface that covers only the methods needed by
// ICUCollatedTermAttributeImpl and ICUCollationDocValuesField.
type Collator interface {
	// GetRawCollationKey generates the binary sort key for s and writes it
	// into key, returning the populated key. The key bytes should be
	// byte-comparable in locale-sensitive collation order.
	GetRawCollationKey(s string) []byte
}

// ICUCollatedTermAttributeImpl extends CharTermAttributeImpl by encoding
// the term text as a binary Unicode collation key instead of UTF-8 bytes
// when GetBytesRef is called.
//
// Go port of
// org.apache.lucene.analysis.icu.tokenattributes.ICUCollatedTermAttributeImpl
// (Apache Lucene 10.4.0).
//
// Deviation: The Java original extends CharTermAttributeImpl directly,
// overriding getBytesRef() to call collator.getRawCollationKey(). In Go,
// CharTermAttributeImpl is a concrete struct; this type embeds
// CharTermAttributeImpl and overrides GetBytesRef to apply the collator.
type ICUCollatedTermAttributeImpl struct {
	analysis.CharTermAttributeImpl
	collator Collator
	keyBuf   *util.BytesRef
}

// Compile-time assertions.
var (
	_ analysis.CharTermAttribute     = (*ICUCollatedTermAttributeImpl)(nil)
	_ util.AttributeImpl             = (*ICUCollatedTermAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*ICUCollatedTermAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (c *ICUCollatedTermAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{
		analysis.CharTermAttributeType,
		analysis.TermToBytesRefAttributeType,
	}
}

// NewICUCollatedTermAttributeImpl creates a new
// ICUCollatedTermAttributeImpl backed by the given collator.
//
// The collator is used as-is (callers are responsible for cloning if
// needed to satisfy thread-safety requirements).
func NewICUCollatedTermAttributeImpl(collator Collator) *ICUCollatedTermAttributeImpl {
	return &ICUCollatedTermAttributeImpl{
		CharTermAttributeImpl: *analysis.NewCharTermAttributeImpl(),
		collator:              collator,
		keyBuf:                &util.BytesRef{},
	}
}

// GetBytesRef returns a BytesRef containing the binary collation key for
// the current term text. The returned BytesRef is reused across calls;
// copy it before calling IncrementToken again.
//
// Overrides CharTermAttributeImpl.GetBytesRef to encode via the collator.
func (c *ICUCollatedTermAttributeImpl) GetBytesRef() *util.BytesRef {
	key := c.collator.GetRawCollationKey(c.String())
	c.keyBuf.Bytes = key
	c.keyBuf.Offset = 0
	c.keyBuf.Length = len(key)
	return c.keyBuf
}
