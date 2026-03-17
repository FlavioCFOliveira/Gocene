// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

func TestNewFileSwitchDirectory(t *testing.T) {
	primary := NewByteBuffersDirectory()
	secondary := NewByteBuffersDirectory()
	extensions := []string{".fdt", ".fdx"}

	dir := NewFileSwitchDirectory(primary, secondary, extensions)

	if dir.GetPrimaryDirectory() != primary {
		t.Error("expected primary directory to match")
	}

	if dir.GetSecondaryDirectory() != secondary {
		t.Error("expected secondary directory to match")
	}

	extMap := dir.GetExtensions()
	if len(extMap) != 2 {
		t.Errorf("expected 2 extensions, got %d", len(extMap))
	}

	// Check that extensions are normalized with dots
	if !extMap[".fdt"] {
		t.Error("expected .fdt to be in extensions")
	}
	if !extMap[".fdx"] {
		t.Error("expected .fdx to be in extensions")
	}
}

func TestFileSwitchDirectory_FileOperations(t *testing.T) {
	primary := NewByteBuffersDirectory()
	secondary := NewByteBuffersDirectory()
	extensions := []string{".dat"}

	dir := NewFileSwitchDirectory(primary, secondary, extensions)

	// Create a file that should go to primary
	ctx := IOContextRead
	out, err := dir.CreateOutput("test.txt", ctx)
	if err != nil {
		t.Fatalf("failed to create output: %v", err)
	}
	out.WriteBytes([]byte("hello"))
	out.Close()

	// Create a file that should go to secondary
	out2, err := dir.CreateOutput("test.dat", IOContextWrite)
	if err != nil {
		t.Fatalf("failed to create output: %v", err)
	}
	out2.WriteBytes([]byte("world"))
	out2.Close()

	// Check file existence
	if !dir.FileExists("test.txt") {
		t.Error("expected test.txt to exist")
	}
	if !dir.FileExists("test.dat") {
		t.Error("expected test.dat to exist")
	}

	// Check file lengths
	len1, err := dir.FileLength("test.txt")
	if err != nil {
		t.Fatalf("failed to get file length: %v", err)
	}
	if len1 != 5 {
		t.Errorf("expected length 5, got %d", len1)
	}

	len2, err := dir.FileLength("test.dat")
	if err != nil {
		t.Fatalf("failed to get file length: %v", err)
	}
	if len2 != 5 {
		t.Errorf("expected length 5, got %d", len2)
	}

	// List all files
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Clean up
	dir.Close()
}

func TestFileSwitchDirectory_ReadWrite(t *testing.T) {
	primary := NewByteBuffersDirectory()
	secondary := NewByteBuffersDirectory()
	extensions := []string{".dat"}

	dir := NewFileSwitchDirectory(primary, secondary, extensions)

	// Write to a file
	out, err := dir.CreateOutput("test.txt", IOContextWrite)
	if err != nil {
		t.Fatalf("failed to create output: %v", err)
	}
	out.WriteBytes([]byte("hello world"))
	out.Close()

	// Read from the file
	in, err := dir.OpenInput("test.txt", IOContextRead)
	if err != nil {
		t.Fatalf("failed to open input: %v", err)
	}

	buf := make([]byte, 11)
	err = in.ReadBytes(buf)
	if err != nil {
		t.Fatalf("failed to read bytes: %v", err)
	}

	if string(buf) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(buf))
	}

	in.Close()
	dir.Close()
}
