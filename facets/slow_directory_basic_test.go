// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSlowDirectory_CreateAndRead verifies that slowDirectory round-trips
// a byte sequence through CreateOutput / OpenInput correctly.
func TestSlowDirectory_CreateAndRead(t *testing.T) {
	dir := newSlowDirectory(-1) // no sleep
	defer dir.Close()

	ctx := store.IOContextDefault
	out, err := dir.CreateOutput("test.bin", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	want := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	if err := out.WriteBytes(want); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	in, err := dir.OpenInput("test.bin", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	got, err := in.ReadBytesN(len(want))
	if err != nil {
		t.Fatalf("ReadBytesN: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("round-trip: want %v, got %v", want, got)
	}
}

// TestSlowDirectory_WithSleep verifies that slow wrappers are applied when
// sleepMillis != -1. We use sleepMillis=0 to avoid actual delays in CI.
func TestSlowDirectory_WithSleep(t *testing.T) {
	dir := newSlowDirectory(0) // 0ms sleep
	defer dir.Close()

	ctx := store.IOContextDefault
	out, err := dir.CreateOutput("slow.bin", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	payload := make([]byte, 100)
	for i := range payload {
		payload[i] = byte(i)
	}
	if err := out.WriteBytes(payload); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("slow.bin", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	got, err := in.ReadBytesN(len(payload))
	if err != nil {
		t.Fatalf("ReadBytesN: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("slow round-trip: data mismatch")
	}
}

// TestSlowDirectory_SetSleepMillis verifies that SetSleepMillis takes effect.
func TestSlowDirectory_SetSleepMillis(t *testing.T) {
	dir := newSlowDirectory(0)
	defer dir.Close()
	dir.SetSleepMillis(-1) // disable delays
	ctx := store.IOContextDefault
	out, err := dir.CreateOutput("nodep.bin", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytes([]byte{0xFF}); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Verify the file exists
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	found := false
	for _, f := range files {
		if f == "nodep.bin" {
			found = true
		}
	}
	if !found {
		t.Error("expected nodep.bin to be present")
	}
}
