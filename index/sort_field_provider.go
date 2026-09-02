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
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
//   - Java's [SortFieldProvider.ReadSortField] returns a `SortField` from
//     `org.apache.lucene.search`. Gocene cannot import `search` here
//     because `search` already depends transitively on `index` via the
//     `document` package, so the contract is expressed in terms of the
//     `SortFieldValue` opaque alias (`any`) at this boundary. Concrete
//     providers will type-assert back to `*search.SortField`. The wire
//     bytes — owned by each provider — remain byte-identical to Java.
//
//   - Java exposes the static helper `SortFieldProvider.write(SortField,
//     DataOutput)`, which internally calls
//     `sf.getIndexSorter().getProviderName()`. Gocene's [WriteSortField]
//     reaches the same outcome via the [SortFieldNamer] view of the
//     value, decoupling the helper from the still-evolving SortField
//     type.
//
//   - SPI discovery uses [util.NamedSPILoader] with explicit Register()
//     calls from init(), matching the strategy applied to Codec,
//     PostingsFormat, KnnVectorsFormat, etc.
// -----------------------------------------------------------------------------

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SortFieldValue is the opaque view of a `search.SortField` used at the
// SortFieldProvider boundary. It is an alias for `any` so the `index`
// package can express the SortFieldProvider contract without importing
// `search` (see file-level PORT NOTE for the cycle constraint).
//
// Concrete providers type-assert back to the concrete SortField type
// they understand. The Gocene built-in providers will assert to
// `*search.SortField`; downstream codecs that ship their own SortField
// flavours may assert to their own types.
type SortFieldValue = any

// ErrSortFieldNotSerializable is returned by [WriteSortField] when the
// supplied SortField cannot be associated with a registered provider
// (Java throws IllegalArgumentException with the same message shape).
var ErrSortFieldNotSerializable = errors.New("cannot serialize sort field")

// SortFieldProvider reads and writes a named SortField from a
// segment-info file. It is the Go port of
// org.apache.lucene.index.SortFieldProvider from Apache Lucene 10.4.0.
//
// The provider Name() is persisted on disk so the codec can dispatch
// back to the correct provider when reading the segment header. The
// name must satisfy [util.CheckServiceName] (ASCII alphanumeric,
// length < 128).
//
// Concrete providers register themselves at init() time via
// [RegisterSortFieldProvider]; that mirrors Java's META-INF/services
// SPI discovery on the Go side.
type SortFieldProvider interface {
	util.NamedSPI

	// ReadSortField reconstructs a SortField from the codec-supplied
	// input. Returns an error if the bytes do not correspond to a valid
	// payload for this provider.
	ReadSortField(in store.DataInput) (SortFieldValue, error)

	// WriteSortField writes sf to out using this provider's wire format.
	// The provider name itself is NOT written here; the static
	// [WriteSortField] helper writes the name and dispatches to the
	// matching provider's WriteSortField.
	WriteSortField(sf SortFieldValue, out store.DataOutput) error
}

// SortFieldNamer is satisfied by SortField-like values that know the
// canonical name of the provider responsible for serialising them. It
// exists to bridge the gap until search.SortField exposes a
// `GetIndexSorter()` method that yields the provider name directly.
//
// PORT NOTE: in Java, the helper consults
// `SortField.getIndexSorter().getProviderName()`. Gocene accepts any
// type that provides the name explicitly. The two formulations are
// equivalent at the wire level.
type SortFieldNamer interface {
	// ProviderName returns the registered SortFieldProvider name that
	// must be used to serialise this SortField, or the empty string if
	// the SortField is not serialisable.
	ProviderName() string
}

// sortFieldProviderLoader is the package-global SPI registry for
// SortFieldProvider. It mirrors the Java `Holder.LOADER` pattern: a
// single eager singleton populated by Register() calls from init().
var sortFieldProviderLoader = util.NewNamedSPILoader[SortFieldProvider]("SortFieldProvider")

// RegisterSortFieldProvider installs p into the global registry. It is
// safe to call from init() and from multiple goroutines. The first
// registration wins for any given name; subsequent calls with the same
// name are silently ignored (matching Java's ServiceLoader semantics).
//
// Returns an error if the provider name violates
// [util.CheckServiceName].
func RegisterSortFieldProvider(p SortFieldProvider) error {
	return sortFieldProviderLoader.Register(p)
}

// LookupSortFieldProvider returns the SortFieldProvider registered
// under name, or an error wrapping [util.ErrSPINotFound] if none is
// found. Equivalent to Java `SortFieldProvider.forName(name)`.
func LookupSortFieldProvider(name string) (SortFieldProvider, error) {
	return sortFieldProviderLoader.Lookup(name)
}

// AvailableSortFieldProviders returns the set of registered provider
// names in insertion order. Equivalent to Java
// `SortFieldProvider.availableSortFieldProviders()`.
func AvailableSortFieldProviders() []string {
	return sortFieldProviderLoader.AvailableServices()
}

// ReloadSortFieldProviders is a Go-side no-op kept for API parity with
// Java's `reloadSortFieldProviders(ClassLoader)`. In Go all SPI
// implementations are linked at build time and registered via init();
// there is no classpath to re-scan.
func ReloadSortFieldProviders() {
	sortFieldProviderLoader.Reload()
}

// WriteSortField is the Go port of Java's static
// `SortFieldProvider.write(SortField, DataOutput)`. It resolves the
// provider name via the [SortFieldNamer] view of sf, looks the
// provider up in the SPI registry, and delegates the actual byte
// emission to [SortFieldProvider.WriteSortField].
//
// Returns [ErrSortFieldNotSerializable] (wrapped with context) if sf
// is nil, does not implement [SortFieldNamer], or its ProviderName is
// empty — the failure modes Java collapses into a single
// IllegalArgumentException.
func WriteSortField(sf SortFieldValue, out store.DataOutput) error {
	if sf == nil {
		return fmt.Errorf("%w: <nil>", ErrSortFieldNotSerializable)
	}
	namer, ok := sf.(SortFieldNamer)
	if !ok {
		return fmt.Errorf("%w: %+v (no SortFieldNamer)", ErrSortFieldNotSerializable, sf)
	}
	name := namer.ProviderName()
	if name == "" {
		return fmt.Errorf("%w: %+v (empty provider name)", ErrSortFieldNotSerializable, sf)
	}
	provider, err := LookupSortFieldProvider(name)
	if err != nil {
		return fmt.Errorf("%w: %+v: %w", ErrSortFieldNotSerializable, sf, err)
	}
	return provider.WriteSortField(sf, out)
}
