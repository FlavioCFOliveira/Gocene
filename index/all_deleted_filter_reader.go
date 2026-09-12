// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AllDeletedFilterReader filters the incoming reader and makes all documents appear deleted.
// Mirrors org.apache.lucene.tests.index.AllDeletedFilterReader from Apache Lucene 10.5.0.
type AllDeletedFilterReader struct {
	*FilterLeafReader
	liveDocs util.Bits
}

// NewAllDeletedFilterReader constructs an AllDeletedFilterReader.
func NewAllDeletedFilterReader(in LeafReader) *AllDeletedFilterReader {
	return &AllDeletedFilterReader{
		FilterLeafReader: NewFilterLeafReader(in),
		liveDocs:         util.NewMatchNoBits(in.MaxDoc()),
	}
}

// GetLiveDocs returns a bitset where no documents are live.
func (f *AllDeletedFilterReader) GetLiveDocs() util.Bits {
	return f.liveDocs
}

// NumDocs returns 0, as all documents appear deleted.
func (f *AllDeletedFilterReader) NumDocs() int {
	return 0
}

// GetCoreCacheHelper delegates to the wrapped reader.
func (f *AllDeletedFilterReader) GetCoreCacheHelper() CacheHelper {
	return f.FilterLeafReader.GetCacheHelper()
}

// GetReaderCacheHelper returns nil, as this reader is not suited for caching.
func (f *AllDeletedFilterReader) GetReaderCacheHelper() CacheHelper {
	return nil
}
