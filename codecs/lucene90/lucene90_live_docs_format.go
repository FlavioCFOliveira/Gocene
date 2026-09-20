// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import "github.com/FlavioCFOliveira/Gocene/codecs"

// Lucene90LiveDocsFormat mirrors
// org.apache.lucene.codecs.lucene90.Lucene90LiveDocsFormat, the encoder of
// the optional per-segment .liv file.
//
// This is the Lucene-organised name for the format. The implementation lives
// in package codecs (codecs/live_docs_format.go) because this package imports
// codecs, so Go's ban on import cycles prevents codecs — which needs the
// format for Lucene104Codec and CompressingCodec — from importing it from
// here. The alias keeps the Lucene package/class correspondence visible and
// lets callers that follow the Java organisation (Lucene94Codec,
// Lucene95Codec) refer to lucene90.Lucene90LiveDocsFormat exactly as the Java
// sources do.
type Lucene90LiveDocsFormat = codecs.Lucene90LiveDocsFormat

// NewLucene90LiveDocsFormat builds a Lucene90LiveDocsFormat. Mirrors the sole
// constructor of the Java class.
func NewLucene90LiveDocsFormat() *Lucene90LiveDocsFormat {
	return codecs.NewLucene90LiveDocsFormat()
}
