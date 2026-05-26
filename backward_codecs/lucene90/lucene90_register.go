// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene90PostingsFormat is a backward-compatibility read-only stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene90.Lucene90PostingsFormat
	codecs.RegisterPostingsFormat(codecs.NewReadOnlyPostingsFormat("Lucene90PostingsFormat"))

	// Lucene90HnswVectorsFormat is a backward-compatibility KNN vectors stub.
	// Mirrors: org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene90HnswVectorsFormat"))
}
