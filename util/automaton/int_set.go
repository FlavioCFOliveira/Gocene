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

// IntSet is the abstract surface for an unordered set of int values used by
// the automaton determinization machinery. It mirrors the package-private
// abstract class org.apache.lucene.util.automaton.IntSet from Lucene 10.4.0.
//
// Implementations expose their values through a backing array of at least
// Size() elements; values are valid for indices [0, Size()). If the
// implementation is mutable, mutations are not guaranteed to be reflected in
// a previously returned array.
type IntSet interface {
	// GetArray returns an array representation of this set's values. The
	// returned slice is guaranteed to have len >= Size(); only the first
	// Size() entries are meaningful.
	GetArray() []int32

	// Size returns the number of valid values in this set. It is guaranteed
	// to be <= len(GetArray()).
	Size() int

	// LongHashCode returns a 64-bit hash of the set's contents, used by
	// IntSetEquals and by callers building secondary hash structures.
	LongHashCode() int64
}

// IntSetHashCode collapses the 64-bit hash exposed by an IntSet into a 32-bit
// value, mirroring Lucene's IntSet.hashCode() which delegates to
// Long.hashCode(longHashCode()).
func IntSetHashCode(s IntSet) int32 {
	h := s.LongHashCode()
	return int32(uint64(h) ^ (uint64(h) >> 32))
}

// IntSetEquals reports whether two IntSet instances contain the same values
// in the same order over their respective [0, Size()) prefixes. It mirrors
// Lucene's IntSet.equals(Object), which short-circuits on identity, then
// compares longHashCode() and the [0, size()) prefix of the backing arrays.
func IntSetEquals(a, b IntSet) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.LongHashCode() != b.LongHashCode() {
		return false
	}
	sa, sb := a.Size(), b.Size()
	if sa != sb {
		return false
	}
	aa, bb := a.GetArray(), b.GetArray()
	for i := 0; i < sa; i++ {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
