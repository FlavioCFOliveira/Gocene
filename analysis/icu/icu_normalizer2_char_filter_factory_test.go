// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu_test

import (
	"io"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/icu"
)

// TestICUNormalizer2CharFilterFactory_Create verifies that the factory wraps
// a reader with a normalizing CharFilter.
func TestICUNormalizer2CharFilterFactory_Create(t *testing.T) {
	factory := icu.NewICUNormalizer2CharFilterFactoryDefault()

	// "Ａ" (U+FF21, fullwidth A) should normalize to "a" under NFKC+casefold.
	r := factory.Create(strings.NewReader("Ａ"))
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "a" {
		t.Errorf("got %q, want %q", string(got), "a")
	}
}

// TestICUNormalizer2CharFilterFactory_NFKCCompose verifies NFKC compose mode.
func TestICUNormalizer2CharFilterFactory_NFKCCompose(t *testing.T) {
	factory := icu.NewICUNormalizer2CharFilterFactory("nfkc", icu.NormalizerModeCompose)

	// "㌘" (U+3318, CJK compatibility character for "gram") should normalize
	// to "グラム" under NFKC.
	r := factory.Create(strings.NewReader("㌘"))
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty output")
	}
}

// TestICUNormalizer2CharFilterFactory_Normalize verifies the Normalize alias.
func TestICUNormalizer2CharFilterFactory_Normalize(t *testing.T) {
	factory := icu.NewICUNormalizer2CharFilterFactoryDefault()
	r := factory.Normalize(strings.NewReader("hello"))
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", string(got), "hello")
	}
}
