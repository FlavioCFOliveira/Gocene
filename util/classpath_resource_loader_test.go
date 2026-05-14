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

func TestClasspathResourceLoader_OpenResource_Found(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"stopwords.txt": &fstest.MapFile{Data: []byte("the\nand\nor\n")},
	}
	rl := NewClasspathResourceLoader(fsys, nil)

	rc, err := rl.OpenResource("stopwords.txt")
	if err != nil {
		t.Fatalf("OpenResource error: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(got) != "the\nand\nor\n" {
		t.Errorf("content = %q", string(got))
	}
}

func TestClasspathResourceLoader_OpenResource_Missing(t *testing.T) {
	t.Parallel()

	rl := NewClasspathResourceLoader(fstest.MapFS{}, nil)
	_, err := rl.OpenResource("nope.txt")
	if err == nil {
		t.Fatalf("expected error for missing resource")
	}
	if !errors.Is(err, ErrResourceNotFound) {
		t.Errorf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestClasspathResourceLoader_OpenResource_NoFS(t *testing.T) {
	t.Parallel()

	rl := NewClasspathResourceLoader(nil, nil)
	_, err := rl.OpenResource("anything")
	if !errors.Is(err, ErrResourceNotFound) {
		t.Errorf("expected ErrResourceNotFound, got %v", err)
	}
}

func TestClasspathResourceLoader_NewInstance(t *testing.T) {
	t.Parallel()

	type customAnalyzer struct{ name string }
	factories := map[string]FactoryFunc{
		"custom": func() (any, error) { return &customAnalyzer{name: "custom"}, nil },
	}
	rl := NewClasspathResourceLoader(nil, factories)

	got, err := rl.NewInstance("custom")
	if err != nil {
		t.Fatalf("NewInstance error: %v", err)
	}
	ca, ok := got.(*customAnalyzer)
	if !ok || ca.name != "custom" {
		t.Errorf("NewInstance returned %#v", got)
	}
}

func TestClasspathResourceLoader_FindFactory_Missing(t *testing.T) {
	t.Parallel()

	rl := NewClasspathResourceLoader(nil, map[string]FactoryFunc{})
	_, err := rl.FindFactory("absent")
	if !errors.Is(err, ErrFactoryNotFound) {
		t.Errorf("expected ErrFactoryNotFound, got %v", err)
	}

	rl2 := NewClasspathResourceLoader(nil, nil)
	_, err = rl2.FindFactory("absent")
	if !errors.Is(err, ErrFactoryNotFound) {
		t.Errorf("expected ErrFactoryNotFound (nil map), got %v", err)
	}
}

func TestClasspathResourceLoader_NewInstance_FactoryError(t *testing.T) {
	t.Parallel()

	bad := errors.New("bad init")
	rl := NewClasspathResourceLoader(nil, map[string]FactoryFunc{
		"x": func() (any, error) { return nil, bad },
	})
	_, err := rl.NewInstance("x")
	if !errors.Is(err, bad) {
		t.Errorf("expected wrapped bad init, got %v", err)
	}
}

// Static type check.
var _ ResourceLoader = (*ClasspathResourceLoader)(nil)
