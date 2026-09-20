// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file mirrors org.apache.lucene.codecs.uniformsplit.RamUsageUtil from
// Apache Lucene 10.5.0: utility methods to estimate the RAM usage of objects,
// relying on RamUsageEstimator.
//
// Java groups the methods as statics on a final-less utility class. Go has no
// static members, so each one becomes a package-level function of this package,
// and the Java overloads of ramBytesUsed — which Java distinguishes by
// parameter type — are distinguished by name here.
//
// PORT NOTE: every constant below renders a
// RamUsageEstimator.shallowSizeOfInstance(X.class) call, which measures a JVM
// object layout. Gocene's util.RamUsageEstimator is already a documented
// best-effort estimator with no JVM-equivalent numbers (see the port note at
// the head of util/ram_usage_estimator.go), so these constants measure the Go
// counterpart of each Java type and the resulting figures are approximations,
// exactly as everywhere else in Gocene. RAM accounting is never serialised, so
// the binary contract is unaffected.

var (
	// bytesRefBaseRAMUsage renders BYTES_REF_BASE_RAM_USAGE
	// (RamUsageUtil.java:41): shallowSizeOfInstance(BytesRef.class).
	bytesRefBaseRAMUsage = util.ShallowSizeOf(util.BytesRef{})

	// bytesRefBuilderBaseRAMUsage renders BYTES_REF_BUILDER_BASE_RAM_USAGE
	// (RamUsageUtil.java:42): shallowSizeOfInstance(BytesRefBuilder.class).
	bytesRefBuilderBaseRAMUsage = util.ShallowSizeOf(util.BytesRefBuilder{})

	// hashMapBaseRAMUsage renders HASH_MAP_BASE_RAM_USAGE
	// (RamUsageUtil.java:44): shallowSizeOfInstance(HashMap.class). Go's
	// counterpart of java.util.HashMap is the built-in map.
	hashMapBaseRAMUsage = util.ShallowSizeOf(map[any]any{})

	// hashMapEntryBaseRAMUsage renders HASH_MAP_ENTRY_BASE_RAM_USAGE
	// (RamUsageUtil.java:46), which the Java static initialiser obtains by
	// building a one-entry HashMap and measuring the entry object it holds
	// (RamUsageUtil.java:50-53). A Go map holds no per-entry object to measure,
	// so the same technique is applied to the Go counterpart: one entry costs
	// the difference between a one-entry map and an empty one.
	hashMapEntryBaseRAMUsage = util.ShallowSizeOf(map[any]any{struct{}{}: struct{}{}}) - hashMapBaseRAMUsage

	// unmodifiableArrayListBaseRAMUsage renders
	// UNMODIFIABLE_ARRAY_LIST_BASE_RAM_USAGE (RamUsageUtil.java:47), which the
	// Java static initialiser computes as the shallow size of the
	// Collections.unmodifiableList wrapper plus the shallow size of an
	// ArrayList (RamUsageUtil.java:54-56). Go's counterpart of ArrayList is a
	// slice, and an unmodifiable view of a slice is the slice itself: there is
	// no wrapper object, so it contributes nothing and only the backing list
	// header is counted.
	unmodifiableArrayListBaseRAMUsage = util.ShallowSizeOf([]any{})
)

// RamBytesUsedByBytesRef renders the static
// RamUsageUtil.ramBytesUsed(BytesRef) (RamUsageUtil.java:59).
func RamBytesUsedByBytesRef(bytesRef *util.BytesRef) int64 {
	return bytesRefBaseRAMUsage + util.SizeOfByteSlice(bytesRef.Bytes)
}

// RamBytesUsedByBytesRefBuilder renders the static
// RamUsageUtil.ramBytesUsed(BytesRefBuilder) (RamUsageUtil.java:63).
func RamBytesUsedByBytesRefBuilder(bytesRefBuilder *util.BytesRefBuilder) int64 {
	return bytesRefBuilderBaseRAMUsage + RamBytesUsedByBytesRef(bytesRefBuilder.Get())
}

// RamBytesUsedByTermState renders the static
// RamUsageUtil.ramBytesUsed(TermState) (RamUsageUtil.java:67), whose body
// delegates to DeltaBaseTermStateSerializer.ramBytesUsed(TermState).
func RamBytesUsedByTermState(termState index.TermState) int64 {
	return DeltaBaseTermStateSerializerRamBytesUsed(termState)
}

// RamBytesUsedByByteArrayOfLength renders
// RamUsageUtil.ramBytesUsedByByteArrayOfLength(int) (RamUsageUtil.java:71).
// Java's Byte.BYTES is 1.
func RamBytesUsedByByteArrayOfLength(length int) int64 {
	return util.AlignObjectSize(int64(util.NumBytesArrayHeader) + 1*int64(length))
}

// RamBytesUsedByHashMapOfSize renders
// RamUsageUtil.ramBytesUsedByHashMapOfSize(int) (RamUsageUtil.java:75).
//
// Java's `(int) (size / 0.6)` is a double division followed by a truncating
// cast, not an integer division.
func RamBytesUsedByHashMapOfSize(size int) int64 {
	return hashMapBaseRAMUsage +
		RamBytesUsedByObjectArrayOfLength(int(float64(size)/0.6)) +
		hashMapEntryBaseRAMUsage*int64(size)
}

// RamBytesUsedByUnmodifiableArrayListOfSize renders
// RamUsageUtil.ramBytesUsedByUnmodifiableArrayListOfSize(int)
// (RamUsageUtil.java:81).
func RamBytesUsedByUnmodifiableArrayListOfSize(size int) int64 {
	return unmodifiableArrayListBaseRAMUsage + RamBytesUsedByObjectArrayOfLength(size)
}

// RamBytesUsedByObjectArrayOfLength renders
// RamUsageUtil.ramBytesUsedByObjectArrayOfLength(int) (RamUsageUtil.java:85).
func RamBytesUsedByObjectArrayOfLength(length int) int64 {
	return util.AlignObjectSize(int64(util.NumBytesArrayHeader) + int64(util.NumBytesObjectRef)*int64(length))
}
