// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"io"
	"testing"
	"testing/fstest"
)

// TestModuleResourceLoader_OpenResource exercises the happy path: a
// resource available on the underlying fs.FS is read back verbatim.
// Indirect coverage mirroring the role of ModuleResourceLoader in
// Lucene's SPI bootstrap (see ClasspathResourceLoader test peers).
func TestModuleResourceLoader_OpenResource(t *testing.T) {
	mod := fstest.MapFS{
		"dict/words.txt":   {Data: []byte("alpha\nbeta\n")},
		"config/codec.cfg": {Data: []byte("codec=lucene99")},
	}
	loader := NewModuleResourceLoader(mod, nil)

	rc, err := loader.OpenResource("dict/words.txt")
	if err != nil {
		t.Fatalf("OpenResource: %v", err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(b) != "alpha\nbeta\n" {
		t.Fatalf("content mismatch: %q", b)
	}
}

// TestModuleResourceLoader_OpenResource_LeadingSlash verifies that
// Lucene's "absolute to module root" semantics are honoured: leading
// slashes are stripped so io/fs lookups succeed.
func TestModuleResourceLoader_OpenResource_LeadingSlash(t *testing.T) {
	mod := fstest.MapFS{"hello.txt": {Data: []byte("hi")}}
	loader := NewModuleResourceLoader(mod, nil)

	rc, err := loader.OpenResource("/hello.txt")
	if err != nil {
		t.Fatalf("OpenResource: %v", err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil || string(b) != "hi" {
		t.Fatalf("got %q err=%v", b, err)
	}
}

// TestModuleResourceLoader_OpenResource_NotFound verifies the Java
// "Resource not found" IOException analogue: ErrResourceNotFound is
// surfaced for missing paths.
func TestModuleResourceLoader_OpenResource_NotFound(t *testing.T) {
	loader := NewModuleResourceLoader(fstest.MapFS{}, nil)
	if _, err := loader.OpenResource("nope"); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("expected ErrResourceNotFound, got %v", err)
	}
}

// TestModuleResourceLoader_NilModule exercises the defensive fallback
// when no fs.FS is configured.
func TestModuleResourceLoader_NilModule(t *testing.T) {
	loader := NewModuleResourceLoader(nil, nil)
	if _, err := loader.OpenResource("anything"); !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("expected ErrResourceNotFound, got %v", err)
	}
}

// TestModuleResourceLoader_NewInstance covers the SPI registry path
// that mirrors Class.forName(Module, name) in Java.
func TestModuleResourceLoader_NewInstance(t *testing.T) {
	type fakeAnalyzer struct{ name string }
	loader := NewModuleResourceLoader(nil, map[string]FactoryFunc{
		"standard": func() (any, error) { return &fakeAnalyzer{name: "standard"}, nil },
	})

	v, err := loader.NewInstance("standard")
	if err != nil {
		t.Fatalf("NewInstance: %v", err)
	}
	fa, ok := v.(*fakeAnalyzer)
	if !ok || fa.name != "standard" {
		t.Fatalf("unexpected value: %#v", v)
	}

	if _, err := loader.NewInstance("missing"); !errors.Is(err, ErrFactoryNotFound) {
		t.Fatalf("expected ErrFactoryNotFound, got %v", err)
	}
}

// TestModuleResourceLoader_ImplementsResourceLoader pins the interface
// satisfaction at compile time so accidental signature drift on
// ResourceLoader breaks here.
func TestModuleResourceLoader_ImplementsResourceLoader(t *testing.T) {
	var _ ResourceLoader = (*ModuleResourceLoader)(nil)
}
