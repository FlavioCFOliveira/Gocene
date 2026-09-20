// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// This file re-exports the IOContext factory functions from package spi under
// the names used by Lucene's org.apache.lucene.store.IOContext static API:
//
//   - IOContext.DEFAULT   -> NewIOContext()
//   - IOContext.merge(mi) -> IOContextMerge(mi)
//   - IOContext.flush(fi) -> IOContextFlush(fi)
//
// NewFlushInfo mirrors the canonical constructor of Lucene's FlushInfo record
// (FlushInfo(int numDocs, long estimatedSegmentSize)).

// NewIOContext returns the default IOContext (Lucene's IOContext.DEFAULT).
func NewIOContext() IOContext {
	return NewDefaultIOContext()
}

// IOContextMerge returns an IOContext for merge operations (Lucene's
// IOContext.merge).
func IOContextMerge(mergeInfo *MergeInfo) IOContext {
	return spi.NewMergeContext(mergeInfo)
}

// NewFlushContext returns an IOContext for flush operations (Lucene's
// IOContext.flush). Alias of spi.NewFlushContext.
func NewFlushContext(flushInfo *FlushInfo) IOContext {
	return spi.NewFlushContext(flushInfo)
}

// IOContextFlush returns an IOContext for flush operations. It is the
// Lucene-named form of NewFlushContext.
func IOContextFlush(flushInfo *FlushInfo) IOContext {
	return spi.NewFlushContext(flushInfo)
}

// NewFlushInfo creates a FlushInfo with the given number of documents and
// estimated segment size, mirroring Lucene's FlushInfo record constructor.
func NewFlushInfo(numDocs int, estimatedSegmentSize int64) *FlushInfo {
	return &FlushInfo{
		NumDocs:              numDocs,
		EstimatedSegmentSize: estimatedSegmentSize,
	}
}
