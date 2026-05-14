// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package util

import (
	"fmt"
	"reflect"
	"strings"
)

// AttributeImpl is the Go port of the abstract class
// org.apache.lucene.util.AttributeImpl. Java models it as a concrete
// base class with abstract hooks; Go has no inheritance so the port
// expresses it as an interface that concrete implementations satisfy.
//
// Method names are PascalCased equivalents of the Java methods. The
// observable contract is identical:
//
//   - Clear() resets all backing fields to their default (matching
//     AttributeImpl#clear()).
//   - End() resets to end-of-field state. The default for most impls is
//     "same as Clear()", matching AttributeImpl#end() which simply calls
//     clear() unless overridden.
//   - ReflectWith(reflector) introspects the impl by calling reflector
//     for each (attClass, key, value) triple it exposes.
//   - CopyTo(target) deep-copies this impl's state onto target, which
//     must support the same Attribute interfaces.
//   - CloneAttribute() returns a deep clone. Lucene relies on
//     Object#clone(); Go has no equivalent so each impl is responsible
//     for returning a value-equivalent copy.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/AttributeImpl.java
type AttributeImpl interface {
	Attribute

	// Clear resets the impl to its default value.
	Clear()

	// End resets the impl to its end-of-field value. The default
	// implementation in Lucene delegates to Clear; concrete Go impls may
	// embed [BaseAttributeImpl] to get that default behaviour.
	End()

	// ReflectWith calls reflector for each (attClass, key, value) triple
	// this impl exposes. Implementations must emit the same set in the
	// same order on every invocation (Lucene contract).
	ReflectWith(reflector AttributeReflector)

	// CopyTo copies this impl's state onto target. The target must
	// support every Attribute interface this impl supports; otherwise
	// implementations should panic with an explanatory message.
	CopyTo(target AttributeImpl)

	// CloneAttribute returns a deep clone of this AttributeImpl,
	// mirroring AttributeImpl#clone(). The return type is AttributeImpl
	// (not the concrete type) to keep the interface uniform; callers
	// type-assert when they need the concrete type back.
	CloneAttribute() AttributeImpl
}

// ReflectAsString is the Go port of AttributeImpl#reflectAsString(boolean).
// It returns the current attribute values as a comma-separated string in
// one of two formats, byte-for-byte compatible with the Lucene reference:
//
//	prependAttClass=true  : "AttributeClass#key=value,AttributeClass#key=value"
//	prependAttClass=false : "key=value,key=value"
//
// The Lucene reference uses Class#getName(); the Go port reproduces the
// fully qualified Java-style name via [AttributeClassName] so the
// emitted strings remain stable when an attribute is registered under
// the canonical Java package, and falls back to the Go type name
// otherwise.
//
// nil values render as the literal "null", matching the Java reference.
func ReflectAsString(impl AttributeImpl, prependAttClass bool) string {
	var sb strings.Builder
	impl.ReflectWith(func(attType reflect.Type, key string, value any) {
		if sb.Len() > 0 {
			sb.WriteByte(',')
		}
		if prependAttClass {
			sb.WriteString(AttributeClassName(attType))
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

// BaseAttributeImpl is a zero-state struct intended for embedding by
// concrete [AttributeImpl] types. It provides the Java default of
// {@code end() { clear(); }} so embedding types only have to override
// when their end-of-field state differs from their default.
//
// The embedded type must still implement Clear, ReflectWith, CopyTo,
// CloneAttribute, and any Attribute sub-interfaces it represents;
// BaseAttributeImpl only contributes End.
type BaseAttributeImpl struct{}

// End is the default end-of-field hook: it asks the surrounding impl
// to clear itself. Concrete impls override End when their end-of-field
// state differs from the default Clear value.
//
// Note: this default cannot call Clear on the embedding type because Go
// embedding is not virtual. Concrete impls that rely on the default
// should override End in their own method set to call self.Clear().
func (BaseAttributeImpl) End() {}

// AttributeClassName returns the canonical name used for attType in
// reflective output. When attType has been registered with
// [RegisterAttributeClassName], the registered Java-style FQCN is
// returned (mirroring Class#getName() in Lucene's reference); otherwise
// the Go reflect.Type string is returned, which is sufficient for tests
// and for any consumer that does not need byte-for-byte parity with the
// Java reference output.
func AttributeClassName(attType reflect.Type) string {
	attributeNameRegistry.mu.RLock()
	name, ok := attributeNameRegistry.entries[attType]
	attributeNameRegistry.mu.RUnlock()
	if ok {
		return name
	}
	if attType == nil {
		return "<nil>"
	}
	return attType.String()
}

// RegisterAttributeClassName associates a fully qualified Java-style
// class name with the given Attribute interface type. This is required
// for byte-for-byte parity with Lucene's reflectAsString output, where
// the prefix is Class#getName() (e.g.
// "org.apache.lucene.analysis.tokenattributes.CharTermAttribute").
//
// Registration is idempotent: re-registering the same type overwrites
// the previous entry.
func RegisterAttributeClassName(attType reflect.Type, javaFQCN string) {
	if attType == nil {
		panic("AttributeImpl: attType must not be nil")
	}
	attributeNameRegistry.mu.Lock()
	attributeNameRegistry.entries[attType] = javaFQCN
	attributeNameRegistry.mu.Unlock()
}
