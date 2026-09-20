// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/document"

// InvertableType describes how an IndexableField is processed when inverted
// (indexed). This is the Go port of Apache Lucene 10.5.0's
// org.apache.lucene.document.InvertableType.
//
// Java declares the enum once, in org.apache.lucene.document. Gocene declares
// it once in package document and aliases it here, so that code on the index
// side keeps Lucene's index-package spelling while naming the one type.
type InvertableType = document.InvertableType

const (
	// InvertableTypeBinary indicates the field should be treated as a single
	// value whose binary content is returned by BinaryValue(). The term
	// frequency is assumed to be one.
	InvertableTypeBinary = document.InvertableTypeBinary

	// InvertableTypeTokenStream indicates the field should be inverted
	// through its TokenStream().
	InvertableTypeTokenStream = document.InvertableTypeTokenStream
)
