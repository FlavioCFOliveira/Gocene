// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file collects the package-level constants and type re-exports that
// Apache Lucene 10.5.0 declares as static members of index classes. Go has no
// class-scoped constants, so each becomes a package-level declaration under
// the Java constant's own name.

// MAX_TERM_LENGTH is the largest term a document may carry, in bytes, once
// encoded as UTF-8. Mirrors org.apache.lucene.index.IndexWriter.MAX_TERM_LENGTH,
// defined as ByteBlockPool.BYTE_BLOCK_SIZE - 2.
const MAX_TERM_LENGTH = util.ByteBlockSize - 2

// DISABLE_AUTO_FLUSH turns off a flush trigger: assigning it to the RAM
// buffer size, the buffered-document count or the buffered-delete-term count
// stops that criterion from ever forcing a flush. Mirrors
// org.apache.lucene.index.IndexWriterConfig.DISABLE_AUTO_FLUSH.
const DISABLE_AUTO_FLUSH = -1

// DocIdSetIteratorNoMoreDocs is the sentinel document id an iterator returns
// once it is exhausted. Mirrors
// org.apache.lucene.search.DocIdSetIterator.NO_MORE_DOCS, which Lucene defines
// as Integer.MAX_VALUE.
const DocIdSetIteratorNoMoreDocs = math.MaxInt32

// BytesRef is the Go port of org.apache.lucene.util.BytesRef, re-exported here
// under the name the index package uses. The canonical declaration lives in
// util, matching the Java package layout.
type BytesRef = util.BytesRef

// LeafReaderInterface is the leaf-reader contract, spelled the way callers
// outside this package name it.
//
// PORT NOTE: Lucene has one abstract class, LeafReader. Gocene splits it into
// the LeafReader interface and its concrete implementations; the
// LeafReaderInterface alias exists because several packages already name the
// contract that way. Both names denote exactly the same type.
type LeafReader = spi.LeafReader
type LeafReaderInterface = spi.LeafReader
type CacheHelper = spi.CacheHelper
type CompositeReader = spi.CompositeReader
type IndexReaderContext = spi.IndexReaderContext
