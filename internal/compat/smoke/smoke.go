// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package smoke implements the Sprint 114 T2 smoke scenario in Go.
//
// The smoke artefact is a single "smoke.dat" file wrapped in the Lucene
// CodecUtil index header / footer envelope, containing a fixed-length
// payload of int64 values derived deterministically from a uint64 seed.
//
// The layout MUST be byte-identical to the Java-side implementation in
// tools/lucene-fixtures/.../scenarios/SmokeScenario.java.
//
//	IndexHeader( codec="GoceneSmoke", version=0, id=16B(seed), suffix="" )
//	int32   count = 4
//	int64   value0 = seed * 1
//	int64   value1 = seed * 2
//	int64   value2 = seed * 3
//	int64   value3 = seed * 4
//	Footer  ( FOOTER_MAGIC, algorithmId=0, CRC32 of preceding bytes )
package smoke

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	// Codec name shared with the Java implementation.
	Codec = "GoceneSmoke"
	// Version of the smoke artefact.
	Version int32 = 0
	// Count is the number of int64 values in the payload.
	Count = 4
	// FileName is the file written inside the target directory.
	FileName = "smoke.dat"
)

// IDFromSeed returns the deterministic 16-byte identifier embedded in the
// CodecUtil index header. Matches SmokeScenario.idFromSeed on the Java side.
func IDFromSeed(seed int64) []byte {
	id := make([]byte, 16)
	binary.BigEndian.PutUint64(id[0:8], uint64(seed))
	binary.BigEndian.PutUint64(id[8:16], uint64(^seed))
	return id
}

// PayloadValue returns value[i] for the given seed.
func PayloadValue(seed int64, i int) int64 {
	return seed * int64(i+1)
}

// Write produces the smoke artefact at targetDir/smoke.dat for the given seed.
// The artefact is byte-identical to the one produced by the Java harness.
func Write(targetDir string, seed int64) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("smoke: mkdir %s: %w", targetDir, err)
	}
	dir, err := store.NewNIOFSDirectory(targetDir)
	if err != nil {
		return fmt.Errorf("smoke: open dir %s: %w", targetDir, err)
	}
	defer dir.Close()

	raw, err := dir.CreateOutput(FileName, store.IOContextDefault)
	if err != nil {
		return fmt.Errorf("smoke: create output: %w", err)
	}
	out := store.NewChecksumIndexOutput(raw)

	if err := codecs.WriteIndexHeader(out, Codec, Version, IDFromSeed(seed), ""); err != nil {
		out.Close()
		return fmt.Errorf("smoke: write index header: %w", err)
	}
	// Payload uses Lucene's DataOutput.writeInt / writeLong byte order, which is
	// little-endian (unlike the CodecUtil header/footer envelope which is BE).
	if err := store.WriteInt32LE(out, Count); err != nil {
		out.Close()
		return fmt.Errorf("smoke: write count: %w", err)
	}
	for i := 0; i < Count; i++ {
		if err := store.WriteInt64LE(out, PayloadValue(seed, i)); err != nil {
			out.Close()
			return fmt.Errorf("smoke: write payload[%d]: %w", i, err)
		}
	}
	if err := codecs.WriteFooter(out); err != nil {
		out.Close()
		return fmt.Errorf("smoke: write footer: %w", err)
	}
	return out.Close()
}

// Read parses and validates the smoke artefact at sourceDir/smoke.dat for the
// given seed and returns the payload values.
func Read(sourceDir string, seed int64) ([]int64, error) {
	dir, err := store.NewNIOFSDirectory(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("smoke: open dir %s: %w", sourceDir, err)
	}
	defer dir.Close()

	rawIn, err := dir.OpenInput(FileName, store.IOContextDefault)
	if err != nil {
		return nil, fmt.Errorf("smoke: open input: %w", err)
	}
	defer rawIn.Close()

	in := store.NewChecksumIndexInput(rawIn)

	if _, err := codecs.CheckIndexHeader(in, Codec, Version, Version, IDFromSeed(seed), ""); err != nil {
		return nil, fmt.Errorf("smoke: check index header: %w", err)
	}
	// Payload byte order matches Lucene's DataOutput.writeInt / writeLong (LE).
	count, err := store.ReadInt32LE(in)
	if err != nil {
		return nil, fmt.Errorf("smoke: read count: %w", err)
	}
	if count != Count {
		return nil, fmt.Errorf("smoke: count mismatch, expected %d, got %d", Count, count)
	}
	values := make([]int64, count)
	for i := int32(0); i < count; i++ {
		v, err := store.ReadInt64LE(in)
		if err != nil {
			return nil, fmt.Errorf("smoke: read payload[%d]: %w", i, err)
		}
		expected := PayloadValue(seed, int(i))
		if v != expected {
			return nil, fmt.Errorf("smoke: payload[%d] mismatch, expected %d, got %d", i, expected, v)
		}
		values[i] = v
	}
	if _, err := codecs.CheckFooter(in); err != nil {
		return nil, fmt.Errorf("smoke: check footer: %w", err)
	}
	return values, nil
}

// Path returns the smoke artefact path inside dir.
func Path(dir string) string {
	return filepath.Join(dir, FileName)
}
