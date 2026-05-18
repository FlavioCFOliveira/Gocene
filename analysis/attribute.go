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

	"github.com/FlavioCFOliveira/Gocene/util"
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
//
// Deprecated: prefer [util.Attribute]. This marker is retained as a
// legacy compatibility shim while Sprint 54 migrates consumers to the
// util.AttributeSource API.
type Attribute interface {
	// Attribute is a marker interface - implementations provide their own methods
}

// AttributeImpl is a thin re-export of util.AttributeImpl (Sprint 54 Phase 2).
// The legacy 3-method Gocene SPI has been unified with the Lucene-faithful
// 5-method surface in util — impls must now satisfy End/ReflectWith/CloneAttribute
// in addition to Clear/CopyTo. The legacy Copy() method is preserved on impls
// for backwards compat and is not part of this interface.
type AttributeImpl = util.AttributeImpl

// AttributeReflector is a thin re-export of util.AttributeReflector (Sprint 54 Phase 2).
type AttributeReflector = util.AttributeReflector

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
//
// Deprecated: Sprint 54 Phase 2 made [AttributeImpl] an alias for
// [util.AttributeImpl], which already mandates ReflectWith on every
// impl. This interface is retained as a legacy compatibility shim and
// will be removed once consumer migration is complete.
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
//
// Deprecated: Sprint 54 Phase 2 made [AttributeImpl] an alias for
// [util.AttributeImpl], which already mandates End on every impl. This
// interface is retained as a legacy compatibility shim and will be
// removed once consumer migration is complete.
type AttributeEnder interface {
	// End resets this impl to its end-of-field state.
	End()
}

// ReflectWith dispatches reflector against impl. Sprint 54 Phase 2
// elevates ReflectWith to a mandatory method on [util.AttributeImpl]
// (the underlying type of the analysis.AttributeImpl alias), so the
// helper now always invokes impl.ReflectWith directly. The legacy
// AttributeReflectable opt-in check is retained for callers that pass
// in a value typed as the legacy interface; the result is the same.
//
// Deprecated: prefer calling impl.ReflectWith(reflector) directly. This
// helper is retained as a legacy compatibility shim.
func ReflectWith(impl AttributeImpl, reflector AttributeReflector) {
	impl.ReflectWith(reflector)
}

// End dispatches the end-of-field hook for impl. Sprint 54 Phase 2
// elevates End to a mandatory method on [util.AttributeImpl] (the
// underlying type of the analysis.AttributeImpl alias), so the helper
// now always invokes impl.End() directly. The Lucene default
// {@code end() -> clear()} is now implemented per-impl by each
// concrete End method (most delegate to Clear).
//
// Deprecated: prefer calling impl.End() directly. This helper is
// retained as a legacy compatibility shim.
func End(impl AttributeImpl) {
	impl.End()
}

// ReflectAsString is the Go port of
// {@code AttributeImpl#reflectAsString(boolean)}. It returns the
// current attribute values as a comma-separated string in one of two
// formats, byte-for-byte compatible with the Lucene reference:
//
//	prependAttClass=true  : "AttributeClass#key=value,AttributeClass#key=value"
//	prependAttClass=false : "key=value,key=value"
//
// nil values render as the literal "null", matching the Java reference.
//
// Deprecated: prefer [util.ReflectAsString]. This helper is retained as
// a legacy compatibility shim and currently mirrors the util variant's
// output for impls that emit at least one triple.
func ReflectAsString(impl AttributeImpl, prependAttClass bool) string {
	var sb strings.Builder
	impl.ReflectWith(func(attType reflect.Type, key string, value any) {
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
