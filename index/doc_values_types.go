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

// PointTreeIntersectVisitor mirrors the nested interface
// org.apache.lucene.index.PointValues.IntersectVisitor: the visitor a
// point-values consumer drives during a BKD intersection. The canonical
// declaration lives in spi so the codec packages can implement it without
// importing index; it is re-exported here because org.apache.lucene.index is
// its home in Lucene 10.5.0.
type PointTreeIntersectVisitor = spi.PointTreeIntersectVisitor
type IndexReaderMetaData = spi.IndexReaderMetaData
