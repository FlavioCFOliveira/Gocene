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
	"reflect"
	"sync"
)

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

// attributeNameRegistry holds the optional FQCN names attached to
// Attribute interface types via [RegisterAttributeClassName]. It lives
// in this file because it is shared by [AttributeImpl] introspection
// helpers (which read it) and by registration call sites (which write
// it).
var attributeNameRegistry = struct {
	mu      sync.RWMutex
	entries map[reflect.Type]string
}{
	entries: make(map[reflect.Type]string),
}
