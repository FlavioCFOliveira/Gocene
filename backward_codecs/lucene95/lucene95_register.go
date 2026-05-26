// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene95HnswVectorsFormat is a backward-compatibility KNN vectors entry.
	// Mirrors: org.apache.lucene.backward_codecs.lucene95.Lucene95HnswVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene95HnswVectorsFormat"))
}
