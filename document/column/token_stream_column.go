// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenStreamColumn is a Column that provides tokenized values.
// Used exclusively for the inverted index.
// Mirrors org.apache.lucene.document.column.TokenStreamColumn.
type TokenStreamColumn interface {
	Column
	// Tuples returns a fresh tuple cursor starting at the beginning of the batch.
	Tuples() ObjectTupleCursor[*util.BytesRef]
}
