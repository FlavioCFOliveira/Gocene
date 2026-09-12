// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// IOContext provides context for I/O operations.
//
// This is the Go port of Lucene's org.apache.lucene.store.IOContext.
//
// Lucene 10.4.0 declares IOContext as an interface with two concrete
// implementations (DefaultIOContext plus anonymous classes returned by
// IOContext.merge / IOContext.flush). Gocene models IOContext as a value
// struct with the same observable shape: a discriminating Context field,
// optional MergeInfo/FlushInfo pointers, and a Hints slice carrying any
// FileOpenHint values supplied at open time. This keeps the type cheap to
// pass by value while preserving immutability semantics (callers should
// never mutate a shared IOContext).
type IOContext struct {
	// Context indicates the type of I/O operation.
	Context IOContextType

	// MergeInfo provides information about a merge operation.
	// Only set when Context == ContextMerge.
	MergeInfo *MergeInfo

	// FlushInfo provides information about a flush operation.
	// Only set when Context == ContextFlush.
	FlushInfo *FlushInfo

	// ReadOnce indicates if the file will be read only once.
	// Mirrors the effect of attaching ReadOnceInstance to Hints; retained as
	// a discrete field for Gocene callers that predate the hints model.
	ReadOnce bool

	// Hints carries any FileOpenHint values supplied at open time. May be nil
	// or empty. Implementations of Directory may inspect the hints to drive
	// page-cache advisories or preloading strategies.
	Hints []FileOpenHint
}

// WithHints returns a copy of ctx with the supplied hints appended. Matches
// the semantics of Lucene's IOContext.withHints: the returned context shares
// Context/MergeInfo/FlushInfo with the receiver and adds hints if doing so
// is meaningful for the context. For MERGE / FLUSH contexts Lucene returns
// the same instance; Gocene mirrors that by ignoring extra hints on those
// contexts.
func (ctx IOContext) WithHints(hints ...FileOpenHint) IOContext {
	if ctx.Context == ContextMerge || ctx.Context == ContextFlush {
		return ctx
	}
	if len(hints) == 0 {
		return ctx
	}
	merged := make([]FileOpenHint, 0, len(ctx.Hints)+len(hints))
	merged = append(merged, ctx.Hints...)
	merged = append(merged, hints...)
	return IOContext{
		Context:   ctx.Context,
		MergeInfo: ctx.MergeInfo,
		FlushInfo: ctx.FlushInfo,
		ReadOnce:  ctx.ReadOnce,
		Hints:     merged,
	}
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

	// ContextReadOnce is for one-time read operations (use hints for optimization)
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

// MergeInfo provides information required for a MERGE-context IOContext.
type MergeInfo struct {
	TotalMaxDoc          int
	EstimatedMergeBytes  int64
	IsExternal           bool
	MergeMaxNumSegments  int
	MergeFactor          int // Deprecated
}

// FlushInfo provides information required for a FLUSH-context IOContext.
type FlushInfo struct {
	NumDocs              int
	EstimatedSegmentSize int64
}

// Default IOContext values for common operations.

var IOContextRead = IOContext{Context: ContextRead}
var IOContextWrite = IOContext{Context: ContextWrite}
var IOContextReadOnce = IOContext{
	Context:  ContextReadOnce,
	ReadOnce: true,
	Hints:    []FileOpenHint{DataAccessSequential, ReadOnceInstance},
}
var IOContextDefault = IOContext{Context: ContextRead}

func NewMergeContext(mergeInfo *MergeInfo) IOContext {
	return IOContext{
		Context:   ContextMerge,
		MergeInfo: mergeInfo,
	}
}

func NewFlushContext(flushInfo *FlushInfo) IOContext {
	return IOContext{
		Context:   ContextFlush,
		FlushInfo: flushInfo,
	}
}

func (ctx IOContext) IsMerge() bool {
	return ctx.Context == ContextMerge
}

func (ctx IOContext) IsFlush() bool {
	return ctx.Context == ContextFlush
}

func (ctx IOContext) IsRead() bool {
	return ctx.Context == ContextRead || ctx.Context == ContextReadOnce
}

func (ctx IOContext) IsWrite() bool {
	return ctx.Context == ContextWrite
}
