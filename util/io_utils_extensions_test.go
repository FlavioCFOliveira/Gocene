// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUseOrSuppress(t *testing.T) {
	errA := errors.New("a")
	errB := errors.New("b")

	t.Run("both nil", func(t *testing.T) {
		if got := UseOrSuppress(nil, nil); got != nil {
			t.Fatalf("got %v want nil", got)
		}
	})
	t.Run("primary nil", func(t *testing.T) {
		if got := UseOrSuppress(nil, errB); got != errB {
			t.Fatalf("got %v want errB", got)
		}
	})
	t.Run("other nil", func(t *testing.T) {
		if got := UseOrSuppress(errA, nil); got != errA {
			t.Fatalf("got %v want errA", got)
		}
	})
	t.Run("both non-nil joins", func(t *testing.T) {
		got := UseOrSuppress(errA, errB)
		if !errors.Is(got, errA) || !errors.Is(got, errB) {
			t.Fatalf("got %v should wrap both errA and errB", got)
		}
	})
}

func TestFSyncAt(t *testing.T) {
	dir := t.TempDir()
	t.Run("file", func(t *testing.T) {
		path := filepath.Join(dir, "data.bin")
		if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := FSyncAt(path, false); err != nil {
			t.Fatalf("fsync file: %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		if err := FSyncAt(dir, true); err != nil {
			t.Fatalf("fsync dir: %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(dir, "does-not-exist")
		if err := FSyncAt(path, false); err == nil {
			t.Fatalf("missing file must error")
		}
	})
}
