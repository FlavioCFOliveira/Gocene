// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

type DocValues = spi.DocValues
type NumericDocValues = spi.NumericDocValues
type BinaryDocValues = spi.BinaryDocValues
type SortedDocValues = spi.SortedDocValues
type SortedSetDocValues = spi.SortedSetDocValues
type SortedNumericDocValues = spi.SortedNumericDocValues
type TermVectors = spi.TermVectors
type StoredFields = spi.StoredFields
type IndexReader = spi.IndexReaderInterface
type IndexReaderInterface = spi.IndexReaderInterface
type LeafReaderContext = spi.LeafReaderContext
type CompositeReaderContext = spi.CompositeReaderContext
type PointValues = spi.PointValues

// PointTree mirrors the nested interface
// org.apache.lucene.index.PointValues.PointTree. The canonical declaration
// lives in spi so the codec packages and org.apache.lucene.util.bkd can name
// it without importing index; it is re-exported here because
// org.apache.lucene.index is its home in Lucene 10.5.0.
type PointTree = spi.PointTree

// IntersectVisitor mirrors the nested interface
// org.apache.lucene.index.PointValues.IntersectVisitor: the visitor a
// point-values consumer drives during a point-tree walk. The canonical
// declaration lives in spi so the codec packages can implement it without
// importing index; it is re-exported here because org.apache.lucene.index is
// its home in Lucene 10.5.0.
type IntersectVisitor = spi.IntersectVisitor
type IndexReaderMetaData = spi.IndexReaderMetaData
