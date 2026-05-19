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

package util

// SortableBytesRefArray is the contract for append-only BytesRef containers
// that can later be iterated in an order defined by a caller-supplied
// comparator.
//
// Port of org.apache.lucene.util.SortableBytesRefArray from Apache Lucene
// 10.4.0. The Java original is package-private; in Go we keep it exported
// so that other packages within Gocene can program against the abstraction
// (e.g. document.SortedSetDocValuesField builders).
//
// Deviation from the Java interface signature: Java declares
// iterator(Comparator<BytesRef>), but every concrete Gocene implementation
// (FixedLengthBytesRefArray and friends) already takes the richer
// BytesRefComparator that also exposes byte-level access for radix
// sorters. We surface the same type here so existing implementations
// satisfy the interface without an adapter shim. Passing nil yields
// insertion order, matching the established convention in the package.
type SortableBytesRefArray interface {
	// Append copies the given BytesRef into the array and returns the
	// index assigned to it. The returned index is monotonically
	// increasing and unique for each successful append.
	Append(bytes *BytesRef) int

	// Clear discards all previously appended values, releasing any
	// internal buffers the implementation deems appropriate. After
	// Clear, Size returns zero.
	Clear()

	// Size returns the number of values appended since the array was
	// created or last cleared.
	Size() int

	// Iterator returns a BytesRefIterator that yields the appended
	// values in the order defined by comp. When comp is nil,
	// implementations iterate in insertion order.
	Iterator(comp BytesRefComparator) BytesRefIterator
}
