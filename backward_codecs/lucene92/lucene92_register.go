// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import "github.com/FlavioCFOliveira/Gocene/codecs"

func init() {
	// Lucene92HnswVectorsFormat is a backward-compatibility KNN vectors entry.
	// Lucene92HnswVectorsFormat has a Name() method but its FieldsWriter
	// signature predates the codecs.KnnVectorsFormat interface; a read-only
	// stub is registered to satisfy KnnVectorsFormatByName lookups.
	// Mirrors: org.apache.lucene.backward_codecs.lucene92.Lucene92HnswVectorsFormat
	codecs.RegisterKnnVectorsFormat(codecs.NewReadOnlyKnnVectorsFormat("Lucene92HnswVectorsFormat"))
}
