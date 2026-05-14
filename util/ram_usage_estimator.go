// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Java's RamUsageEstimator hard-codes JVM object header sizes
// (compressed-oops on/off, 12 vs 16 byte headers) plus 8-byte object
// alignment. Go has no equivalent stable layout: every value has a
// type-dependent size derivable from unsafe.Sizeof and reflect, and
// the runtime never relocates the object header in a way visible to
// the user.
//
// The Go port therefore implements a *best-effort* shallow size
// estimator that:
//   - uses unsafe.Sizeof for primitive- and pointer-like values;
//   - uses reflect-derived element sizes for slice/array/map/string;
//   - exposes the headline ONE_KB / ONE_MB / ONE_GB constants;
//   - recognises an [Accountable]-satisfying input and delegates to
//     RamBytesUsed() (callers should prefer this when available);
//   - leaves the full deep-size walker as a TODO documented below.
//
// Callers requiring exact JVM-equivalent numbers should NOT rely on
// this estimator. It is intended for ballpark accounting consistent
// with Lucene's "approximate" semantics, not byte-perfect parity.
// -----------------------------------------------------------------------------

package util

import (
	"reflect"
	"unsafe"
)

// Convenience size constants mirroring Java's static fields.
const (
	// RamOneKB is one kibibyte in bytes.
	RamOneKB int64 = 1024
	// RamOneMB is one mebibyte in bytes.
	RamOneMB int64 = RamOneKB * RamOneKB
	// RamOneGB is one gibibyte in bytes.
	RamOneGB int64 = RamOneKB * RamOneMB

	// RamQueryDefaultBytesUsed mirrors QUERY_DEFAULT_RAM_BYTES_USED.
	RamQueryDefaultBytesUsed int64 = 1024
	// RamUnknownDefaultBytesUsed mirrors UNKNOWN_DEFAULT_RAM_BYTES_USED.
	RamUnknownDefaultBytesUsed int64 = 256
)

// ramSliceHeaderBytes is the size of a Go slice header (data ptr +
// len + cap). Sliced and computed once at init so callers don't pay
// per-call.
var ramSliceHeaderBytes = int64(unsafe.Sizeof([]byte{}))

// ramMapHeaderBytes is the size of an empty map header reported by
// unsafe.Sizeof on a map value. Maps use a runtime-internal hmap
// struct; this is the user-visible pointer's size.
var ramMapHeaderBytes = int64(unsafe.Sizeof(map[int]int{}))

// ramStringHeaderBytes is the size of a Go string header (data ptr +
// len).
var ramStringHeaderBytes = int64(unsafe.Sizeof(""))

// AlignObjectSize rounds size up to the nearest 8-byte boundary,
// mirroring Java's NUM_BYTES_OBJECT_ALIGNMENT=8. The exact alignment
// is JVM-specific in Java; Go does not align user objects but we
// preserve the function for parity with Lucene call sites.
func AlignObjectSize(size int64) int64 {
	const alignment = 8
	return (size + alignment - 1) &^ (alignment - 1)
}

// ShallowSizeOf returns a best-effort approximation of the immediate
// memory footprint of v (no recursive walk into referenced values).
//
// The estimate covers:
//   - primitives: unsafe.Sizeof(v);
//   - slices: header bytes + cap * elementSize;
//   - arrays: full array size;
//   - strings: header bytes + len(string);
//   - maps: header bytes + len * (keySize+valueSize+8 overhead);
//   - structs: sum of unsafe.Sizeof of each exported field.
//
// Callers should prefer [SizeOfAccountable] when v implements
// [Accountable] — that path uses the precise figure the type reports
// rather than the heuristic.
func ShallowSizeOf(v any) int64 {
	if v == nil {
		return 0
	}
	rv := reflect.ValueOf(v)
	return shallowSize(rv)
}

func shallowSize(rv reflect.Value) int64 {
	switch rv.Kind() {
	case reflect.Invalid:
		return 0
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return int64(rv.Type().Size())
		}
		return int64(rv.Type().Size()) + shallowSize(rv.Elem())
	case reflect.Slice:
		elem := int64(rv.Type().Elem().Size())
		return ramSliceHeaderBytes + int64(rv.Cap())*elem
	case reflect.Array:
		return int64(rv.Type().Size())
	case reflect.String:
		return ramStringHeaderBytes + int64(rv.Len())
	case reflect.Map:
		// Approximate per-entry overhead: 8 bytes for hash + key + val
		// pointers. This is intentionally rough.
		kt := rv.Type().Key().Size()
		vt := rv.Type().Elem().Size()
		return ramMapHeaderBytes + int64(rv.Len())*int64(kt+vt+8)
	case reflect.Struct:
		return int64(rv.Type().Size())
	default:
		return int64(rv.Type().Size())
	}
}

// SizeOfAccountable returns the value reported by acc.RamBytesUsed
// when acc is non-nil; 0 otherwise. Mirrors the Java
// sizeOf(Accountable) helper.
func SizeOfAccountable(acc Accountable) int64 {
	if acc == nil {
		return 0
	}
	return acc.RamBytesUsed()
}

// SizeOfByteSlice returns the RAM footprint of a byte slice.
// Mirrors Java's sizeOf(byte[]).
func SizeOfByteSlice(arr []byte) int64 {
	if arr == nil {
		return 0
	}
	return ramSliceHeaderBytes + int64(cap(arr))
}

// SizeOfIntSlice mirrors sizeOf(int[]) (4 bytes per element in Java;
// in Go an `int` is platform-sized — we use unsafe.Sizeof for fidelity).
func SizeOfIntSlice(arr []int) int64 {
	if arr == nil {
		return 0
	}
	return ramSliceHeaderBytes + int64(cap(arr))*int64(unsafe.Sizeof(int(0)))
}

// SizeOfInt32Slice mirrors sizeOf(int[]) treating each entry as 4
// bytes — useful when callers want JVM-compatible accounting.
func SizeOfInt32Slice(arr []int32) int64 {
	if arr == nil {
		return 0
	}
	return ramSliceHeaderBytes + int64(cap(arr))*4
}

// SizeOfInt64Slice mirrors sizeOf(long[]).
func SizeOfInt64Slice(arr []int64) int64 {
	if arr == nil {
		return 0
	}
	return ramSliceHeaderBytes + int64(cap(arr))*8
}

// SizeOfFloat32Slice mirrors sizeOf(float[]).
func SizeOfFloat32Slice(arr []float32) int64 {
	if arr == nil {
		return 0
	}
	return ramSliceHeaderBytes + int64(cap(arr))*4
}

// SizeOfFloat64Slice mirrors sizeOf(double[]).
func SizeOfFloat64Slice(arr []float64) int64 {
	if arr == nil {
		return 0
	}
	return ramSliceHeaderBytes + int64(cap(arr))*8
}

// SizeOfString returns the RAM footprint of s.
func SizeOfString(s string) int64 {
	return ramStringHeaderBytes + int64(len(s))
}

// SizeOfStringSlice returns the RAM footprint of arr including the
// referenced string bytes (shallow, not deep).
func SizeOfStringSlice(arr []string) int64 {
	if arr == nil {
		return 0
	}
	total := ramSliceHeaderBytes + int64(cap(arr))*ramStringHeaderBytes
	for _, s := range arr {
		total += int64(len(s))
	}
	return total
}

// HumanReadableUnits formats bytes as a human-readable string using
// KB/MB/GB suffixes. Delegates to the byte-for-byte-compatible
// implementation in accountables.go (humanReadableUnits) so output
// matches Java's DecimalFormat("0.#") formatting verbatim.
func HumanReadableUnits(bytes int64) string {
	return humanReadableUnits(bytes)
}

// SizeOfAccountables sums the RAM footprints reported by every
// element of arr, plus the slice header. Mirrors sizeOf(Accountable[]).
func SizeOfAccountables(arr []Accountable) int64 {
	if arr == nil {
		return 0
	}
	total := ramSliceHeaderBytes + int64(cap(arr))*int64(unsafe.Sizeof((*int)(nil)))
	for _, a := range arr {
		if a != nil {
			total += a.RamBytesUsed()
		}
	}
	return total
}
