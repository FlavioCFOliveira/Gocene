// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"errors"
	"testing"
)

// TestSimpleFSIndexInput_CloneNilFileGuard verifies that read methods on a
// SimpleFSIndexInput with a nil file handle return an error instead of
// panicking. This covers the case where Clone() fails (e.g., file descriptor
// exhaustion). The ensureFileOpen() guard is checked before the directory
// IsOpen() check, so a nil directory is safe here.
func TestSimpleFSIndexInput_CloneNilFileGuard(t *testing.T) {
	in := &SimpleFSIndexInput{
		BaseIndexInput: NewBaseIndexInput("test(description)", 100),
		file:           nil, // nil file simulates failed Clone
		path:           "/nonexistent/path",
		name:           "test",
	}

	_, err := in.ReadByte()
	if err == nil {
		t.Error("ReadByte on nil-file input should return error")
	}
	if !errors.Is(err, ErrIllegalState) {
		t.Errorf("ReadByte error should wrap ErrIllegalState, got: %v", err)
	}

	err = in.ReadBytes(make([]byte, 10))
	if err == nil {
		t.Error("ReadBytes on nil-file input should return error")
	}
	if !errors.Is(err, ErrIllegalState) {
		t.Errorf("ReadBytes error should wrap ErrIllegalState, got: %v", err)
	}
}

// TestNIOFSIndexInput_CloneNilFileGuard verifies that read methods on a
// NIOFSIndexInput with nil file/bufReader return an error instead of panicking.
func TestNIOFSIndexInput_CloneNilFileGuard(t *testing.T) {
	in := &NIOFSIndexInput{
		BaseIndexInput: NewBaseIndexInput("test(description)", 100),
		file:           nil,
		bufReader:      nil,
		path:           "/nonexistent/path",
		name:           "test",
	}

	_, err := in.ReadByte()
	if err == nil {
		t.Error("ReadByte on nil-file input should return error")
	}
	if !errors.Is(err, ErrIllegalState) {
		t.Errorf("ReadByte error should wrap ErrIllegalState, got: %v", err)
	}

	err = in.ReadBytes(make([]byte, 10))
	if err == nil {
		t.Error("ReadBytes on nil-file input should return error")
	}
	if !errors.Is(err, ErrIllegalState) {
		t.Errorf("ReadBytes error should wrap ErrIllegalState, got: %v", err)
	}
}

// TestMMapIndexInput_CloneNilChunksGuard verifies that read methods on an
// MMapIndexInput with nil chunks return an error instead of panicking.
func TestMMapIndexInput_CloneNilChunksGuard(t *testing.T) {
	in := &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput("test(description)", 100),
		chunks:         nil, // nil chunks simulates failed Clone
		chunkSize:      1 << 20,
		path:           "/nonexistent/path",
		name:           "test",
	}

	_, err := in.ReadByte()
	if err == nil {
		t.Error("ReadByte on nil-chunks input should return error")
	}
	if !errors.Is(err, ErrIllegalState) {
		t.Errorf("ReadByte error should wrap ErrIllegalState, got: %v", err)
	}

	err = in.ReadBytes(make([]byte, 10))
	if err == nil {
		t.Error("ReadBytes on nil-chunks input should return error")
	}
	if !errors.Is(err, ErrIllegalState) {
		t.Errorf("ReadBytes error should wrap ErrIllegalState, got: %v", err)
	}
}
