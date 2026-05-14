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
// Java's NamedSPILoader discovers SPI implementations via the JDK
// ServiceLoader, which reads META-INF/services/<interface>. Go has no
// equivalent on the binary level: every implementation must be linked
// in at build time. We expose Register() so packages can call it from
// their init() functions, mirroring what other Go ecosystems do for
// codecs (image/png), databases (database/sql/Register), etc.
//
// The exported API matches the Java surface as closely as possible:
//   - Register(name, instance) is the Go-only counterpart of
//     ServiceLoader discovery (callers invoke it at init-time).
//   - Reload(...) becomes a no-op that exists to keep the contract
//     surface stable for consumers that might be ported later. It is
//     safe to call concurrently but does nothing useful in Go.
//   - Lookup, AvailableServices, Iterator behave exactly as the Java
//     methods. CheckServiceName enforces the same constraints.
//
// The type is generic over a NamedSPI interface, so each Lucene SPI
// (Codec, PostingsFormat, …) gets its own NamedSPILoader instance.
// -----------------------------------------------------------------------------

package util

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"sync"
	"sync/atomic"
)

// NamedSPI is the minimal contract any pluggable Lucene service must
// satisfy: a name accessor whose return value is unique within the SPI
// type and respects [CheckServiceName] constraints (ASCII alphanumeric,
// less than 128 characters).
type NamedSPI interface {
	Name() string
}

// ErrSPINotFound is returned by [NamedSPILoader.Lookup] when no
// registered implementation matches the requested name.
var ErrSPINotFound = errors.New("named SPI not found")

// ErrInvalidSPIName is returned by [CheckServiceName] when a name
// violates the contract (too long, or non ASCII-alphanumeric).
var ErrInvalidSPIName = errors.New("invalid SPI service name")

// CheckServiceName mirrors Java NamedSPILoader.checkServiceName. It
// returns an error wrapping [ErrInvalidSPIName] for names longer than
// 127 characters or containing anything other than ASCII letters and
// digits. The empty string is accepted, matching the Java behaviour
// of iterating zero characters in the validation loop.
func CheckServiceName(name string) error {
	if len(name) >= 128 {
		return fmt.Errorf("%w: %q is too long (must be < 128 chars)", ErrInvalidSPIName, name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if !isASCIILetterOrDigit(c) {
			return fmt.Errorf("%w: %q must be simple ASCII alphanumeric", ErrInvalidSPIName, name)
		}
	}
	return nil
}

func isASCIILetterOrDigit(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')
}

// NamedSPILoader is the Go analogue of org.apache.lucene.util.NamedSPILoader.
// It is generic over a [NamedSPI]-shaped interface and acts as a
// concurrent name->instance registry.
//
// Use [NewNamedSPILoader] to construct one. The loader is safe for
// concurrent use; readers (Lookup, AvailableServices, Iterator) are
// lock-free via atomic.Pointer snapshots, writers (Register) serialise
// on a mutex.
//
// Insertion order is preserved across AvailableServices and Iterator
// so the resulting Set matches the Java LinkedHashMap iteration order.
type NamedSPILoader[S NamedSPI] struct {
	mu       sync.Mutex
	services atomic.Pointer[spiState[S]]
	typeName string
}

// spiState is the immutable snapshot held by [NamedSPILoader]. Each
// Register builds a fresh state and swaps it in atomically so readers
// never observe a partially populated map.
type spiState[S NamedSPI] struct {
	byName map[string]S
	order  []string // insertion order, parallel keys of byName
}

// NewNamedSPILoader returns an empty loader for SPI type S. typeName
// is used in error messages and may be the friendly SPI name (for
// example "Codec" or "PostingsFormat").
func NewNamedSPILoader[S NamedSPI](typeName string) *NamedSPILoader[S] {
	l := &NamedSPILoader[S]{typeName: typeName}
	l.services.Store(&spiState[S]{byName: map[string]S{}})
	return l
}

// Register inserts svc into the loader if no implementation with the
// same name has been registered yet. Mirrors the "only add the first
// one for each name" semantics of the Java ServiceLoader-driven
// reload(). Returns nil if the registration succeeded, an error if
// the name is invalid (wrapping [ErrInvalidSPIName]).
//
// Register is safe to call concurrently and from init().
func (l *NamedSPILoader[S]) Register(svc S) error {
	name := svc.Name()
	if err := CheckServiceName(name); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	cur := l.services.Load()
	if _, ok := cur.byName[name]; ok {
		// Lucene silently keeps the first one; mirror that behaviour.
		return nil
	}

	next := &spiState[S]{
		byName: make(map[string]S, len(cur.byName)+1),
		order:  make([]string, 0, len(cur.order)+1),
	}
	maps.Copy(next.byName, cur.byName)
	next.order = append(next.order, cur.order...)
	next.byName[name] = svc
	next.order = append(next.order, name)
	l.services.Store(next)
	return nil
}

// Reload is a Go-side no-op kept for API parity with Java's
// reload(ClassLoader). In Go all SPI implementations are linked at
// build time and registered via init(); there is no classpath to
// re-scan.
func (l *NamedSPILoader[S]) Reload() {
	// Intentionally empty — see file-level PORT NOTE.
}

// Lookup returns the registered implementation for name, or an error
// wrapping [ErrSPINotFound] including the list of available services
// in the same shape Java uses ("does not exist. … available names: …").
func (l *NamedSPILoader[S]) Lookup(name string) (S, error) {
	cur := l.services.Load()
	if svc, ok := cur.byName[name]; ok {
		return svc, nil
	}
	var zero S
	return zero, fmt.Errorf(
		"%w: %s SPI with name %q does not exist. Available names: %v",
		ErrSPINotFound, l.typeName, name, cur.order,
	)
}

// AvailableServices returns the set of registered service names in
// the order they were first registered. The slice is a fresh copy so
// callers may freely mutate it.
func (l *NamedSPILoader[S]) AvailableServices() []string {
	cur := l.services.Load()
	out := make([]string, len(cur.order))
	copy(out, cur.order)
	return out
}

// Iterator returns the registered services in insertion order. The
// returned slice is a fresh copy to keep snapshot semantics.
func (l *NamedSPILoader[S]) Iterator() []S {
	cur := l.services.Load()
	out := make([]S, 0, len(cur.order))
	for _, n := range cur.order {
		out = append(out, cur.byName[n])
	}
	return out
}

// SortedAvailableServices returns AvailableServices() in lexicographic
// order. Provided for callers that need a deterministic listing
// independent of registration order (for example diagnostics).
func (l *NamedSPILoader[S]) SortedAvailableServices() []string {
	out := l.AvailableServices()
	sort.Strings(out)
	return out
}

// Size returns the number of registered services.
func (l *NamedSPILoader[S]) Size() int {
	return len(l.services.Load().order)
}
