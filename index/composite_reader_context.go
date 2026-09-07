// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// CompositeReaderContext is the IndexReaderContext for CompositeReader instances.
// Mirrors org.apache.lucene.index.CompositeReaderContext from Apache Lucene 10.5.0.
//
// PORT NOTE: Lucene stores the reader as a CompositeReader. Gocene stores it as the wider
// IndexReaderInterface because Reader() must satisfy IndexReaderContext (Go has no
// covariant return types); CompositeReader() narrows it back when a caller needs the
// composite contract.
type CompositeReaderContext struct {
	baseReaderContext

	// reader is the underlying composite reader.
	reader IndexReaderInterface

	// children are the child contexts.
	children []IndexReaderContext

	// leaves are all leaf contexts in order, nil when this is not a top-level context.
	leaves []*LeafReaderContext
}

// newCompositeReaderContext is the private all-argument constructor the Lucene
// constructors delegate to.
func newCompositeReaderContext(parent *CompositeReaderContext, reader IndexReaderInterface, ordInParent, docBaseInParent int, children []IndexReaderContext, leaves []*LeafReaderContext) *CompositeReaderContext {
	return &CompositeReaderContext{
		baseReaderContext: newBaseReaderContext(parent, ordInParent, docBaseInParent),
		reader:            reader,
		children:          children,
		leaves:            leaves,
	}
}

// NewCompositeReaderContext creates a CompositeReaderContext for intermediate readers that
// are not top-level readers in the current context. Mirrors
// CompositeReaderContext(CompositeReaderContext, CompositeReader, int, int, List).
func NewCompositeReaderContext(reader IndexReaderInterface, parent *CompositeReaderContext, ordInParent int, docBaseInParent int, children []IndexReaderContext) *CompositeReaderContext {
	return newCompositeReaderContext(parent, reader, ordInParent, docBaseInParent, children, nil)
}

// NewCompositeReaderContextTopLevel creates a CompositeReaderContext for top-level readers,
// with parent set to nil. Mirrors CompositeReaderContext(CompositeReader, List, List).
func NewCompositeReaderContextTopLevel(reader IndexReaderInterface, children []IndexReaderContext, leaves []*LeafReaderContext) *CompositeReaderContext {
	return newCompositeReaderContext(nil, reader, 0, 0, children, leaves)
}

// NewCompositeReaderContextWithChildren creates a CompositeReaderContext with an explicit
// parent, children and leaves.
//
// PORT NOTE: Lucene has no such constructor; Gocene needs it because several readers build
// their context eagerly with both the children and the leaves already known. It delegates
// to the same private constructor as the Lucene forms, and carries the leaves only when the
// context is top-level, exactly as Lucene's two public constructors do.
func NewCompositeReaderContextWithChildren(reader IndexReaderInterface, parent *CompositeReaderContext, children []IndexReaderContext, leaves []*LeafReaderContext) *CompositeReaderContext {
	if parent != nil {
		return newCompositeReaderContext(parent, reader, 0, 0, children, nil)
	}
	return newCompositeReaderContext(nil, reader, 0, 0, children, leaves)
}

// Reader returns the composite reader for this context.
func (ctx *CompositeReaderContext) Reader() IndexReaderInterface {
	return ctx.reader
}

// CompositeReader returns the reader narrowed to the composite contract, or nil when the
// underlying reader does not implement it. This is the Go stand-in for Lucene's covariant
// `public CompositeReader reader()` override.
func (ctx *CompositeReaderContext) CompositeReader() CompositeReaderInterface {
	if cr, ok := ctx.reader.(CompositeReaderInterface); ok {
		return cr
	}
	return nil
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

var _ IndexReaderContext = (*CompositeReaderContext)(nil)

// CompositeReaderContextBuilder builds reader contexts from a reader hierarchy.
// Mirrors the private CompositeReaderContext.Builder class.
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
func (b *CompositeReaderContextBuilder) Build() (*CompositeReaderContext, error) {
	ctx, err := b.build(nil, b.reader, 0, 0)
	if err != nil {
		return nil, err
	}
	top, ok := ctx.(*CompositeReaderContext)
	if !ok {
		return nil, fmt.Errorf("top-level reader %T is not a composite reader", b.reader)
	}
	return top, nil
}

// build recursively builds the context hierarchy.
func (b *CompositeReaderContextBuilder) build(parent *CompositeReaderContext, reader IndexReaderInterface, ord int, docBase int) (IndexReaderContext, error) {
	if ar, ok := reader.(LeafReader); ok {
		leafCtx := NewLeafReaderContextFull(parent, ar, ord, docBase, len(b.leaves), b.leafDocBase)
		b.leaves = append(b.leaves, leafCtx)
		b.leafDocBase += reader.MaxDoc()
		return leafCtx, nil
	}

	compReader, ok := reader.(CompositeReaderInterface)
	if !ok {
		return nil, fmt.Errorf("unsupported reader type: %T", reader)
	}

	sequentialSubReaders := compReader.GetSequentialSubReaders()
	children := make([]IndexReaderContext, len(sequentialSubReaders))

	var newParent *CompositeReaderContext
	if parent == nil {
		newParent = NewCompositeReaderContextTopLevel(compReader, children, nil)
	} else {
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

	// Lucene hands the still-growing `leaves` list to the top-level context by reference.
	// Go slices are values, so the fully built slice is assigned once the walk is done.
	if parent == nil {
		newParent.leaves = b.leaves
	}

	return newParent, nil
}
