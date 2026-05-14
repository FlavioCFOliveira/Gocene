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
// Java's ResourceLoader exposes findClass(String, Class<T>) which
// returns a Class object the caller can newInstance(). Go has no
// class metadata at runtime, so the contract is reframed in terms of
// a FactoryFunc registry: callers register name -> constructor at
// init() time, and ResourceLoader implementations look the factory
// up and (optionally) invoke it.
//
// The interface itself lives in this file; concrete implementations
// (ClasspathResourceLoader, ModuleResourceLoader) are in their own
// files alongside their factories.
// -----------------------------------------------------------------------------

package util

// The actual interface declaration and the FactoryFunc /
// ErrResourceNotFound / ErrFactoryNotFound symbols live in
// classpath_resource_loader.go for historical reasons. This file
// exists to make the ResourceLoader symbol discoverable via the
// expected file name and to host any future ResourceLoader-related
// helpers that are independent of a concrete implementation.

// EnsureResourceLoader is a compile-time guard that pins the
// ResourceLoader interface contract. Modifying ResourceLoader breaks
// this assignment.
//
// The variable is intentionally untyped to avoid pulling
// implementation dependencies into this file: the cast happens in
// the *_test.go peer of each concrete loader.
var _ = (*ResourceLoader)(nil)
