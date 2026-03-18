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
	// reader is the underlying LeafReader
	reader *LeafReader

	// parent is the parent context
	parent IndexReaderContext

	// docBase is the base document ID for this leaf
	docBase int

	// ord is the ordinal of this leaf in the parent
	ord int
}

// NewLeafReaderContext creates a new LeafReaderContext.
func NewLeafReaderContext(reader *LeafReader, parent IndexReaderContext, ord int, docBase int) *LeafReaderContext {
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
func (ctx *LeafReaderContext) LeafReader() *LeafReader {
	return ctx.reader
}

// CompositeReaderContext provides context information for a CompositeReader.
// This is the Go port of Lucene's org.apache.lucene.index.CompositeReaderContext.
//
// CompositeReaderContext represents a composite reader (e.g., DirectoryReader)
// in the index hierarchy. It has child contexts for each sub-reader.
type CompositeReaderContext struct {
	// reader is the underlying composite reader
	reader IndexReaderInterface

	// parent is the parent context
	parent IndexReaderContext

	// children are the child contexts
	children []IndexReaderContext

	// leaves are all leaf contexts in order
	leaves []*LeafReaderContext
}

// NewCompositeReaderContext creates a new CompositeReaderContext.
func NewCompositeReaderContext(reader IndexReaderInterface, parent IndexReaderContext) *CompositeReaderContext {
	return &CompositeReaderContext{
		reader:   reader,
		parent:   parent,
		children: make([]IndexReaderContext, 0),
		leaves:   make([]*LeafReaderContext, 0),
	}
}

// NewCompositeReaderContextWithChildren creates a new CompositeReaderContext with children.
func NewCompositeReaderContextWithChildren(reader IndexReaderInterface, parent IndexReaderContext, children []IndexReaderContext, leaves []*LeafReaderContext) *CompositeReaderContext {
	return &CompositeReaderContext{
		reader:   reader,
		parent:   parent,
		children: children,
		leaves:   leaves,
	}
}

// Reader returns the composite reader for this context.
func (ctx *CompositeReaderContext) Reader() IndexReaderInterface {
	return ctx.reader
}

// Parent returns the parent context.
func (ctx *CompositeReaderContext) Parent() IndexReaderContext {
	return ctx.parent
}

// IsTopLevel returns true if this is a top-level context.
func (ctx *CompositeReaderContext) IsTopLevel() bool {
	return ctx.parent == nil
}

// DocBase returns 0 for composite contexts (docBase is only meaningful for leaves).
func (ctx *CompositeReaderContext) DocBase() int {
	return 0
}

// IsLeaf returns false (always false for CompositeReaderContext).
func (ctx *CompositeReaderContext) IsLeaf() bool {
	return false
}

// Children returns the child contexts.
func (ctx *CompositeReaderContext) Children() []IndexReaderContext {
	return ctx.children
}

// Leaves returns all leaf contexts in order.
func (ctx *CompositeReaderContext) Leaves() []*LeafReaderContext {
	return ctx.leaves
}

// ReaderContextBuilder builds reader contexts from a reader hierarchy.
type ReaderContextBuilder struct {
	reader IndexReaderInterface
}

// NewReaderContextBuilder creates a new ReaderContextBuilder.
func NewReaderContextBuilder(reader IndexReaderInterface) *ReaderContextBuilder {
	return &ReaderContextBuilder{
		reader: reader,
	}
}

// Build builds the reader context hierarchy.
func (b *ReaderContextBuilder) Build() (IndexReaderContext, error) {
	return b.build(nil)
}

// build recursively builds the context hierarchy.
func (b *ReaderContextBuilder) build(parent IndexReaderContext) (IndexReaderContext, error) {
	// Check if this is a leaf reader
	if leafReader, ok := b.reader.(*LeafReader); ok {
		return b.buildLeafContext(leafReader, parent, 0, 0)
	}

	// Check if this is a DirectoryReader (composite)
	if dirReader, ok := b.reader.(*DirectoryReader); ok {
		return b.buildDirectoryContext(dirReader, parent)
	}

	// Check if this is a SegmentReader (leaf via LeafReader embedding)
	if segReader, ok := b.reader.(*SegmentReader); ok {
		return b.buildLeafContext(segReader.LeafReader, parent, 0, 0)
	}

	return nil, fmt.Errorf("unsupported reader type: %T", b.reader)
}

// buildLeafContext builds a LeafReaderContext.
func (b *ReaderContextBuilder) buildLeafContext(reader *LeafReader, parent IndexReaderContext, ord int, docBase int) (*LeafReaderContext, error) {
	return NewLeafReaderContext(reader, parent, ord, docBase), nil
}

// buildDirectoryContext builds a CompositeReaderContext for a DirectoryReader.
func (b *ReaderContextBuilder) buildDirectoryContext(dirReader *DirectoryReader, parent IndexReaderContext) (*CompositeReaderContext, error) {
	children := make([]IndexReaderContext, 0)
	leaves := make([]*LeafReaderContext, 0)

	subReaders := dirReader.GetSequentialSubReaders()
	docBase := 0

	for _, subReader := range subReaders {
		var child IndexReaderContext
		var err error

		// Build context for each sub-reader
		builder := NewReaderContextBuilder(subReader)
		child, err = builder.build(nil)
		if err != nil {
			return nil, err
		}

		children = append(children, child)

		// Collect leaves from child context
		if leafCtx, ok := child.(*LeafReaderContext); ok {
			// Update docBase for leaf context
			leafCtx.docBase = docBase
			leafCtx.ord = len(leaves)
			leaves = append(leaves, leafCtx)
			docBase += subReader.MaxDoc()
		} else if compCtx, ok := child.(*CompositeReaderContext); ok {
			// Update docBase for nested composite context
			for _, leaf := range compCtx.Leaves() {
				leaf.docBase += docBase
				leaf.ord = len(leaves)
				leaves = append(leaves, leaf)
			}
			docBase += subReader.MaxDoc()
		}
	}

	return NewCompositeReaderContextWithChildren(dirReader, parent, children, leaves), nil
}

// GetLeafReaderContexts returns all leaf reader contexts from an IndexReaderContext.
func GetLeafReaderContexts(ctx IndexReaderContext) []*LeafReaderContext {
	if leafCtx, ok := ctx.(*LeafReaderContext); ok {
		return []*LeafReaderContext{leafCtx}
	}
	if compCtx, ok := ctx.(*CompositeReaderContext); ok {
		return compCtx.Leaves()
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
		builder := NewReaderContextBuilder(dirReader)
		return builder.Build()
	}

	return nil, fmt.Errorf("unsupported reader type: %T", reader)
}
