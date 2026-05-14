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
// Lucene's CommandLineUtil is a thin reflective wrapper around
// FSDirectory subclasses, instantiating one of them by class name
// (Class.forName + asSubclass) and invoking a (Path, LockFactory)
// constructor by reflection. Go has neither Class.forName nor a
// universal asSubclass; both are statically resolved at build time.
//
// Importing the store package from util would invert the dependency
// graph (store depends on util, not the other way around). To stay
// dependency-clean, this port exposes:
//
//   - AdjustDirectoryClassName: the name-normalisation rule from the
//     Java private helper (prepend the canonical store package when the
//     argument is unqualified). Kept as a string utility so the rule is
//     auditable and reusable.
//   - DirectoryRegistry: a generic registry that stores factory
//     functions keyed by name and resolves them at runtime. The store
//     package (or any caller) populates this registry at init() time.
//
// Together these reproduce the *intent* (look up an FSDirectory by
// name, then construct it) without dragging reflection into Go.
// -----------------------------------------------------------------------------

package util

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// DirectoryPackagePrefix is the canonical Gocene equivalent of Java's
// org.apache.lucene.store package. Used by AdjustDirectoryClassName to
// fully-qualify bare names. The string form keeps util free of any
// import dependency on store.
const DirectoryPackagePrefix = "github.com/FlavioCFOliveira/Gocene/store."

// ErrDirectoryNameEmpty is returned by AdjustDirectoryClassName when
// the input is empty or whitespace-only.
var ErrDirectoryNameEmpty = errors.New("directory implementation name must not be empty")

// ErrDirectoryNotRegistered is returned by DirectoryRegistry.Lookup
// when no factory was registered for the requested name.
var ErrDirectoryNotRegistered = errors.New("directory implementation not registered")

// AdjustDirectoryClassName mirrors the private helper from Lucene's
// CommandLineUtil: an empty/whitespace-only name returns
// ErrDirectoryNameEmpty; an unqualified name (no '.') is prepended
// with DirectoryPackagePrefix; any other name is returned verbatim.
//
// The function is pure and does not consult the registry — it is
// purely a string-shape utility.
func AdjustDirectoryClassName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrDirectoryNameEmpty
	}
	if !strings.Contains(trimmed, ".") {
		return DirectoryPackagePrefix + trimmed, nil
	}
	return trimmed, nil
}

// DirectoryFactory constructs a Directory implementation given a path
// and an opaque lock-factory value. The return type is `any` so this
// utility can live in util without importing store. Callers cast to
// the concrete type they expect.
type DirectoryFactory func(path string, lockFactory any) (any, error)

// DirectoryRegistry is a name-keyed table of DirectoryFactory entries.
// Concrete Directory implementations call Register at init() time;
// command-line front-ends (or anything that resolves a directory by
// name) call Lookup.
//
// DirectoryRegistry is safe for concurrent use.
type DirectoryRegistry struct {
	mu        sync.RWMutex
	factories map[string]DirectoryFactory
}

// NewDirectoryRegistry returns an empty registry.
func NewDirectoryRegistry() *DirectoryRegistry {
	return &DirectoryRegistry{factories: make(map[string]DirectoryFactory)}
}

// Register inserts the factory under the given canonical name. The
// name is normalised through AdjustDirectoryClassName before storage,
// so callers may register either qualified or bare names and lookup
// will agree. Returns the normalised name actually used.
func (r *DirectoryRegistry) Register(name string, factory DirectoryFactory) (string, error) {
	if factory == nil {
		return "", errors.New("directory factory must not be nil")
	}
	key, err := AdjustDirectoryClassName(name)
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	r.factories[key] = factory
	r.mu.Unlock()
	return key, nil
}

// Lookup returns the registered factory for the given name. Returns
// ErrDirectoryNotRegistered when no entry exists. The name is
// normalised through AdjustDirectoryClassName before lookup.
func (r *DirectoryRegistry) Lookup(name string) (DirectoryFactory, error) {
	key, err := AdjustDirectoryClassName(name)
	if err != nil {
		return nil, err
	}
	r.mu.RLock()
	f, ok := r.factories[key]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDirectoryNotRegistered, key)
	}
	return f, nil
}

// NewDirectoryFromName resolves and invokes the factory for name with
// the supplied arguments. Equivalent in spirit to Java's
// CommandLineUtil.newFSDirectory(String, Path, LockFactory).
func (r *DirectoryRegistry) NewDirectoryFromName(name, path string, lockFactory any) (any, error) {
	f, err := r.Lookup(name)
	if err != nil {
		return nil, err
	}
	return f(path, lockFactory)
}
