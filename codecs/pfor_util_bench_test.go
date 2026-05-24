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

// BenchmarkPForUtilEncode measures PForUtil.Encode throughput with a typical
// posting-list delta block (mostly 7-bit values, a few exceptions).
func BenchmarkPForUtilEncode(b *testing.B) {
	src := make([]int32, codecs.ForUtilBlockSize)
	for i := range src {
		// 90 % of values fit in 7 bits; 10 % are exceptions (14 bits).
		if rand.N(10) < 9 {
			src[i] = int32(rand.N(uint32(128)))
		} else {
			src[i] = int32(rand.N(uint32(16384)))
		}
	}

	dir := store.NewByteBuffersDirectory()
	b.Cleanup(func() { dir.Close() })

	out, err := dir.CreateOutput("bench_pfor_encode.bin", store.IOContextWrite)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { out.Close() })

	pfu := codecs.NewPForUtil(codecs.NewForUtil())

	b.ResetTimer()
	b.SetBytes(int64(codecs.ForUtilBlockSize) * 4)

	for i := 0; i < b.N; i++ {
		if err := pfu.Encode(src, out); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPForUtilDecode measures PForUtil.Decode throughput.
func BenchmarkPForUtilDecode(b *testing.B) {
	src := make([]int32, codecs.ForUtilBlockSize)
	for i := range src {
		if rand.N(10) < 9 {
			src[i] = int32(rand.N(uint32(128)))
		} else {
			src[i] = int32(rand.N(uint32(16384)))
		}
	}

	dir := store.NewByteBuffersDirectory()
	b.Cleanup(func() { dir.Close() })

	const blocks = 1024

	out, err := dir.CreateOutput("bench_pfor_decode.bin", store.IOContextWrite)
	if err != nil {
		b.Fatal(err)
	}
	pfu := codecs.NewPForUtil(codecs.NewForUtil())
	for i := 0; i < blocks; i++ {
		if err := pfu.Encode(src, out); err != nil {
			b.Fatal(err)
		}
	}
	out.Close()

	in, err := dir.OpenInput("bench_pfor_decode.bin", store.IOContextReadOnce)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { in.Close() })

	dst := make([]int64, codecs.ForUtilBlockSize)

	b.ResetTimer()
	b.SetBytes(int64(codecs.ForUtilBlockSize) * 4)

	for i := 0; i < b.N; i++ {
		if i%blocks == 0 {
			if err := in.SetPosition(0); err != nil {
				b.Fatal(err)
			}
		}
		if err := pfu.Decode(in, dst); err != nil {
			b.Fatal(err)
		}
	}
}
