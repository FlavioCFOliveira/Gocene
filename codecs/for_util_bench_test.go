// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BenchmarkForUtilEncode measures ForUtil.Encode throughput for a representative
// bit-width (10 bpv — typical posting list delta encoding).
func BenchmarkForUtilEncode(b *testing.B) {
	const bpv = 10
	src := make([]int32, codecs.ForUtilBlockSize)
	for i := range src {
		src[i] = int32(rand.N(uint32(1 << bpv)))
	}

	dir := store.NewByteBuffersDirectory()
	b.Cleanup(func() { dir.Close() })

	out, err := dir.CreateOutput("bench_encode.bin", store.IOContextWrite)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { out.Close() })

	fu := codecs.NewForUtil()

	b.ResetTimer()
	b.SetBytes(int64(codecs.ForUtilBlockSize) * 4)

	for i := 0; i < b.N; i++ {
		if err := fu.Encode(src, bpv, out); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkForUtilDecode measures ForUtil.Decode throughput for a representative
// bit-width (10 bpv). The benchmark pre-encodes 1024 blocks so that each
// b.N iteration decodes one block from a single open IndexInput, keeping
// the OpenInput overhead out of the measurement.
func BenchmarkForUtilDecode(b *testing.B) {
	const (
		bpv    = 10
		blocks = 1024
	)

	src := make([]int32, codecs.ForUtilBlockSize)
	for i := range src {
		src[i] = int32(rand.N(uint32(1 << bpv)))
	}

	dir := store.NewByteBuffersDirectory()
	b.Cleanup(func() { dir.Close() })

	out, err := dir.CreateOutput("bench_decode.bin", store.IOContextWrite)
	if err != nil {
		b.Fatal(err)
	}
	fu := codecs.NewForUtil()
	for i := 0; i < blocks; i++ {
		if err := fu.Encode(src, bpv, out); err != nil {
			b.Fatal(err)
		}
	}
	out.Close()

	in, err := dir.OpenInput("bench_decode.bin", store.IOContextReadOnce)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { in.Close() })

	dst := make([]int64, codecs.ForUtilBlockSize)

	b.ResetTimer()
	b.SetBytes(int64(codecs.ForUtilBlockSize) * 4)

	for i := 0; i < b.N; i++ {
		// Wrap around to the beginning when we exhaust all pre-encoded blocks.
		if i%blocks == 0 {
			if err := in.SetPosition(0); err != nil {
				b.Fatal(err)
			}
		}
		if err := fu.Decode(bpv, in, dst); err != nil {
			b.Fatal(err)
		}
	}
}
