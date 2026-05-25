// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smoke

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSmoke_GoOnlyRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	const seed int64 = 42

	if err := Write(tmp, seed); err != nil {
		t.Fatalf("Write: %v", err)
	}
	values, err := Read(tmp, seed)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(values) != Count {
		t.Fatalf("payload length = %d, want %d", len(values), Count)
	}
	for i, v := range values {
		if want := PayloadValue(seed, i); v != want {
			t.Errorf("payload[%d] = %d, want %d", i, v, want)
		}
	}
}

func TestSmoke_ByteDeterminism(t *testing.T) {
	const seed int64 = 0xCAFE
	tmp1 := t.TempDir()
	tmp2 := t.TempDir()

	if err := Write(tmp1, seed); err != nil {
		t.Fatalf("Write tmp1: %v", err)
	}
	if err := Write(tmp2, seed); err != nil {
		t.Fatalf("Write tmp2: %v", err)
	}
	b1, err := os.ReadFile(filepath.Join(tmp1, FileName))
	if err != nil {
		t.Fatalf("ReadFile tmp1: %v", err)
	}
	b2, err := os.ReadFile(filepath.Join(tmp2, FileName))
	if err != nil {
		t.Fatalf("ReadFile tmp2: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatalf("two writes with identical seed produced different bytes (lens %d vs %d)",
			len(b1), len(b2))
	}
	// Expected total size: index-header(9 + len("GoceneSmoke")=20) + 16 id + 1 suffix-len
	//   + 4 (count int32) + 4*8 (payload) + 16 (footer) = 89 bytes.
	if got, want := len(b1), 89; got != want {
		t.Errorf("smoke.dat size = %d, want %d", got, want)
	}
}

func TestSmoke_ReadDetectsSeedMismatch(t *testing.T) {
	tmp := t.TempDir()
	if err := Write(tmp, 0); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := Read(tmp, 1); err == nil {
		t.Fatalf("Read with wrong seed should fail")
	}
}
