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

// DefaultAttributeFactory is the default implementation of AttributeFactory.
// It creates attribute instances using reflection and their default constructors.
type DefaultAttributeFactory struct {
	// creators holds custom creator functions for specific attribute types
	creators map[string]func() AttributeImpl
}

// NewDefaultAttributeFactory creates a new DefaultAttributeFactory with
// pre-configured creators for standard Lucene attribute types.
func NewDefaultAttributeFactory() *DefaultAttributeFactory {
	factory := &DefaultAttributeFactory{
		creators: make(map[string]func() AttributeImpl),
	}

	// Register creators for standard attribute types
	factory.RegisterCreator("CharTermAttribute", func() AttributeImpl {
		return NewCharTermAttribute()
	})

	factory.RegisterCreator("OffsetAttribute", func() AttributeImpl {
		return NewOffsetAttribute()
	})

	factory.RegisterCreator("PositionIncrementAttribute", func() AttributeImpl {
		return NewPositionIncrementAttribute()
	})

	factory.RegisterCreator("TypeAttribute", func() AttributeImpl {
		return NewTypeAttribute()
	})

	factory.RegisterCreator("PayloadAttribute", func() AttributeImpl {
		return NewPayloadAttribute()
	})

	factory.RegisterCreator("FlagsAttribute", func() AttributeImpl {
		return NewFlagsAttribute()
	})

	factory.RegisterCreator("KeywordAttribute", func() AttributeImpl {
		return NewKeywordAttribute()
	})

	factory.RegisterCreator("PositionLengthAttribute", func() AttributeImpl {
		return NewPositionLengthAttribute()
	})

	factory.RegisterCreator("TermFrequencyAttribute", func() AttributeImpl {
		return NewTermFrequencyAttribute()
	})

	return factory
}

// CreateAttributeInstance creates an attribute instance of the given type.
// If a custom creator is registered for the type, it will be used.
// Otherwise, it attempts to create using reflection.
func (daf *DefaultAttributeFactory) CreateAttributeInstance(attribType string) AttributeImpl {
	// Check for custom creator first
	if creator, ok := daf.creators[attribType]; ok {
		return creator()
	}

	// Try to create by type name
	switch attribType {
	case "CharTermAttribute":
		return NewCharTermAttribute()
	case "OffsetAttribute":
		return NewOffsetAttribute()
	case "PositionIncrementAttribute":
		return NewPositionIncrementAttribute()
	case "TypeAttribute":
		return NewTypeAttribute()
	case "PayloadAttribute":
		return NewPayloadAttribute()
	case "FlagsAttribute":
		return NewFlagsAttribute()
	case "KeywordAttribute":
		return NewKeywordAttribute()
	case "PositionLengthAttribute":
		return NewPositionLengthAttribute()
	case "TermFrequencyAttribute":
		return NewTermFrequencyAttribute()
	default:
		return nil
	}
}

// RegisterCreator registers a custom creator function for an attribute type.
// This allows custom instantiation logic for specific attribute types.
func (daf *DefaultAttributeFactory) RegisterCreator(attribType string, creator func() AttributeImpl) {
	daf.creators[attribType] = creator
}

// StaticAttributeFactory is a factory that returns static (pre-created) instances.
// This is useful for testing or when attributes should be shared.
type StaticAttributeFactory struct {
	instances map[string]AttributeImpl
}

// NewStaticAttributeFactory creates a new StaticAttributeFactory.
func NewStaticAttributeFactory() *StaticAttributeFactory {
	return &StaticAttributeFactory{
		instances: make(map[string]AttributeImpl),
	}
}

// RegisterAttribute registers a static instance for an attribute type.
func (saf *StaticAttributeFactory) RegisterAttribute(attribType string, instance AttributeImpl) {
	saf.instances[attribType] = instance
}

// CreateAttributeInstance creates an attribute instance of the given type.
// Returns nil if the type is not registered.
func (saf *StaticAttributeFactory) CreateAttributeInstance(attribType string) AttributeImpl {
	if instance, ok := saf.instances[attribType]; ok {
		return instance
	}
	return nil
}
