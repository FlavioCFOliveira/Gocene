// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// Attribute is a marker interface for token attributes.
//
// This is the Go port of Lucene's org.apache.lucene.util.Attribute.
//
// In Lucene's analysis pipeline, Attributes are pieces of information
// associated with a token (e.g., term text, position, offsets, payload).
// TokenStreams and TokenFilters use Attributes to pass information
// between components.
//
// In Go, this is implemented as an empty interface that concrete
// attribute types implement.
type Attribute interface {
	// Attribute is a marker interface - implementations provide their own methods
}

// AttributeImpl is the base implementation for all Attribute implementations.
//
// This is the Go port of Lucene's org.apache.lucene.util.AttributeImpl.
//
// AttributeImpl provides common functionality for attribute implementations
// including cloning and clear operations.
type AttributeImpl interface {
	Attribute
	// Clear clears this attribute, resetting its state.
	// This is called at the end of a token stream.
	Clear()
	// CopyTo copies the contents of this attribute to another implementation.
	CopyTo(target AttributeImpl)
	// Copy creates a deep copy of this attribute.
	Copy() AttributeImpl
}

// AttributeFactory creates instances of AttributeImpl.
//
// This is the Go port of Lucene's org.apache.lucene.util.AttributeFactory.
type AttributeFactory interface {
	// CreateAttributeInstance creates a new instance of the given attribute class.
	CreateAttributeInstance(attribType string) AttributeImpl
}
