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
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

// ResourceLoaderAware is implemented by components that need to be
// initialised with a [ResourceLoader] (used for loading classes,
// files, etc.).
//
// Mirrors org.apache.lucene.util.ResourceLoaderAware.
type ResourceLoaderAware interface {
	// Inform initialises the receiver with the provided ResourceLoader.
	// Implementations should return any error encountered while loading
	// secondary resources from the loader.
	Inform(loader ResourceLoader) error
}

// InformAll invokes Inform on every component in components, stopping
// (and returning) at the first non-nil error. This mirrors the
// Lucene-side pattern of iterating a list of ResourceLoaderAware
// instances during analyzer / codec wiring.
func InformAll(loader ResourceLoader, components ...ResourceLoaderAware) error {
	for _, c := range components {
		if c == nil {
			continue
		}
		if err := c.Inform(loader); err != nil {
			return err
		}
	}
	return nil
}
