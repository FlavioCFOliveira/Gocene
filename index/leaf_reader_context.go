// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file ports org.apache.lucene.index.LeafReaderContext (Apache Lucene
// 10.5.0). The struct itself is declared in spi so that the reader
// implementations can construct it without importing index; the type alias
// lives in doc_values_types.go and the constructor is re-exported here,
// because org.apache.lucene.index is the class's home in Lucene.

import "github.com/FlavioCFOliveira/Gocene/spi"

// NewLeafReaderContext creates a LeafReaderContext whose ord/docBase within the
// parent are also its leaf ord/leaf docBase.
//
// Mirrors the LeafReaderContext(CompositeReaderContext, LeafReader, int, int,
// int, int) constructor invoked with leafOrd == ord and leafDocBase == docBase.
func NewLeafReaderContext(reader LeafReader, parent *CompositeReaderContext, ord, docBase int) *LeafReaderContext {
	return spi.NewLeafReaderContext(reader, parent, ord, docBase)
}
