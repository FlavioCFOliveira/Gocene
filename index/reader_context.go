// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

// IndexReaderContext represents a hierarchical relationship between IndexReader instances.
// Mirrors org.apache.lucene.index.IndexReaderContext from Apache Lucene 10.5.0.
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
	ID() interface{}

	// Reader returns the IndexReader this context represents.
	Reader() IndexReader

	// Leaves returns the context's leaves if this context is a top-level context.
	// Returns an error if this is not a top-level context.
	Leaves() ([]*LeafReaderContext, error)

	// Children returns the context's children if this context is a composite context, otherwise nil.
	Children() []*IndexReaderContext
}

// baseReaderContext provides common fields for IndexReaderContext implementations.
type baseReaderContext struct {
	parent          *CompositeReaderContext
	isTopLevel      bool
	docBaseInParent int
	ordInParent     int
	identity        interface{}
}

func newBaseReaderContext(parent *CompositeReaderContext, ordInParent, docBaseInParent int) baseReaderContext {
	return baseReaderContext{
		parent:          parent,
		docBaseInParent: docBaseInParent,
		ordInParent:     ordInParent,
		isTopLevel:      parent == nil,
		identity:        struct{}{}, // Simplified identity
	}
}

func (b *baseReaderContext) Parent() *CompositeReaderContext { return b.parent }
func (b *baseReaderContext) IsTopLevel() bool                { return b.isTopLevel }
func (b *baseReaderContext) DocBaseInParent() int           { return b.docBaseInParent }
func (b *baseReaderContext) OrdInParent() int               { return b.ordInParent }
func (b *baseReaderContext) ID() interface{}                { return b.identity }

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

func NewLeafReaderContext(parent *CompositeReaderContext, reader LeafReader, ord, docBase, leafOrd, leafDocBase int) *LeafReaderContext {
	base := newBaseReaderContext(parent, ord, docBase)
	lrc := &LeafReaderContext{
		baseReaderContext: base,
		Ord:              leafOrd,
		DocBase:           leafDocBase,
		reader:            reader,
	}
	if lrc.isTopLevel {
		lrc.leaves = []*LeafReaderContext{lrc}
	}
	return lrc
}

func (l *LeafReaderContext) Reader() IndexReader { return l.reader }

func (l *LeafReaderContext) Leaves() ([]*LeafReaderContext, error) {
	if !l.isTopLevel {
		return nil, fmt.Errorf("this is not a top-level context")
	}
	return l.leaves, nil
}

func (l *LeafReaderContext) Children() []*IndexReaderContext {
	return nil
}

func (l *LeafReaderContext) String() string {
	return fmt.Sprintf("LeafReaderContext(%v docBase=%d ord=%d)", l.reader, l.DocBase, l.Ord)
}
