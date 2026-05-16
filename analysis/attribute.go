// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package analysis hosts the legacy Gocene token-attribute SPI. The
// Sprint 12 port (option d, minimal-additive) keeps the existing
// [AttributeImpl] interface untouched and layers Lucene-faithful
// behaviour on top through optional sibling interfaces and free
// functions:
//
//   - [AttributeReflector] mirrors org.apache.lucene.util.AttributeReflector.
//   - [AttributeReflectable] is the opt-in counterpart to Lucene's
//     {@code reflectWith(AttributeReflector)}.
//   - [AttributeEnder] is the opt-in counterpart to Lucene's
//     {@code end()} which most impls leave as a Clear() alias.
//   - [ReflectWith], [End] and [ReflectAsString] are package helpers
//     that operate on any [AttributeImpl] regardless of whether it
//     opts into the reflection or end-of-field surfaces.
//
// The Lucene-faithful interface+impl rewrite (and the migration of the
// ~396 consumer references to the [util.AttributeImpl]/[util.AttributeSource]
// subsystem) is tracked in the backlog task created at the end of
// Sprint 12; until then, both packages coexist intentionally.
package analysis

import (
	"fmt"
	"reflect"
	"strings"
)

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
//
// Sprint 12 extends the surface area through optional sibling
// interfaces ([AttributeReflectable], [AttributeEnder]) instead of
// adding methods to this interface, which would break the ~396
// consumer references that depend on it today.
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

// AttributeReflector is the Go port of
// org.apache.lucene.util.AttributeReflector.
//
// In Java this is a {@code @FunctionalInterface} with the single method
// {@code reflect(Class<? extends Attribute>, String key, Object value)}.
// In Go we model it as a function type, which is the canonical Go
// equivalent of a single-method interface.
//
// The first argument is the Attribute interface type (obtained via
// {@code reflect.TypeOf((*FooAttribute)(nil)).Elem()}); the second is
// the property key; the third is the property value, or nil for an
// absent value.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/AttributeReflector.java
type AttributeReflector func(attType reflect.Type, key string, value any)

// AttributeReflectable is the opt-in counterpart to Lucene's
// {@code AttributeImpl#reflectWith(AttributeReflector)}. An impl that
// can describe its own keyed state implements this interface; callers
// that need reflection should go through the [ReflectWith] helper,
// which type-checks the opt-in for any [AttributeImpl] value.
//
// The contract matches Lucene: a single invocation must emit the same
// set of (attType, key, value) triples in the same order on every
// call, so that callers can rely on deterministic output (see
// [ReflectAsString]).
type AttributeReflectable interface {
	// ReflectWith pushes each (attType, key, value) triple this impl
	// exposes through reflector.
	ReflectWith(reflector AttributeReflector)
}

// AttributeEnder is the opt-in counterpart to Lucene's
// {@code AttributeImpl#end()}. Lucene's default behaviour delegates to
// {@code clear()} which most impls inherit; only impls that have a
// distinct end-of-field state need to implement this interface. The
// [End] helper falls back to [AttributeImpl.Clear] when the impl does
// not opt in.
type AttributeEnder interface {
	// End resets this impl to its end-of-field state.
	End()
}

// ReflectWith dispatches reflector against impl. If impl implements
// [AttributeReflectable] its own ReflectWith is invoked; otherwise the
// helper is a no-op, matching the Lucene contract where the base
// {@code reflectWith} default is to emit nothing for impls that have
// not overridden the hook in a meaningful way.
//
// Callers should prefer this helper over a manual type assertion so
// that the opt-in surface remains the single point of change when the
// Sprint 12 follow-up migrates consumers to [util.AttributeImpl].
func ReflectWith(impl AttributeImpl, reflector AttributeReflector) {
	if r, ok := impl.(AttributeReflectable); ok {
		r.ReflectWith(reflector)
	}
}

// End dispatches the end-of-field hook for impl. If impl implements
// [AttributeEnder] its End method is invoked; otherwise End falls back
// to Clear, which matches the Java default
// {@code AttributeImpl#end() -> clear()}.
func End(impl AttributeImpl) {
	if e, ok := impl.(AttributeEnder); ok {
		e.End()
		return
	}
	impl.Clear()
}

// ReflectAsString is the Go port of
// {@code AttributeImpl#reflectAsString(boolean)}. It returns the
// current attribute values as a comma-separated string in one of two
// formats, byte-for-byte compatible with the Lucene reference:
//
//	prependAttClass=true  : "AttributeClass#key=value,AttributeClass#key=value"
//	prependAttClass=false : "key=value,key=value"
//
// Impls that do not opt into [AttributeReflectable] produce the empty
// string, matching the Lucene default. nil values render as the
// literal "null", matching the Java reference.
func ReflectAsString(impl AttributeImpl, prependAttClass bool) string {
	var sb strings.Builder
	ReflectWith(impl, func(attType reflect.Type, key string, value any) {
		if sb.Len() > 0 {
			sb.WriteByte(',')
		}
		if prependAttClass {
			if attType == nil {
				sb.WriteString("<nil>")
			} else {
				sb.WriteString(attType.String())
			}
			sb.WriteByte('#')
		}
		sb.WriteString(key)
		sb.WriteByte('=')
		if value == nil {
			sb.WriteString("null")
		} else {
			fmt.Fprintf(&sb, "%v", value)
		}
	})
	return sb.String()
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
