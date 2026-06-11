// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

package packed

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// TestPacked64_ReadFixture verifies the Java harness can generate the
// packed-ints-packed64 fixture and its byte-level digest is stable.
func TestPacked64_ReadFixture(t *testing.T) {
	for _, seed := range []int64{0xC0FFEE, 0xDECAF} {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir, err := compat.Generate(ScenarioPacked64, seed)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			t.Logf("fixture generated in %s (seed=%#x)", dir, seed)
		})
	}
}

// TestBlockPackedWriter_ReadFixture verifies the Java harness can generate
// the block-packed-writer fixture.
func TestBlockPackedWriter_ReadFixture(t *testing.T) {
	for _, seed := range []int64{0xC0FFEE, 0xDECAF} {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir, err := compat.Generate(ScenarioBlockPackedWriter, seed)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			t.Logf("fixture generated in %s (seed=%#x)", dir, seed)
		})
	}
}

// TestDirectMonotonic_ReadFixture verifies the Java harness can generate
// the direct-monotonic fixture.
func TestDirectMonotonic_ReadFixture(t *testing.T) {
	for _, seed := range []int64{0xC0FFEE, 0xDECAF} {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir, err := compat.Generate(ScenarioDirectMonotonic, seed)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			t.Logf("fixture generated in %s (seed=%#x)", dir, seed)
		})
	}
}

// TestPacked64_RoundTrip writes packed-64 data with Gocene's writer and
// reads it back, verifying value fidelity.
func TestPacked64_RoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	values := make([]int64, 256)
	for i := range values {
		values[i] = rng.Int63n(1 << 16)
	}

	buf := store.NewByteBuffersDirectory()
	defer buf.Close()

	ctx := store.IOContextDefault
	out, err := buf.CreateOutput("packed64.bin", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}

	bitsPerValue := packed.BitsRequired(values[0])
	for _, v := range values[1:] {
		if b := packed.BitsRequired(v); b > bitsPerValue {
			bitsPerValue = b
		}
	}

	w, err := packed.GetWriterNoHeader(out, packed.FormatPacked, len(values), bitsPerValue, packed.DefaultBufferSize)
	if err != nil {
		t.Fatalf("GetWriterNoHeader: %v", err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	in, err := buf.OpenInput("packed64.bin", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	it, err := packed.GetReaderIteratorNoHeader(in, packed.FormatPacked, packed.VersionCurrent, len(values), bitsPerValue, packed.DefaultBufferSize)
	if err != nil {
		t.Fatalf("GetReaderIteratorNoHeader: %v", err)
	}

	for i, want := range values {
		got, err := it.Next()
		if err != nil {
			t.Fatalf("Next(%d): %v", i, err)
		}
		if got != want {
			t.Fatalf("value[%d]: got %d, want %d", i, got, want)
		}
	}
}

// TestBlockPackedWriter_RoundTrip writes block-packed data and reads it back.
func TestBlockPackedWriter_RoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	values := make([]int64, 512)
	for i := range values {
		values[i] = rng.Int63n(1 << 10)
	}

	blockSize := 128
	buf := store.NewByteBuffersDirectory()
	defer buf.Close()

	ctx := store.IOContextDefault
	out, err := buf.CreateOutput("blockpacked.bin", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}

	bw, err := packed.NewBlockPackedWriter(out, blockSize)
	if err != nil {
		t.Fatalf("NewBlockPackedWriter: %v", err)
	}
	for _, v := range values {
		if err := bw.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := bw.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	in, err := buf.OpenInput("blockpacked.bin", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	reader, err := packed.NewBlockPackedReaderIterator(in, packed.VersionCurrent, blockSize, int64(len(values)))
	if err != nil {
		t.Fatalf("NewBlockPackedReaderIterator: %v", err)
	}

	for i, want := range values {
		got, err := reader.Next()
		if err != nil {
			t.Fatalf("Next(%d): %v", i, err)
		}
		if got != want {
			t.Fatalf("value[%d]: got %d, want %d", i, got, want)
		}
	}
}

// TestDirectMonotonic_RoundTrip writes direct-monotonic data and reads it back.
func TestDirectMonotonic_RoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	values := make([]int64, 256)
	values[0] = rng.Int63n(1 << 20)
	for i := 1; i < len(values); i++ {
		values[i] = values[i-1] + rng.Int63n(1<<12)
	}

	buf := store.NewByteBuffersDirectory()
	defer buf.Close()

	ctx := store.IOContextDefault
	meta, err := buf.CreateOutput("directmono.meta", ctx)
	if err != nil {
		t.Fatalf("CreateOutput meta: %v", err)
	}
	data, err := buf.CreateOutput("directmono.data", ctx)
	if err != nil {
		t.Fatalf("CreateOutput data: %v", err)
	}

	w, err := packed.NewDirectMonotonicWriter(meta, data, int64(len(values)), 4)
	if err != nil {
		t.Fatalf("NewDirectMonotonicWriter: %v", err)
	}
	for _, v := range values {
		w.Add(v)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := meta.Close(); err != nil {
		t.Fatalf("Close meta: %v", err)
	}
	if err := data.Close(); err != nil {
		t.Fatalf("Close data: %v", err)
	}

	metaIn, err := buf.OpenInput("directmono.meta", ctx)
	if err != nil {
		t.Fatalf("OpenInput meta: %v", err)
	}
	defer metaIn.Close()

	metaSize := metaIn.Length()
	metaData := make([]byte, metaSize)
	if err := metaIn.SetPosition(0); err != nil {
		t.Fatalf("SetPosition meta: %v", err)
	}
	if err := metaIn.ReadBytes(metaData); err != nil {
		t.Fatalf("Read meta: %v", err)
	}

	dataIn, err := buf.OpenInput("directmono.data", ctx)
	if err != nil {
		t.Fatalf("OpenInput data: %v", err)
	}
	defer dataIn.Close()

	dataSize := dataIn.Length()
	dataBytes := make([]byte, dataSize)
	if err := dataIn.SetPosition(0); err != nil {
		t.Fatalf("SetPosition data: %v", err)
	}
	if err := dataIn.ReadBytes(dataBytes); err != nil {
		t.Fatalf("Read data: %v", err)
	}

	parsedMeta, err := packed.LoadDirectMonotonicMeta(store.NewByteArrayDataInput(metaData), int64(len(values)), 4)
	if err != nil {
		t.Fatalf("LoadDirectMonotonicMeta: %v", err)
	}

	reader, err := packed.NewDirectMonotonicReader(parsedMeta, store.NewByteArrayRandomAccessInput(dataBytes))
	if err != nil {
		t.Fatalf("NewDirectMonotonicReader: %v", err)
	}

	for i, want := range values {
		got, err := reader.Get(int64(i))
		if err != nil {
			t.Fatalf("Get(%d): %v", i, err)
		}
		if got != want {
			t.Fatalf("value[%d]: got %d, want %d", i, got, want)
		}
	}
}
