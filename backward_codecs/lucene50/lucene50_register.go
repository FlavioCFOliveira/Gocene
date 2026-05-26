// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene50PostingsFormat is a backward-compatibility read-only stub.
	// The deep-port sprint for this format has not landed; until then,
	// PostingsFormatByName resolves the name but FieldsProducer returns an error.
	// Mirrors: org.apache.lucene.backward_codecs.lucene50.Lucene50PostingsFormat
	codecs.RegisterPostingsFormat(codecs.NewReadOnlyPostingsFormat("Lucene50PostingsFormat"))
}
