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

// Attribute is the Go port of org.apache.lucene.util.Attribute.
//
// It is the base marker interface for attributes. In Lucene's design, an
// [AttributeSource] is a heterogeneous registry keyed by the Attribute
// sub-interface (e.g. CharTermAttribute, OffsetAttribute, ...). Each
// sub-interface declares the public, consumer-facing operations exposed
// by that attribute kind; the backing data is held by an
// [AttributeImpl] that implements one or more such sub-interfaces.
//
// In Java the interface is intentionally empty so any sub-interface that
// "extends Attribute" is automatically an attribute. Go has no
// sub-interfacing, so the convention here is identical: concrete
// attribute interfaces embed Attribute. The interface is left empty
// (matching the Lucene reference byte-for-byte at the source level) and
// type identity is recovered via [reflect.Type] in [AttributeSource].
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/Attribute.java
type Attribute interface{}
