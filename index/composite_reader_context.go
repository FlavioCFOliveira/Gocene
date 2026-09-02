// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

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

	// ordInParent is the ordinal of this reader in the parent, 0 if parent is nil
	ordInParent int

	// docBaseInParent is the doc base for this reader in the parent, 0 if parent is nil
	docBaseInParent int
}

// NewCompositeReaderContext creates a new CompositeReaderContext.
func NewCompositeReaderContext(reader IndexReaderInterface, parent IndexReaderContext, ordInParent int, docBaseInParent int, children []IndexReaderContext) *CompositeReaderContext {
	return &CompositeReaderContext{
		reader:          reader,
		parent:          parent,
		ordInParent:     ordInParent,
		docBaseInParent: docBaseInParent,
		children:        children,
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

// NewCompositeReaderContextTopLevel creates a new CompositeReaderContext for top-level readers.
func NewCompositeReaderContextTopLevel(reader IndexReaderInterface, children []IndexReaderContext, leaves []*LeafReaderContext) *CompositeReaderContext {
	return &CompositeReaderContext{
		reader:   reader,
		parent:   nil,
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

// DocBase returns 0 for composite contexts.
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
// It returns an error if this is not a top-level context.
func (ctx *CompositeReaderContext) Leaves() ([]*LeafReaderContext, error) {
	if !ctx.IsTopLevel() {
		return nil, fmt.Errorf("this is not a top-level context")
	}
	return ctx.leaves, nil
}

// CompositeReaderContextBuilder builds reader contexts from a reader hierarchy.
type CompositeReaderContextBuilder struct {
	reader      IndexReaderInterface
	leaves      []*LeafReaderContext
	leafDocBase int
}

// NewCompositeReaderContextBuilder creates a new CompositeReaderContextBuilder.
func NewCompositeReaderContextBuilder(reader IndexReaderInterface) *CompositeReaderContextBuilder {
	return &CompositeReaderContextBuilder{
		reader: reader,
	}
}

// Build builds the reader context hierarchy.
func (b *CompositeReaderContextBuilder) Build() (IndexReaderContext, error) {
	return b.build(nil, b.reader, 0, 0)
}

// build recursively builds the context hierarchy.
func (b *CompositeReaderContextBuilder) build(parent IndexReaderContext, reader IndexReaderInterface, ord int, docBase int) (IndexReaderContext, error) {
	// If the reader is a leaf reader
	if _, ok := reader.(LeafReaderInterface); ok {
		// Create a LeafReaderContext
		leafCtx := NewLeafReaderContext(reader, parent, ord, docBase)
		leafCtx.ord = len(b.leaves)
		leafCtx.docBase = b.leafDocBase

		b.leaves = append(b.leaves, leafCtx)
		b.leafDocBase += reader.MaxDoc()

		return leafCtx, nil
	}

	// If the reader is a composite reader
	if compReader, ok := reader.(CompositeReaderInterface); ok {
		sequentialSubReaders := compReader.GetSequentialSubReaders()
		children := make([]IndexReaderContext, len(sequentialSubReaders))

		var newParent *CompositeReaderContext
		if parent == nil {
			// Top-level composite context
			newParent = NewCompositeReaderContextTopLevel(compReader, children, b.leaves)
		} else {
			// Intermediate composite context
			newParent = NewCompositeReaderContext(compReader, parent, ord, docBase, children)
		}

		newDocBase := 0
		for i, r := range sequentialSubReaders {
			child, err := b.build(newParent, r, i, newDocBase)
			if err != nil {
				return nil, err
			}
			children[i] = child
			newDocBase += r.MaxDoc()
		}

		// Update the children slice in the context
		newParent.children = children

		return newParent, nil
	}

	return nil, fmt.Errorf("unsupported reader type: %T", reader)
}
