// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// IOContext provides context for I/O operations.
//
// This is the Go port of Lucene's org.apache.lucene.store.IOContext.
type IOContext struct {
	// Context indicates the type of I/O operation
	Context IOContextType

	// MergeInfo provides information about a merge operation
	// Only set when Context == ContextMerge
	MergeInfo *MergeInfo

	// FlushInfo provides information about a flush operation
	// Only set when Context == ContextFlush
	FlushInfo *FlushInfo

	// ReadOnce indicates if the file will be read only once
	ReadOnce bool
}

// IOContextType represents the type of I/O context.
type IOContextType int

const (
	// ContextRead is for normal read operations
	ContextRead IOContextType = iota

	// ContextWrite is for write operations
	ContextWrite

	// ContextMerge is for merge operations
	ContextMerge

	// ContextFlush is for flush operations
	ContextFlush

	// ContextReadOnce is for one-time read operations (can use hints for optimization)
	ContextReadOnce
)

// String returns the string representation of the IOContextType.
func (t IOContextType) String() string {
	switch t {
	case ContextRead:
		return "READ"
	case ContextWrite:
		return "WRITE"
	case ContextMerge:
		return "MERGE"
	case ContextFlush:
		return "FLUSH"
	case ContextReadOnce:
		return "READONCE"
	default:
		return "UNKNOWN"
	}
}

// MergeInfo provides information about a merge operation.
type MergeInfo struct {
	// TotalMaxDoc is the total number of documents in the merge
	TotalMaxDoc int

	// EstimatedMergeBytes is the estimated size of the merge in bytes
	EstimatedMergeBytes int64

	// IsExternal indicates if this is an external merge
	IsExternal bool

	// MergeFactor is the merge factor (number of segments being merged)
	MergeFactor int
}

// FlushInfo provides information about a flush operation.
type FlushInfo struct {
	// NumDocs is the number of documents being flushed
	NumDocs int

	// EstimatedSegmentSize is the estimated size of the segment in bytes
	EstimatedSegmentSize int64
}

// Default IOContext values for common operations

// IOContextRead is a context for normal read operations.
var IOContextRead = IOContext{Context: ContextRead}

// IOContextWrite is a context for write operations.
var IOContextWrite = IOContext{Context: ContextWrite}

// IOContextReadOnce is a context for one-time read operations.
var IOContextReadOnce = IOContext{Context: ContextReadOnce, ReadOnce: true}

// NewMergeContext creates an IOContext for a merge operation.
func NewMergeContext(mergeInfo *MergeInfo) IOContext {
	return IOContext{
		Context:   ContextMerge,
		MergeInfo: mergeInfo,
	}
}

// NewFlushContext creates an IOContext for a flush operation.
func NewFlushContext(flushInfo *FlushInfo) IOContext {
	return IOContext{
		Context:   ContextFlush,
		FlushInfo: flushInfo,
	}
}

// IsMerge returns true if this is a merge context.
func (ctx IOContext) IsMerge() bool {
	return ctx.Context == ContextMerge
}

// IsFlush returns true if this is a flush context.
func (ctx IOContext) IsFlush() bool {
	return ctx.Context == ContextFlush
}

// IsRead returns true if this is a read context.
func (ctx IOContext) IsRead() bool {
	return ctx.Context == ContextRead || ctx.Context == ContextReadOnce
}

// IsWrite returns true if this is a write context.
func (ctx IOContext) IsWrite() bool {
	return ctx.Context == ContextWrite
}
