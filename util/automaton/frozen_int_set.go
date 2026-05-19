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

package automaton

import (
	"fmt"
	"strings"
)

// FrozenIntSet mirrors the package-private final class
// org.apache.lucene.util.automaton.FrozenIntSet from Lucene 10.4.0.
//
// It is an immutable snapshot of an IntSet, produced primarily by
// StateSet.Freeze, carrying:
//   - the sorted state values,
//   - a precomputed 64-bit hash code (mirroring StateSet.longHashCode), and
//   - the associated dstate identifier used by the determinization machinery
//     to look up the corresponding compiled DFA state.
//
// Instances are read-only after construction; the Values slice is owned by
// the FrozenIntSet and must not be mutated by callers.
//
// Lucene's type is package-private; Gocene exports it because the
// determinization machinery scheduled for later sprints needs to share
// frozen snapshots across files within the package, and consumers of
// StateSet.Freeze already need to read the associated State identifier.
type FrozenIntSet struct {
	// Values holds the snapshot's contents, sorted ascending. The slice is
	// shared with the caller of NewFrozenIntSet; the FrozenIntSet does not
	// copy it.
	Values []int32
	// State is the dstate identifier associated with this snapshot, mirroring
	// the int state field on Lucene's FrozenIntSet.
	State int32
	// HashCode is the precomputed 64-bit hash of Values, mirroring the long
	// hashCode field on Lucene's FrozenIntSet.
	HashCode int64
}

// NewFrozenIntSet returns a FrozenIntSet wrapping the given values, hash and
// associated state. It mirrors the FrozenIntSet(int[], long, int)
// constructor in Lucene 10.4.0: no copy is made of values, and the hash is
// trusted as-is (callers are expected to pass the longHashCode of the source
// IntSet).
func NewFrozenIntSet(values []int32, hashCode int64, state int32) *FrozenIntSet {
	return &FrozenIntSet{Values: values, State: state, HashCode: hashCode}
}

// GetArray returns the snapshot's backing array. It mirrors
// FrozenIntSet.getArray() and returns the slice unchanged (no copy).
func (f *FrozenIntSet) GetArray() []int32 { return f.Values }

// Size returns the number of values in the snapshot. It mirrors
// FrozenIntSet.size(), which is values.length.
func (f *FrozenIntSet) Size() int { return len(f.Values) }

// LongHashCode returns the precomputed 64-bit hash. It mirrors
// FrozenIntSet.longHashCode().
func (f *FrozenIntSet) LongHashCode() int64 { return f.HashCode }

// String returns a Java-style array rendering of the snapshot's values,
// mirroring FrozenIntSet.toString(), which delegates to Arrays.toString.
// The format matches Java exactly: "[]", "[7]", or "[1, 2, 3]".
func (f *FrozenIntSet) String() string {
	if len(f.Values) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range f.Values {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%d", v)
	}
	b.WriteByte(']')
	return b.String()
}
