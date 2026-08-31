// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// IndexReaderContext provides context information about an IndexReader.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReaderContext.
//
// IndexReaderContext is the base interface for reader context objects,
// which provide information about where a reader fits in the index hierarchy.
// There are two implementations:
//   - LeafReaderContext for atomic (leaf) readers
//   - CompositeReaderContext for composite readers (e.g., DirectoryReader)
type IndexReaderContext interface {
	// Reader returns the IndexReaderInterface this context refers to.
	Reader() IndexReaderInterface

	// Parent returns the parent context (nil for top-level context).
	Parent() IndexReaderContext

	// IsTopLevel returns true if this is a top-level context.
	IsTopLevel() bool

	// DocBase returns the base document ID for this context.
	// For top-level contexts, this is always 0.
	// For child contexts, this is the sum of MaxDoc() of all preceding siblings.
	DocBase() int

	// IsLeaf returns true if this is a LeafReaderContext.
	IsLeaf() bool
}

// LeafReaderContext provides context information for a LeafReader.
// This is the Go port of Lucene's org.apache.lucene.index.LeafReaderContext.
//
// LeafReaderContext represents a single atomic reader in the index hierarchy.
// It provides information about the reader's position in the composite structure
// and its document ID range.
type LeafReaderContext struct {
	// reader is the underlying LeafReader (can be *LeafReader or *SegmentReader)
	reader IndexReaderInterface

	// parent is the parent context
	parent IndexReaderContext

	// docBase is the base document ID for this leaf
	docBase int

	// ord is the ordinal of this leaf in the parent
	ord int
}

// NewLeafReaderContext creates a new LeafReaderContext.
func NewLeafReaderContext(reader IndexReaderInterface, parent IndexReaderContext, ord int, docBase int) *LeafReaderContext {
	return &LeafReaderContext{
		reader:  reader,
		parent:  parent,
		ord:     ord,
		docBase: docBase,
	}
}

// Reader returns the LeafReader for this context.
func (ctx *LeafReaderContext) Reader() IndexReaderInterface {
	return ctx.reader
}

// Parent returns the parent context.
func (ctx *LeafReaderContext) Parent() IndexReaderContext {
	return ctx.parent
}

// IsTopLevel returns true if this is a top-level context.
func (ctx *LeafReaderContext) IsTopLevel() bool {
	return ctx.parent == nil
}

// DocBase returns the base document ID for this context.
func (ctx *LeafReaderContext) DocBase() int {
	return ctx.docBase
}

// IsLeaf returns true (always true for LeafReaderContext).
func (ctx *LeafReaderContext) IsLeaf() bool {
	return true
}

// Ord returns the ordinal of this leaf in the parent.
func (ctx *LeafReaderContext) Ord() int {
	return ctx.ord
}

// LeafReader returns the underlying LeafReader.
// This returns the reader as LeafReaderInterface.
func (ctx *LeafReaderContext) LeafReader() LeafReaderInterface {
	if leafReader, ok := ctx.reader.(LeafReaderInterface); ok {
		return leafReader
	}
	// Try to get from SegmentReader
	if segReader, ok := ctx.reader.(*SegmentReader); ok {
		return segReader
	}
	return nil
}

// GetLeafReaderContexts returns all leaf reader contexts from an IndexReaderContext.
func GetLeafReaderContexts(ctx IndexReaderContext) []*LeafReaderContext {
	if leafCtx, ok := ctx.(*LeafReaderContext); ok {
		return []*LeafReaderContext{leafCtx}
	}
	if compCtx, ok := ctx.(interface {
		Leaves() ([]*LeafReaderContext, error)
	}); ok {
		leaves, err := compCtx.Leaves()
		if err != nil {
			return nil
		}
		return leaves
	}
	return nil
}

// GetReaderContext gets the context for a reader.
func GetReaderContext(reader IndexReaderInterface) (IndexReaderContext, error) {
	// Check for CompositeReader with GetContext method
	if withContext, ok := reader.(interface {
		GetContext() (IndexReaderContext, error)
	}); ok {
		return withContext.GetContext()
	}

	// Build context for LeafReader
	if leafReader, ok := reader.(*LeafReader); ok {
		return NewLeafReaderContext(leafReader, nil, 0, 0), nil
	}

	// Build context for SegmentReader
	if segReader, ok := reader.(*SegmentReader); ok {
		return NewLeafReaderContext(segReader.LeafReader, nil, 0, 0), nil
	}

	// Build context for DirectoryReader
	if dirReader, ok := reader.(*DirectoryReader); ok {
		builder := NewCompositeReaderContextBuilder(dirReader)
		return builder.Build()
	}

	return nil, fmt.Errorf("unsupported reader type: %T", reader)
}
