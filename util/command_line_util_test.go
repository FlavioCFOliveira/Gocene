// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"testing"
)

func TestAdjustDirectoryClassName_Empty(t *testing.T) {
	t.Parallel()

	cases := []string{"", "   ", "\t\n"}
	for _, in := range cases {
		_, err := AdjustDirectoryClassName(in)
		if !errors.Is(err, ErrDirectoryNameEmpty) {
			t.Errorf("AdjustDirectoryClassName(%q) err = %v, want ErrDirectoryNameEmpty", in, err)
		}
	}
}

func TestAdjustDirectoryClassName_BareName(t *testing.T) {
	t.Parallel()

	got, err := AdjustDirectoryClassName("MMapDirectory")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := DirectoryPackagePrefix + "MMapDirectory"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdjustDirectoryClassName_Qualified(t *testing.T) {
	t.Parallel()

	in := "github.com/SomeOrg/SomeRepo/store.CustomDir"
	got, err := AdjustDirectoryClassName(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

func TestAdjustDirectoryClassName_TrimsWhitespace(t *testing.T) {
	t.Parallel()

	got, err := AdjustDirectoryClassName("  MMapDirectory  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := DirectoryPackagePrefix + "MMapDirectory"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDirectoryRegistry_RegisterAndLookup(t *testing.T) {
	t.Parallel()

	reg := NewDirectoryRegistry()

	callCount := 0
	factory := func(path string, lockFactory any) (any, error) {
		callCount++
		return path + "@" + (lockFactory.(string)), nil
	}

	key, err := reg.Register("FakeDir", factory)
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if want := DirectoryPackagePrefix + "FakeDir"; key != want {
		t.Errorf("Register key = %q, want %q", key, want)
	}

	got, err := reg.NewDirectoryFromName("FakeDir", "/tmp/data", "native-lock")
	if err != nil {
		t.Fatalf("NewDirectoryFromName error: %v", err)
	}
	if s, ok := got.(string); !ok || s != "/tmp/data@native-lock" {
		t.Errorf("factory result = %v", got)
	}

	// Look up by both bare and qualified forms — both must hit.
	if _, err := reg.Lookup("FakeDir"); err != nil {
		t.Errorf("Lookup bare: %v", err)
	}
	if _, err := reg.Lookup(DirectoryPackagePrefix + "FakeDir"); err != nil {
		t.Errorf("Lookup qualified: %v", err)
	}
	if callCount != 1 {
		t.Errorf("factory call count = %d", callCount)
	}
}

func TestDirectoryRegistry_Lookup_Missing(t *testing.T) {
	t.Parallel()

	reg := NewDirectoryRegistry()
	_, err := reg.Lookup("Absent")
	if !errors.Is(err, ErrDirectoryNotRegistered) {
		t.Errorf("Lookup absent = %v, want ErrDirectoryNotRegistered", err)
	}
}

func TestDirectoryRegistry_Register_NilFactory(t *testing.T) {
	t.Parallel()

	reg := NewDirectoryRegistry()
	if _, err := reg.Register("X", nil); err == nil {
		t.Errorf("Register(nil) should fail")
	}
}

func TestDirectoryRegistry_Register_EmptyName(t *testing.T) {
	t.Parallel()

	reg := NewDirectoryRegistry()
	if _, err := reg.Register("", func(string, any) (any, error) { return nil, nil }); !errors.Is(err, ErrDirectoryNameEmpty) {
		t.Errorf("Register('') err = %v, want ErrDirectoryNameEmpty", err)
	}
}
