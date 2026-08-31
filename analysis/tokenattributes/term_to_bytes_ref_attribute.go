// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermToBytesRefAttributeType is the reflect.Type of the
// TermToBytesRefAttribute interface, used as the lookup key for
// AttributeSource.
var TermToBytesRefAttributeType = reflect.TypeOf((*TermToBytesRefAttribute)(nil)).Elem()

// TermToBytesRefAttribute is requested by TermsHashPerField to index the contents.
// This attribute can be used to customize the final byte[] encoding of terms.
//
// Consumers of this attribute call GetBytesRef() for each term.
//
// This is the Go port of
// org.apache.lucene.analysis.tokenattributes.TermToBytesRefAttribute.
type TermToBytesRefAttribute interface {
	util.Attribute

	// GetBytesRef retrieves this attribute's BytesRef. The bytes are updated from the current term.
	// The implementation may return a new instance or keep the previous one.
	//
	// The returned BytesRef only stays valid until the token stream gets incremented.
	GetBytesRef() *util.BytesRef
}
