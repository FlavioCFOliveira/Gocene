// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Apache Lucene's ClassLoaderUtils inspects the runtime parent chain of
// java.lang.ClassLoader instances. Go has no class loaders at all: code
// is statically linked at build time and SPI is provided by explicit
// factory registries (see util/named_spi_loader.go, expected in a
// later batch).
//
// Rather than emulate parent/child reflection — which would be a
// no-op masquerading as a real check — this port exposes the minimal
// helper surface that Lucene-derived callers need: a registry-scope
// comparator. Two scopes are "related" when one is identical to, or
// nests within, the other (mirroring the "X is a parent of Y"
// predicate semantically, without the class-loader machinery).
//
// The intent is to keep this file's API small enough that callers
// porting from Java can replace `ClassLoaderUtils.isParentClassLoader`
// with `IsParentScope` and continue compiling, while remaining
// honest about the fact that Go SPI is registry-based.
// -----------------------------------------------------------------------------

package util

// Scope is a logical SPI registry scope. The Gocene equivalent of a
// java.lang.ClassLoader for the purposes of ServiceLoader-style lookup.
//
// Implementations are expected to be reference-stable; pointer identity
// is the natural way to express "the same scope" in Go.
type Scope interface {
	// Parent returns the enclosing scope, or nil if this is the root.
	Parent() Scope
}

// IsParentScope reports whether parent is an ancestor of, or identical
// to, child. Equivalent in spirit to
// {@code ClassLoaderUtils.isParentClassLoader} in Lucene.
//
// Returns false when either argument is nil. A nil parent has no
// position in any scope chain; a nil child has no chain to walk.
func IsParentScope(parent, child Scope) bool {
	if parent == nil || child == nil {
		return false
	}
	for s := child; s != nil; s = s.Parent() {
		if s == parent {
			return true
		}
	}
	return false
}
