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
// Java's ModuleResourceLoader delegates resource access to the JPMS
// Module system (Module.getResourceAsStream / Class.forName(Module,…)).
// Go has no analogue for the Java Module concept, so this port maps the
// notion onto an embed.FS (or any io/fs.FS) anchored at the "module
// root". Class lookup is replaced by a name-keyed factory registry,
// mirroring the same pattern adopted by ClasspathResourceLoader.
//
// The exported surface — OpenResource / FindFactory / NewInstance —
// matches the ResourceLoader contract verbatim. The constructor takes
// an fs.FS rather than a java.lang.reflect.Module.
// -----------------------------------------------------------------------------

package util

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// ModuleResourceLoader is the Go analogue of
// org.apache.lucene.util.ModuleResourceLoader. It serves resources from
// an fs.FS (intended to be an embed.FS at compile time) and SPI
// implementations from a name-keyed factory registry.
//
// Use NewModuleResourceLoader to construct one; the zero value is not
// usable.
type ModuleResourceLoader struct {
	module    fs.FS
	factories map[string]FactoryFunc
}

// NewModuleResourceLoader returns a ResourceLoader that reads resources
// from module (expected to be an embed.FS) and dispatches SPI lookups
// through factories. Either argument may be nil; the same fallback
// semantics as ClasspathResourceLoader apply.
func NewModuleResourceLoader(module fs.FS, factories map[string]FactoryFunc) *ModuleResourceLoader {
	return &ModuleResourceLoader{module: module, factories: factories}
}

// OpenResource opens the named resource. Resource paths must be
// absolute to the module root (no leading slash); fs.FS lookups reject
// paths that begin with "/" so we strip them defensively to match the
// Java behaviour of treating absolute paths as module-rooted.
func (l *ModuleResourceLoader) OpenResource(name string) (io.ReadCloser, error) {
	if l.module == nil {
		return nil, fmt.Errorf("%w: %s", ErrResourceNotFound, name)
	}
	cleaned := name
	for len(cleaned) > 0 && cleaned[0] == '/' {
		cleaned = cleaned[1:]
	}
	f, err := l.module.Open(cleaned)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrResourceNotFound, name)
		}
		return nil, fmt.Errorf("open resource %q: %w", name, err)
	}
	if rc, ok := f.(io.ReadCloser); ok {
		return rc, nil
	}
	return readCloser{Reader: f, Closer: f}, nil
}

// FindFactory looks up the registered factory for name.
func (l *ModuleResourceLoader) FindFactory(name string) (FactoryFunc, error) {
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
func (l *ModuleResourceLoader) NewInstance(name string) (any, error) {
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
