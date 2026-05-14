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
	"sync"
)

// AttributeFactory is the Go port of org.apache.lucene.util.AttributeFactory.
//
// An AttributeFactory creates instances of [AttributeImpl] for a given
// [Attribute] interface type. Lucene's implementation relies on
// java.lang.Class as the lookup key; the Go port uses [reflect.Type]
// obtained via [reflect.TypeOf]((*FooAttribute)(nil)).Elem(), which
// gives a stable identity for an interface type.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/AttributeFactory.java
type AttributeFactory interface {
	// CreateAttributeInstance returns an [AttributeImpl] for the supplied
	// Attribute interface type, mirroring
	// AttributeFactory#createAttributeInstance(Class<? extends Attribute>).
	//
	// The returned impl must implement attType (verified by the caller
	// via reflect.Type.Implements on the impl's dynamic type).
	//
	// Implementations panic when the requested attribute type cannot be
	// resolved, matching the Lucene IllegalArgumentException semantics.
	CreateAttributeInstance(attType reflect.Type) AttributeImpl
}

// AttributeImplFactory is the constructor function used by
// [DefaultAttributeFactory]'s registry. It returns a freshly allocated
// AttributeImpl. The signature mirrors the no-arg constructor that
// Lucene's default factory invokes via MethodHandle.
type AttributeImplFactory func() AttributeImpl

// DefaultAttributeFactory is the package-default [AttributeFactory]. It
// is backed by a process-wide registry mapping an Attribute interface
// type to an [AttributeImplFactory]; the registry is populated by calls
// to [RegisterAttributeImpl]. This replaces Lucene's reflective lookup
// of the "<className>Impl" companion class, which has no portable
// equivalent in Go.
//
// Use [DefaultAttributeFactoryInstance] for the singleton instance
// equivalent to Lucene's {@code AttributeFactory.DEFAULT_ATTRIBUTE_FACTORY}.
type DefaultAttributeFactory struct{}

// CreateAttributeInstance implements [AttributeFactory] by consulting
// the registry populated by [RegisterAttributeImpl]. Panics with an
// "Cannot find implementing class for: <type>" message when no
// registration exists for attType, mirroring the Java
// IllegalArgumentException.
func (DefaultAttributeFactory) CreateAttributeInstance(attType reflect.Type) AttributeImpl {
	defaultAttributeRegistry.mu.RLock()
	factory, ok := defaultAttributeRegistry.entries[attType]
	defaultAttributeRegistry.mu.RUnlock()
	if !ok {
		panic(fmt.Sprintf("Cannot find implementing class for: %s", attType))
	}
	return factory()
}

// DefaultAttributeFactoryInstance is the canonical default factory,
// mirroring {@code AttributeFactory.DEFAULT_ATTRIBUTE_FACTORY}. It is a
// zero-cost value: callers may pass it by value without taking its
// address.
var DefaultAttributeFactoryInstance AttributeFactory = DefaultAttributeFactory{}

// defaultAttributeRegistry stores the AttributeImpl constructors keyed
// by the Attribute interface type they satisfy.
var defaultAttributeRegistry = struct {
	mu      sync.RWMutex
	entries map[reflect.Type]AttributeImplFactory
}{
	entries: make(map[reflect.Type]AttributeImplFactory),
}

// RegisterAttributeImpl associates an [AttributeImplFactory] with the
// given Attribute interface type so that [DefaultAttributeFactory] can
// resolve it. Registration is idempotent: re-registering the same type
// silently overwrites the previous entry, matching the "last writer
// wins" behaviour of Java's class-name reflection in the presence of
// duplicate class loaders.
//
// Callers must obtain attType via
// {@code reflect.TypeOf((*MyAttribute)(nil)).Elem()} so the runtime
// stores the interface type, not the *MyAttribute pointer type.
func RegisterAttributeImpl(attType reflect.Type, factory AttributeImplFactory) {
	if attType == nil {
		panic("AttributeFactory: attType must not be nil")
	}
	if attType.Kind() != reflect.Interface {
		panic(fmt.Sprintf("AttributeFactory: attType must be an interface type, got %s", attType.Kind()))
	}
	if factory == nil {
		panic("AttributeFactory: factory must not be nil")
	}
	defaultAttributeRegistry.mu.Lock()
	defaultAttributeRegistry.entries[attType] = factory
	defaultAttributeRegistry.mu.Unlock()
}

// StaticImplementationAttributeFactory is the Go port of Lucene's nested
// {@code AttributeFactory.StaticImplementationAttributeFactory<A>}. It
// returns instances of a fixed [AttributeImpl] for any Attribute type
// the impl satisfies, falling back to a delegate for the rest.
//
// Use [NewStaticImplementationAttributeFactory] to build one.
type StaticImplementationAttributeFactory struct {
	delegate AttributeFactory
	implType reflect.Type
	create   AttributeImplFactory
}

// NewStaticImplementationAttributeFactory returns an [AttributeFactory]
// that produces instances of the AttributeImpl returned by create for
// any Attribute interface satisfied by that impl, and delegates
// otherwise. Mirrors {@code AttributeFactory.getStaticImplementation}.
//
// The factory probes the dynamic type of a single create() invocation
// to learn which Attribute interfaces the impl implements; create is
// then re-invoked for every new instance.
func NewStaticImplementationAttributeFactory(delegate AttributeFactory, create AttributeImplFactory) *StaticImplementationAttributeFactory {
	if delegate == nil {
		panic("StaticImplementationAttributeFactory: delegate must not be nil")
	}
	if create == nil {
		panic("StaticImplementationAttributeFactory: create must not be nil")
	}
	// Probe once to discover the impl type. This mirrors the Java
	// constructor's use of {@code Class<A> clazz} to retain the impl
	// type for later assignability checks.
	probe := create()
	if probe == nil {
		panic("StaticImplementationAttributeFactory: create must not return nil")
	}
	return &StaticImplementationAttributeFactory{
		delegate: delegate,
		implType: reflect.TypeOf(probe),
		create:   create,
	}
}

// CreateAttributeInstance implements [AttributeFactory] by returning a
// fresh instance of the configured impl when it satisfies attType, and
// otherwise delegating to the wrapped factory.
func (s *StaticImplementationAttributeFactory) CreateAttributeInstance(attType reflect.Type) AttributeImpl {
	if s.implType.Implements(attType) {
		return s.create()
	}
	return s.delegate.CreateAttributeInstance(attType)
}

// Equals reports whether two [StaticImplementationAttributeFactory]
// values are equivalent, mirroring the Java override. Two factories are
// equal when they share the same delegate (compared with reflect.DeepEqual
// to accept pointer or value identity) and the same impl type.
func (s *StaticImplementationAttributeFactory) Equals(other any) bool {
	o, ok := other.(*StaticImplementationAttributeFactory)
	if !ok {
		return false
	}
	if s == o {
		return true
	}
	if s.implType != o.implType {
		return false
	}
	return reflect.DeepEqual(s.delegate, o.delegate)
}
