// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

func TestBaseDirectory(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new base directory has correct initial state",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				if bd == nil {
					t.Fatal("expected non-nil BaseDirectory")
				}
				if !bd.IsOpen() {
					t.Error("expected directory to be open")
				}
				if bd.GetLockFactory() == nil {
					t.Error("expected lock factory to be set")
				}
			},
		},
		{
			name: "ensure open returns nil when open",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				if err := bd.EnsureOpen(); err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			},
		},
		{
			name: "ensure open returns error when closed",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				bd.MarkClosed()
				if err := bd.EnsureOpen(); err == nil {
					t.Error("expected error for closed directory")
				}
			},
		},
		{
			name: "track open files",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)

				bd.AddOpenFile("test.txt")
				if !bd.IsFileOpen("test.txt") {
					t.Error("expected file to be tracked as open")
				}
				if bd.GetOpenFileCount("test.txt") != 1 {
					t.Errorf("expected count 1, got %d", bd.GetOpenFileCount("test.txt"))
				}

				bd.AddOpenFile("test.txt")
				if bd.GetOpenFileCount("test.txt") != 2 {
					t.Errorf("expected count 2, got %d", bd.GetOpenFileCount("test.txt"))
				}

				bd.RemoveOpenFile("test.txt")
				if bd.GetOpenFileCount("test.txt") != 1 {
					t.Errorf("expected count 1, got %d", bd.GetOpenFileCount("test.txt"))
				}

				bd.RemoveOpenFile("test.txt")
				if bd.IsFileOpen("test.txt") {
					t.Error("expected file to not be open")
				}
			},
		},
		{
			name: "set lock factory",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				factory := NewNativeFSLockFactory()

				if err := bd.SetLockFactory(factory); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if bd.GetLockFactory() != factory {
					t.Error("expected lock factory to be set")
				}
			},
		},
		{
			name: "set lock factory returns error when closed",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)
				bd.MarkClosed()

				if err := bd.SetLockFactory(NewNativeFSLockFactory()); err == nil {
					t.Error("expected error when setting factory on closed directory")
				}
			},
		},
		{
			name: "set nil lock factory returns error",
			fn: func(t *testing.T) {
				bd := NewBaseDirectory(nil)

				if err := bd.SetLockFactory(nil); err == nil {
					t.Error("expected error when setting nil factory")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrFileNotFound", ErrFileNotFound},
		{"ErrFileAlreadyExists", ErrFileAlreadyExists},
		{"ErrFileIsOpen", ErrFileIsOpen},
		{"ErrIllegalState", ErrIllegalState},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("expected non-nil error")
			}
		})
	}
}
