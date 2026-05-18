// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermToBytesRefAttribute is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.TermToBytesRefAttribute.
//
// This attribute is requested by TermsHashPerField to index the
// contents of a token. It can be used to customise the final byte
// encoding of a term. Consumers call [TermToBytesRefAttribute.GetBytesRef]
// for each term:
//
//	termAtt := tokenStream.GetAttribute("TermToBytesRefAttribute")
//	for tokenStream.IncrementToken() {
//	    bytes := termAtt.(TermToBytesRefAttribute).GetBytesRef()
//	    if isInteresting(bytes) {
//	        // because the BytesRef is reused by the attribute, make a deep
//	        // copy if you need persistent access to the data.
//	        doSomethingWith(util.DeepCopyOf(bytes))
//	    }
//	}
//
// This is an internal API. End users should rely on [CharTermAttribute]
// (UTF-8 terms) or [BytesTermAttribute] (raw binary terms), both of
// which extend TermToBytesRefAttribute.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/TermToBytesRefAttribute.java
type TermToBytesRefAttribute interface {
	Attribute

	// GetBytesRef retrieves this attribute's BytesRef. The bytes are
	// refreshed from the current term on each call. Implementations
	// may return a new instance or reuse the previous one; the
	// returned BytesRef only stays valid until the token stream is
	// incremented.
	GetBytesRef() *util.BytesRef
}

// TermToBytesRefAttributeType is the reflect.Type of the
// TermToBytesRefAttribute interface, used as the lookup key for
// AttributeSource. Phase 4 (consumer migration) converts all
// string-keyed GetAttribute calls to use these vars.
var TermToBytesRefAttributeType = reflect.TypeOf((*TermToBytesRefAttribute)(nil)).Elem()
