// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

type ignoredSampleType struct{}
type notIgnoredSampleType struct{}

func TestIgnoreRandomChains(t *testing.T) {
	// Always start clean: another test may have left entries.
	defer UnregisterIgnoredForRandomChains(ignoredSampleType{})

	t.Run("unregistered reports false", func(t *testing.T) {
		if IsIgnoredForRandomChains(notIgnoredSampleType{}) {
			t.Fatalf("unregistered type must not be ignored")
		}
		if _, ok := IgnoredForRandomChainsReason(notIgnoredSampleType{}); ok {
			t.Fatalf("unregistered type must have no reason")
		}
	})

	t.Run("nil sample is safe", func(t *testing.T) {
		RegisterIgnoredForRandomChains(nil, "x") // no-op
		if IsIgnoredForRandomChains(nil) {
			t.Fatalf("nil sample must report false")
		}
		if _, ok := IgnoredForRandomChainsReason(nil); ok {
			t.Fatalf("nil sample must have no reason")
		}
		UnregisterIgnoredForRandomChains(nil) // no-op
	})

	t.Run("register and lookup by value", func(t *testing.T) {
		RegisterIgnoredForRandomChains(ignoredSampleType{}, "non-deterministic")
		if !IsIgnoredForRandomChains(ignoredSampleType{}) {
			t.Fatalf("registered type must be ignored")
		}
		reason, ok := IgnoredForRandomChainsReason(ignoredSampleType{})
		if !ok || reason != "non-deterministic" {
			t.Fatalf("reason got (%q, %v) want (\"non-deterministic\", true)", reason, ok)
		}
	})

	t.Run("register accepts pointer and unifies with value", func(t *testing.T) {
		RegisterIgnoredForRandomChains(&ignoredSampleType{}, "via-pointer")
		if !IsIgnoredForRandomChains(ignoredSampleType{}) {
			t.Fatalf("registering via *T must mark T as ignored")
		}
		reason, _ := IgnoredForRandomChainsReason(&ignoredSampleType{})
		if reason != "via-pointer" {
			t.Fatalf("re-registration must overwrite: got %q", reason)
		}
	})

	t.Run("unregister removes the entry", func(t *testing.T) {
		RegisterIgnoredForRandomChains(ignoredSampleType{}, "tmp")
		UnregisterIgnoredForRandomChains(ignoredSampleType{})
		if IsIgnoredForRandomChains(ignoredSampleType{}) {
			t.Fatalf("unregistered type must not be ignored")
		}
	})
}
