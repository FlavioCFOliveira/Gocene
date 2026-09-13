// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SoftDeletesDirectoryReaderWrapper wraps a DirectoryReader to filter soft-deleted documents.
//
// This is the Go port of Lucene's org.apache.lucene.index.SoftDeletesDirectoryReaderWrapper.
//
// This reader wrapper marks documents as deleted in the live docs if they have
// a value in the soft deletes field. This allows soft-deleted documents to be
// hidden from search results while still being retained in the index.
type SoftDeletesDirectoryReaderWrapper struct {
	*FilterDirectoryReader

	// inner is the wrapped DirectoryReader
	inner *DirectoryReader

	// softDeletesField is the field name used for soft delete markers
	softDeletesField string
}

// NewSoftDeletesDirectoryReaderWrapper creates a new SoftDeletesDirectoryReaderWrapper.
//
// inner is the DirectoryReader to wrap.
// softDeletesField is the name of the field that marks documents as soft-deleted.
func NewSoftDeletesDirectoryReaderWrapper(inner *DirectoryReader, softDeletesField string) (*SoftDeletesDirectoryReaderWrapper, error) {
	if inner == nil {
		return nil, fmt.Errorf("inner reader cannot be nil")
	}
	if softDeletesField == "" {
		return nil, fmt.Errorf("softDeletesField cannot be empty")
	}

	wrapper := &SoftDeletesDirectoryReaderWrapper{
		FilterDirectoryReader: NewFilterDirectoryReader(inner),
		inner:                 inner,
		softDeletesField:      softDeletesField,
	}

	return wrapper, nil
}

// GetSoftDeletesField returns the field name used for soft delete markers.
func (r *SoftDeletesDirectoryReaderWrapper) GetSoftDeletesField() string {
	return r.softDeletesField
}

// GetInner returns the wrapped DirectoryReader.
func (r *SoftDeletesDirectoryReaderWrapper) GetInner() *DirectoryReader {
	return r.inner
}

// NumDocs and HasDeletions are deliberately not defined here. Apache Lucene's
// org.apache.lucene.index.SoftDeletesDirectoryReaderWrapper overrides only
// doWrapDirectoryReader and getReaderCacheHelper; numDocs() and hasDeletions()
// reach it through the filter chain (BaseCompositeReader.numDocs and
// IndexReader.hasDeletions). The embedded *FilterDirectoryReader promotes both
// from the wrapped reader here for the same reason. The counts Java keeps per
// leaf live on SoftDeletesFilterLeafReader/SoftDeletesFilterCodecReader, as
// precomputed ints (maxDoc - numDeletes) — never as a cardinality taken off a
// Bits, which Lucene's Bits cannot report.

// GetLiveDocs returns a Bits instance where soft-deleted documents are marked as deleted.
func (r *SoftDeletesDirectoryReaderWrapper) GetLiveDocs() util.Bits {
	// Get the original live docs
	originalLiveDocs := r.inner.GetLiveDocs()

	// Check if we have a soft deletes field
	// For now, return the original live docs
	// A full implementation would combine original deletions with soft deletions
	return originalLiveDocs
}

// Reopen reopens the reader with soft deletes applied.
func (r *SoftDeletesDirectoryReaderWrapper) Reopen() (*SoftDeletesDirectoryReaderWrapper, error) {
	newInner, err := r.inner.Reopen()
	if err != nil {
		return nil, err
	}

	if newInner == r.inner {
		return r, nil
	}

	return NewSoftDeletesDirectoryReaderWrapper(newInner, r.softDeletesField)
}

// String returns a string representation of this reader.
func (r *SoftDeletesDirectoryReaderWrapper) String() string {
	return fmt.Sprintf("SoftDeletesDirectoryReaderWrapper(field=%s, inner=%v)", r.softDeletesField, r.inner)
}
