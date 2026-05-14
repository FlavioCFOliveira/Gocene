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
// Lucene's ClasspathResourceLoader resolves resources via
// ClassLoader.getResourceAsStream and classes via Class.forName. Go has
// neither facility. The Go port keeps the ResourceLoader interface
// (OpenResource + FindFactory + NewInstance) but backs it with
// io/fs.FS — typically an embed.FS or os.DirFS at the caller's
// discretion.
//
// The lookup of *classes* maps to a name-keyed factory registry that
// callers populate at init time. This is consistent with how the rest
// of Gocene wires SPI (see NamedSPILoader, expected later in Sprint 1).
//
// The API surface here is deliberately minimal: enough for analyzers,
// codecs and other SPI consumers to compile, no more.
// -----------------------------------------------------------------------------

package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// FactoryFunc constructs an SPI implementation by name. Used by
// ResourceLoader to mirror Java's findClass / newInstance reflection.
type FactoryFunc func() (any, error)

// ResourceLoader abstracts access to resources (byte streams) and to
// SPI implementations identified by name. Mirrors
// org.apache.lucene.util.ResourceLoader, adapted to Go idioms.
type ResourceLoader interface {
	// OpenResource opens a named resource. Callers are responsible for
	// closing the returned reader.
	OpenResource(name string) (io.ReadCloser, error)

	// FindFactory locates a registered constructor for the SPI
	// implementation with the given canonical name. Returns
	// ErrFactoryNotFound when no factory has been registered.
	FindFactory(name string) (FactoryFunc, error)

	// NewInstance is a convenience wrapper that invokes FindFactory and
	// then the returned factory.
	NewInstance(name string) (any, error)
}

// ErrResourceNotFound is returned when a named resource cannot be
// located on the underlying file system.
var ErrResourceNotFound = errors.New("resource not found")

// ErrFactoryNotFound is returned by FindFactory / NewInstance when no
// factory is registered for the requested name.
var ErrFactoryNotFound = errors.New("factory not found")

// ClasspathResourceLoader is a ResourceLoader backed by an fs.FS and a
// name-keyed factory registry. Mirrors the surface of Java's
// ClasspathResourceLoader.
//
// The zero value is not usable; use NewClasspathResourceLoader.
type ClasspathResourceLoader struct {
	files     fs.FS
	factories map[string]FactoryFunc
}

// NewClasspathResourceLoader returns a ResourceLoader that reads
// resources from files and constructs SPI implementations from the
// given factory map. Both arguments may be nil; an absent fs.FS makes
// OpenResource always return ErrResourceNotFound, and an absent map
// makes FindFactory always return ErrFactoryNotFound.
func NewClasspathResourceLoader(files fs.FS, factories map[string]FactoryFunc) *ClasspathResourceLoader {
	return &ClasspathResourceLoader{files: files, factories: factories}
}

// OpenResource opens the named resource on the configured fs.FS.
// Returns ErrResourceNotFound (wrapped with the resource name) when
// the loader has no fs.FS or the entry is missing.
func (l *ClasspathResourceLoader) OpenResource(name string) (io.ReadCloser, error) {
	if l.files == nil {
		return nil, fmt.Errorf("%w: %s", ErrResourceNotFound, name)
	}
	f, err := l.files.Open(name)
	if err != nil {
		// fs.ErrNotExist is the canonical "missing" signal; surface it
		// behind ErrResourceNotFound while keeping the underlying error
		// reachable through errors.Is/As.
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrResourceNotFound, name)
		}
		return nil, fmt.Errorf("open resource %q: %w", name, err)
	}
	rc, ok := f.(io.ReadCloser)
	if !ok {
		// fs.File satisfies io.Reader but not necessarily io.Closer; in
		// practice every concrete implementation we expect (embed.FS,
		// os.DirFS) does. Defensively wrap.
		rc = readCloser{Reader: f, Closer: f}
	}
	return rc, nil
}

// FindFactory looks up the registered factory for name.
func (l *ClasspathResourceLoader) FindFactory(name string) (FactoryFunc, error) {
	if l.factories == nil {
		return nil, fmt.Errorf("%w: %s", ErrFactoryNotFound, name)
	}
	f, ok := l.factories[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrFactoryNotFound, name)
	}
	return f, nil
}

// NewInstance returns a freshly constructed instance of the SPI
// implementation registered as name.
func (l *ClasspathResourceLoader) NewInstance(name string) (any, error) {
	f, err := l.FindFactory(name)
	if err != nil {
		return nil, err
	}
	v, err := f()
	if err != nil {
		return nil, fmt.Errorf("instantiate %q: %w", name, err)
	}
	return v, nil
}

// readCloser bundles an io.Reader and io.Closer into an io.ReadCloser
// for fs.File implementations that satisfy both behaviorally but not
// nominally.
type readCloser struct {
	io.Reader
	io.Closer
}
