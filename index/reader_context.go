// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// IndexReaderContext represents a hierarchical relationship between IndexReader instances.
// Mirrors org.apache.lucene.index.IndexReaderContext from Apache Lucene 10.5.0.
//
// PORT NOTE: Lucene models this as a sealed abstract class permitting only
// CompositeReaderContext and LeafReaderContext. Go has no abstract classes, so the
// contract is an interface and the shared state lives in the embedded
// baseReaderContext struct.
//
// PORT NOTE: Lucene declares `public abstract IndexReader reader()` here and narrows it
// covariantly in the subclasses (LeafReaderContext returns LeafReader,
// CompositeReaderContext returns CompositeReader). Go has no covariant return types, so
// Reader() returns the IndexReaderInterface contract in every implementation and each
// implementation additionally exposes a narrowing accessor (LeafReaderContext.LeafReader,
// CompositeReaderContext.CompositeReader) for callers that need the concrete kind.
type IndexReaderContext interface {
	// Parent is the reader context for this reader's immediate parent, or nil if none.
	Parent() *CompositeReaderContext

	// IsTopLevel is true if this context struct represents the top level reader within the hierarchical context.
	IsTopLevel() bool

	// DocBaseInParent is the doc base for this reader in the parent, 0 if parent is null.
	DocBaseInParent() int

	// OrdInParent is the ord for this reader in the parent, 0 if parent is null.
	OrdInParent() int

	// ID returns an object that uniquely identifies this context without referencing segments.
	ID() any

	// Reader returns the IndexReader this context represents.
	Reader() IndexReaderInterface

	// Leaves returns the context's leaves if this context is a top-level context.
	// Returns an error if this is not a top-level context.
	Leaves() ([]*LeafReaderContext, error)

	// Children returns the context's children if this context is a composite context, otherwise nil.
	Children() []IndexReaderContext
}

// baseReaderContext provides the state Lucene keeps on the abstract IndexReaderContext
// class: parent, isTopLevel, docBaseInParent, ordInParent and the identity object.
type baseReaderContext struct {
	parent          *CompositeReaderContext
	isTopLevel      bool
	docBaseInParent int
	ordInParent     int
	identity        any
}

func newBaseReaderContext(parent *CompositeReaderContext, ordInParent, docBaseInParent int) baseReaderContext {
	return baseReaderContext{
		parent:          parent,
		docBaseInParent: docBaseInParent,
		ordInParent:     ordInParent,
		isTopLevel:      parent == nil,
		// Lucene allocates `final Object identity = new Object()`; a fresh pointer to an
		// empty struct is the Go equivalent of a unique, cheap identity token.
		identity: new(struct{}),
	}
}

func (b *baseReaderContext) Parent() *CompositeReaderContext { return b.parent }
func (b *baseReaderContext) IsTopLevel() bool                { return b.isTopLevel }
func (b *baseReaderContext) DocBaseInParent() int            { return b.docBaseInParent }
func (b *baseReaderContext) OrdInParent() int                { return b.ordInParent }
func (b *baseReaderContext) ID() any                         { return b.identity }

// LeafReaderContext is an IndexReaderContext for LeafReader instances.
// Mirrors org.apache.lucene.index.LeafReaderContext from Apache Lucene 10.5.0.
type LeafReaderContext struct {
	baseReaderContext
	// Ord is the reader's ord in the top-level's leaves array.
	Ord int
	// DocBase is the reader's absolute doc base.
	DocBase int
	reader  LeafReader
	leaves  []*LeafReaderContext
}

// NewLeafReaderContextFull creates a LeafReaderContext, mirroring the six-argument
// package-private constructor
// LeafReaderContext(CompositeReaderContext, LeafReader, int, int, int, int).
func NewLeafReaderContextFull(parent *CompositeReaderContext, reader LeafReader, ord, docBase, leafOrd, leafDocBase int) *LeafReaderContext {
	lrc := &LeafReaderContext{
		baseReaderContext: newBaseReaderContext(parent, ord, docBase),
		Ord:               leafOrd,
		DocBase:           leafDocBase,
		reader:            reader,
	}
	if lrc.isTopLevel {
		lrc.leaves = []*LeafReaderContext{lrc}
	}
	return lrc
}

// NewLeafReaderContext creates a LeafReaderContext whose ord/docBase in the parent are
// also its ord/docBase among the top-level leaves.
//
// PORT NOTE: Lucene overloads the constructor; Go cannot. This is the flat form used
// throughout Gocene, where a leaf sits directly under the top-level reader and therefore
// ordInParent == leafOrd and docBaseInParent == leafDocBase. Nested hierarchies must use
// NewLeafReaderContextFull, which is the exact six-argument Lucene constructor.
func NewLeafReaderContext(reader LeafReader, parent *CompositeReaderContext, ord, docBase int) *LeafReaderContext {
	return NewLeafReaderContextFull(parent, reader, ord, docBase, ord, docBase)
}

// NewLeafReaderContextForReader creates a top-level LeafReaderContext for a single leaf,
// mirroring the one-argument Lucene constructor LeafReaderContext(LeafReader).
func NewLeafReaderContextForReader(reader LeafReader) *LeafReaderContext {
	return NewLeafReaderContextFull(nil, reader, 0, 0, 0, 0)
}

// Reader returns the leaf reader this context represents, as the IndexReader contract.
// Use LeafReader for the narrowed type Lucene's covariant reader() override returns.
func (l *LeafReaderContext) Reader() IndexReaderInterface { return l.reader }

// LeafReader returns the leaf reader this context represents. This is the Go stand-in for
// Lucene's covariant `public LeafReader reader()` override.
func (l *LeafReaderContext) LeafReader() LeafReader { return l.reader }

// Leaves returns this context as the only leaf when it is a top-level context.
func (l *LeafReaderContext) Leaves() ([]*LeafReaderContext, error) {
	if !l.isTopLevel {
		return nil, fmt.Errorf("this is not a top-level context")
	}
	return l.leaves, nil
}

// Children returns nil: a leaf context has no children.
func (l *LeafReaderContext) Children() []IndexReaderContext {
	return nil
}

func (l *LeafReaderContext) String() string {
	return fmt.Sprintf("LeafReaderContext(%v docBase=%d ord=%d)", l.reader, l.DocBase, l.Ord)
}

var _ IndexReaderContext = (*LeafReaderContext)(nil)
