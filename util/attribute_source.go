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

// AttributeSource is the Go port of org.apache.lucene.util.AttributeSource.
//
// It is a heterogeneous registry of [AttributeImpl] instances keyed by
// the [Attribute] interface type the consumer uses to look them up.
// There can only be a single instance of each Attribute interface in
// the same AttributeSource. AddAttribute is the canonical entry point:
// it returns the existing impl if one is registered, otherwise creates
// a new one through the configured [AttributeFactory].
//
// The Java implementation keeps two LinkedHashMaps in sync; the Go
// port stores both maps plus the insertion-order slices and the cached
// state list inside a shared [attributeSourceState] struct so that
// sources constructed via [NewAttributeSourceFrom] observe each
// other's mutations (LUCENE-3042 contract).
//
// All methods are safe to call from a single goroutine; the type is
// not safe for concurrent mutation, mirroring Lucene's non-thread-safe
// AttributeSource contract.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/AttributeSource.java
type AttributeSource struct {
	// state holds the mutable internal state. It is a pointer to a
	// shared struct so that AttributeSources created via
	// [NewAttributeSourceFrom] observe one another's mutations exactly
	// as Lucene's {@code AttributeSource(AttributeSource)} sharing
	// constructor does (LUCENE-3042).
	state   *attributeSourceState
	factory AttributeFactory
}

// attributeSourceState collects the mutable maps, order slices and
// cached state-list of an [AttributeSource]. It is shared by reference
// between sources that wrap the same input.
type attributeSourceState struct {
	// attributes maps Attribute interface type -> AttributeImpl. Note:
	// in Lucene the keys are the Attribute interface (Class<? extends
	// Attribute>); here they are reflect.Type values obtained from
	// {@code reflect.TypeOf((*FooAttribute)(nil)).Elem()}.
	attributes map[reflect.Type]AttributeImpl
	// attributeImpls maps AttributeImpl dynamic type -> AttributeImpl.
	// Two interfaces sharing the same impl appear once here.
	attributeImpls map[reflect.Type]AttributeImpl
	// order preserves the insertion order of attribute interface types,
	// reproducing LinkedHashMap iteration semantics.
	order []reflect.Type
	// implOrder preserves the insertion order of impl types.
	implOrder []reflect.Type
	// currentState is a lazily computed linked list snapshot used by
	// captureState/restoreState/clearAttributes/endAttributes/reflectWith.
	// It is set to nil whenever the impl set changes.
	currentState *AttributeState
}

// AttributeState mirrors the nested class
// org.apache.lucene.util.AttributeSource.State. It is a singly linked
// list of [AttributeImpl] snapshots used by [AttributeSource.CaptureState]
// / [AttributeSource.RestoreState].
type AttributeState struct {
	Attribute AttributeImpl
	Next      *AttributeState
}

// Clone returns a deep copy of the state list, deep-cloning each
// AttributeImpl via [AttributeImpl.CloneAttribute].
func (s *AttributeState) Clone() *AttributeState {
	if s == nil {
		return nil
	}
	clone := &AttributeState{Attribute: s.Attribute.CloneAttribute()}
	if s.Next != nil {
		clone.Next = s.Next.Clone()
	}
	return clone
}

// NewAttributeSource returns an [AttributeSource] using
// [DefaultAttributeFactoryInstance], mirroring the Java no-arg
// constructor.
func NewAttributeSource() *AttributeSource {
	return NewAttributeSourceWithFactory(DefaultAttributeFactoryInstance)
}

// NewAttributeSourceWithFactory returns an [AttributeSource] backed by
// the supplied factory, mirroring {@code AttributeSource(AttributeFactory)}.
// Panics if factory is nil.
func NewAttributeSourceWithFactory(factory AttributeFactory) *AttributeSource {
	if factory == nil {
		panic("AttributeFactory must not be nil")
	}
	return &AttributeSource{
		state: &attributeSourceState{
			attributes:     make(map[reflect.Type]AttributeImpl),
			attributeImpls: make(map[reflect.Type]AttributeImpl),
		},
		factory: factory,
	}
}

// NewAttributeSourceFrom returns a shallow view of input: the
// underlying state pointer is shared, mirroring the Java
// {@code AttributeSource(AttributeSource input)} constructor used by
// LUCENE-3042 to wire chained streams. Panics if input is nil.
func NewAttributeSourceFrom(input *AttributeSource) *AttributeSource {
	if input == nil {
		panic("input AttributeSource must not be nil")
	}
	return &AttributeSource{
		state:   input.state,
		factory: input.factory,
	}
}

// GetAttributeFactory returns the factory the source uses to create new
// AttributeImpl instances.
func (a *AttributeSource) GetAttributeFactory() AttributeFactory {
	return a.factory
}

// GetAttributeClassesIterator returns the Attribute interface types
// registered with this source, in insertion order. Mirrors
// {@code getAttributeClassesIterator()}.
//
// The returned slice is a fresh copy and may be mutated by the caller
// without affecting the source.
func (a *AttributeSource) GetAttributeClassesIterator() []reflect.Type {
	out := make([]reflect.Type, len(a.state.order))
	copy(out, a.state.order)
	return out
}

// GetAttributeImplsIterator returns the unique AttributeImpl instances
// registered with this source, in insertion order. This may have fewer
// entries than [AttributeSource.GetAttributeClassesIterator] when a
// single impl satisfies multiple Attribute interfaces.
func (a *AttributeSource) GetAttributeImplsIterator() []AttributeImpl {
	s := a.state
	out := make([]AttributeImpl, 0, len(s.implOrder))
	for _, t := range s.implOrder {
		out = append(out, s.attributeImpls[t])
	}
	return out
}

// AttributeInterfaceProvider may be implemented by [AttributeImpl]
// types to declare exactly which Attribute interface types they
// satisfy. This is the Go equivalent of Lucene's reflective
// {@code clazz.getInterfaces()} traversal: Go reflection cannot
// enumerate interface satisfaction without a candidate set, so impls
// either implement this optional method, register their interfaces via
// [RegisterAttributeImpl], or both.
type AttributeInterfaceProvider interface {
	// AttributeInterfaces returns the Attribute interface types this
	// impl satisfies, in declaration order.
	AttributeInterfaces() []reflect.Type
}

// AddAttributeImpl registers att with this source for every Attribute
// interface type it satisfies. Mirrors
// {@code AttributeSource#addAttributeImpl(AttributeImpl)}. If att's
// dynamic type is already registered the call is a no-op.
//
// The set of Attribute interfaces is discovered via, in order:
//  1. The AttributeInterfaceProvider optional method on att, if present.
//  2. The defaultAttributeRegistry: every registered Attribute interface
//     type for which att's dynamic type satisfies reflect.Type.Implements
//     is included.
//
// If neither path yields any interface, the impl is silently ignored,
// matching Lucene's behaviour: the impl is benign because it cannot be
// reached via any AddAttribute lookup.
func (a *AttributeSource) AddAttributeImpl(att AttributeImpl) {
	if att == nil {
		panic("AttributeImpl must not be nil")
	}
	s := a.state
	implType := reflect.TypeOf(att)
	if _, exists := s.attributeImpls[implType]; exists {
		return
	}

	interfaces := discoverAttributeInterfaces(att, implType)

	for _, attType := range interfaces {
		if _, exists := s.attributes[attType]; exists {
			continue
		}
		s.currentState = nil
		s.attributes[attType] = att
		s.order = append(s.order, attType)
		if _, implExists := s.attributeImpls[implType]; !implExists {
			s.attributeImpls[implType] = att
			s.implOrder = append(s.implOrder, implType)
		}
	}
}

// AddAttribute is the canonical entry point: it returns the existing
// AttributeImpl for attType, or creates a new one via the configured
// AttributeFactory and registers it. Mirrors
// {@code <T extends Attribute> T addAttribute(Class<T>)}.
//
// Panics with an "addAttribute() only accepts an interface that extends
// Attribute" message when attType is not an interface type. Callers
// obtain attType via {@code reflect.TypeOf((*Foo)(nil)).Elem()}.
//
// The return type is the generic AttributeImpl; callers type-assert to
// the concrete Attribute sub-interface (Java's reflective {@code
// attClass.cast(attImpl)} has no Go analogue because attType is itself
// the target interface).
func (a *AttributeSource) AddAttribute(attType reflect.Type) AttributeImpl {
	if existing, ok := a.state.attributes[attType]; ok {
		return existing
	}
	if attType == nil || attType.Kind() != reflect.Interface {
		panic(fmt.Sprintf("addAttribute() only accepts an interface that extends Attribute, but %v does not fulfil this contract.", attType))
	}
	impl := a.factory.CreateAttributeInstance(attType)
	a.AddAttributeImpl(impl)
	return impl
}

// HasAttributes reports whether this source contains any attributes.
func (a *AttributeSource) HasAttributes() bool {
	return len(a.state.attributes) > 0
}

// HasAttribute reports whether this source contains an AttributeImpl
// registered under attType.
func (a *AttributeSource) HasAttribute(attType reflect.Type) bool {
	_, ok := a.state.attributes[attType]
	return ok
}

// GetAttribute returns the AttributeImpl registered under attType, or
// nil if none is registered. Mirrors {@code getAttribute(Class<T>)}.
func (a *AttributeSource) GetAttribute(attType reflect.Type) AttributeImpl {
	return a.state.attributes[attType]
}

// RemoveAllAttributes removes every attribute and impl from this
// source. After this call HasAttributes returns false.
func (a *AttributeSource) RemoveAllAttributes() {
	s := a.state
	for k := range s.attributes {
		delete(s.attributes, k)
	}
	for k := range s.attributeImpls {
		delete(s.attributeImpls, k)
	}
	s.order = s.order[:0]
	s.implOrder = s.implOrder[:0]
	s.currentState = nil
}

// ClearAttributes resets every registered AttributeImpl to its default
// value by calling Clear on each. Mirrors {@code clearAttributes()}.
func (a *AttributeSource) ClearAttributes() {
	for state := a.getCurrentState(); state != nil; state = state.Next {
		state.Attribute.Clear()
	}
}

// EndAttributes resets every registered AttributeImpl to its
// end-of-field state by calling End on each. Mirrors
// {@code endAttributes()}.
func (a *AttributeSource) EndAttributes() {
	for state := a.getCurrentState(); state != nil; state = state.Next {
		state.Attribute.End()
	}
}

// CaptureState returns a deep snapshot of all registered AttributeImpl
// instances, suitable for later [AttributeSource.RestoreState]. Returns
// nil when the source has no attributes.
func (a *AttributeSource) CaptureState() *AttributeState {
	s := a.getCurrentState()
	if s == nil {
		return nil
	}
	return s.Clone()
}

// RestoreState copies the impls from state onto this source's matching
// impls (matched by impl dynamic type). Panics with an "State contains
// AttributeImpl of type X that is not in this AttributeSource" message
// when state contains an impl type that this source does not host.
//
// A nil state is a no-op, mirroring Lucene's `if (state == null) return;`.
func (a *AttributeSource) RestoreState(state *AttributeState) {
	if state == nil {
		return
	}
	for st := state; st != nil; st = st.Next {
		implType := reflect.TypeOf(st.Attribute)
		target, ok := a.state.attributeImpls[implType]
		if !ok {
			panic(fmt.Sprintf("State contains AttributeImpl of type %s that is not in in this AttributeSource", implType))
		}
		st.Attribute.CopyTo(target)
	}
}

// ReflectWith iterates every registered impl in insertion order and
// asks each one to reflect its state through reflector. Mirrors
// {@code reflectWith(AttributeReflector)}.
func (a *AttributeSource) ReflectWith(reflector AttributeReflector) {
	for state := a.getCurrentState(); state != nil; state = state.Next {
		state.Attribute.ReflectWith(reflector)
	}
}

// ReflectAsString returns the current attribute values as a
// comma-separated string. Mirrors {@code reflectAsString(boolean)};
// see [ReflectAsString] for the exact format.
func (a *AttributeSource) ReflectAsString(prependAttClass bool) string {
	var sb strings.Builder
	a.ReflectWith(func(attType reflect.Type, key string, value any) {
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

// CloneAttributes returns a new AttributeSource configured with the
// same AttributeFactory and containing deep clones of every impl in
// this source, registered under the same Attribute interface types.
// Mirrors {@code cloneAttributes()}.
func (a *AttributeSource) CloneAttributes() *AttributeSource {
	clone := NewAttributeSourceWithFactory(a.factory)
	src := a.state
	if !a.HasAttributes() {
		return clone
	}
	dst := clone.state
	oldImplToNew := make(map[reflect.Type]AttributeImpl, len(src.implOrder))
	for _, implType := range src.implOrder {
		newImpl := src.attributeImpls[implType].CloneAttribute()
		oldImplToNew[implType] = newImpl
		dst.attributeImpls[implType] = newImpl
		dst.implOrder = append(dst.implOrder, implType)
	}
	for _, attType := range src.order {
		oldImpl := src.attributes[attType]
		dst.attributes[attType] = oldImplToNew[reflect.TypeOf(oldImpl)]
		dst.order = append(dst.order, attType)
	}
	return clone
}

// CopyTo copies the values of every impl in this source onto the
// matching impl in target (matched by impl dynamic type). Mirrors
// {@code copyTo(AttributeSource)}. Panics when target lacks an impl
// type present in this source.
func (a *AttributeSource) CopyTo(target *AttributeSource) {
	for state := a.getCurrentState(); state != nil; state = state.Next {
		implType := reflect.TypeOf(state.Attribute)
		targetImpl, ok := target.state.attributeImpls[implType]
		if !ok {
			panic(fmt.Sprintf("This AttributeSource contains AttributeImpl of type %s that is not in the target", implType))
		}
		state.Attribute.CopyTo(targetImpl)
	}
}

// HashCode returns a deterministic hash of this source's impls in
// insertion order. Mirrors Lucene's {@code hashCode()} which uses
// {@code 31 * acc + impl.hashCode()}, but does not assume any specific
// hashCode method on AttributeImpl: instead, it hashes a digest of
// each impl's reflected state through ReflectAsString, which provides
// equivalent equivalence-class semantics.
func (a *AttributeSource) HashCode() int {
	code := 0
	for state := a.getCurrentState(); state != nil; state = state.Next {
		code = code*31 + attributeImplHash(state.Attribute)
	}
	return code
}

// Equals reports whether two AttributeSources hold the same impl types
// in the same order with the same reflected state. Mirrors
// {@code equals(Object)}.
func (a *AttributeSource) Equals(other any) bool {
	if a == other {
		return true
	}
	o, ok := other.(*AttributeSource)
	if !ok || o == nil {
		return false
	}
	if !a.HasAttributes() {
		return !o.HasAttributes()
	}
	if !o.HasAttributes() {
		return false
	}
	if len(a.state.attributeImpls) != len(o.state.attributeImpls) {
		return false
	}
	thisState := a.getCurrentState()
	otherState := o.getCurrentState()
	for thisState != nil && otherState != nil {
		thisType := reflect.TypeOf(thisState.Attribute)
		otherType := reflect.TypeOf(otherState.Attribute)
		if thisType != otherType {
			return false
		}
		if attributeImplDigest(thisState.Attribute) != attributeImplDigest(otherState.Attribute) {
			return false
		}
		thisState = thisState.Next
		otherState = otherState.Next
	}
	return thisState == nil && otherState == nil
}

// String returns a deterministic textual description of this source,
// mirroring {@code toString()} as
// "AttributeSource@<addr> <reflectAsString(false)>". The address part
// is taken from the Go runtime pointer to provide rough uniqueness;
// callers should not rely on its exact value for parsing.
func (a *AttributeSource) String() string {
	return fmt.Sprintf("AttributeSource@%p %s", a, a.ReflectAsString(false))
}

// getCurrentState lazily computes the linked-list of AttributeImpl
// snapshots used by Capture/Restore/ReflectWith. The result is cached
// in a.state.currentState and invalidated whenever the impl set changes.
func (a *AttributeSource) getCurrentState() *AttributeState {
	s := a.state
	if s.currentState != nil || !a.HasAttributes() {
		return s.currentState
	}
	first := &AttributeState{Attribute: s.attributeImpls[s.implOrder[0]]}
	cur := first
	for i := 1; i < len(s.implOrder); i++ {
		cur.Next = &AttributeState{Attribute: s.attributeImpls[s.implOrder[i]]}
		cur = cur.Next
	}
	s.currentState = first
	return first
}

// discoverAttributeInterfaces walks the registry + optional
// AttributeInterfaceProvider to enumerate the Attribute interface types
// satisfied by att, mirroring Lucene's reflective
// {@code clazz.getInterfaces()} traversal up the class hierarchy.
func discoverAttributeInterfaces(att AttributeImpl, implType reflect.Type) []reflect.Type {
	if provider, ok := att.(AttributeInterfaceProvider); ok {
		return provider.AttributeInterfaces()
	}
	defaultAttributeRegistry.mu.RLock()
	defer defaultAttributeRegistry.mu.RUnlock()
	var out []reflect.Type
	for attType := range defaultAttributeRegistry.entries {
		if implType.Implements(attType) {
			out = append(out, attType)
		}
	}
	return out
}

// attributeImplHash returns a stable hash for impl, derived from a
// digest of its reflected state plus its concrete type identity.
func attributeImplHash(impl AttributeImpl) int {
	digest := attributeImplDigest(impl)
	h := 0
	for i := 0; i < len(digest); i++ {
		h = h*31 + int(digest[i])
	}
	return h
}

// attributeImplDigest returns a string fingerprint of impl by invoking
// ReflectWith and concatenating its emitted triples. Used by HashCode
// and Equals as a portable substitute for Java's Object#equals/hashCode
// on AttributeImpl.
func attributeImplDigest(impl AttributeImpl) string {
	var sb strings.Builder
	implType := reflect.TypeOf(impl)
	sb.WriteString(implType.String())
	sb.WriteByte('{')
	impl.ReflectWith(func(attType reflect.Type, key string, value any) {
		sb.WriteString(attType.String())
		sb.WriteByte('#')
		sb.WriteString(key)
		sb.WriteByte('=')
		if value == nil {
			sb.WriteString("null")
		} else {
			fmt.Fprintf(&sb, "%v", value)
		}
		sb.WriteByte(';')
	})
	sb.WriteByte('}')
	return sb.String()
}
