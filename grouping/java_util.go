// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/mutable"
)

// This file has no counterpart in org.apache.lucene.search.grouping. It
// renders the three java.util facilities that the grouping classes rely on
// and that Go does not provide: the value-equality keying of HashMap and
// HashSet (java.lang.Object.equals/hashCode dispatch on the group value type)
// and the comparator-ordered NavigableSet of TreeSet.
//
// Everything here is unexported: no API is added to the package, and the
// observable behaviour of the ported classes is the Java one.

// groupKey renders the java.util.HashMap/HashSet key of a group value, i.e.
// the equivalence class induced by that value's equals/hashCode pair. Lucene
// instantiates the grouping generics with BytesRef, DoubleRange, LongRange
// and MutableValue; each of those overrides equals/hashCode, so the key is
// derived from the payload. Any other value keeps Go's own equality, which is
// identity for pointers exactly as java.lang.Object.equals is.
//
// A Java null group value maps to the nil key.
func groupKey(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case *util.BytesRef:
		if t == nil {
			return nil
		}
		return bytesRefKey(string(t.Bytes[t.Offset : t.Offset+t.Length]))
	case *DoubleRange:
		if t == nil {
			return nil
		}
		return doubleRangeKey{min: canonicalFloat64Bits(t.Min), max: canonicalFloat64Bits(t.Max)}
	case *LongRange:
		if t == nil {
			return nil
		}
		return longRangeKey{min: t.Min, max: t.Max}
	case mutable.MutableValue:
		if t == nil {
			return nil
		}
		return mutableValueKey{typ: fmt.Sprintf("%T", t), exists: t.Exists(), value: t.ToObject()}
	default:
		return v
	}
}

// bytesRefKey is the equivalence class of BytesRef.equals, which compares the
// [offset, offset+length) window byte by byte.
type bytesRefKey string

// doubleRangeKey is the equivalence class of DoubleRange.equals, which uses
// Double.compare on both bounds.
type doubleRangeKey struct {
	min uint64
	max uint64
}

// longRangeKey is the equivalence class of LongRange.equals.
type longRangeKey struct {
	min int64
	max int64
}

// mutableValueKey is the equivalence class of MutableValue.equals, which
// requires the same concrete class and the same payload.
type mutableValueKey struct {
	typ    string
	exists bool
	value  any
}

// canonicalFloat64Bits mirrors Double.doubleToLongBits: every NaN collapses
// onto the canonical NaN so that Double.compare(NaN, NaN) == 0 holds, while
// +0.0 and -0.0 keep distinct bit patterns.
func canonicalFloat64Bits(v float64) uint64 {
	if math.IsNaN(v) {
		return 0x7ff8000000000000
	}
	return math.Float64bits(v)
}

// groupMap renders java.util.HashMap<T, V> keyed by the group value.
type groupMap[T any, V any] struct {
	entries map[any]groupMapEntry[T, V]
}

// groupMapEntry keeps the original key alongside the value so that the map can
// hand Java's keySet()/values() back with the group values themselves.
type groupMapEntry[T any, V any] struct {
	key   T
	value V
}

// newGroupMap builds an empty map.
func newGroupMap[T any, V any]() *groupMap[T, V] {
	return &groupMap[T, V]{entries: make(map[any]groupMapEntry[T, V])}
}

// newGroupMapWithSize builds an empty map sized for expected entries, as
// HashMap.newHashMap(int) does.
func newGroupMapWithSize[T any, V any](expected int) *groupMap[T, V] {
	if expected < 0 {
		expected = 0
	}
	return &groupMap[T, V]{entries: make(map[any]groupMapEntry[T, V], expected)}
}

// get returns the value bound to key, and whether the key was present.
func (m *groupMap[T, V]) get(key T) (V, bool) {
	e, ok := m.entries[groupKey(key)]
	return e.value, ok
}

// put binds value to key.
func (m *groupMap[T, V]) put(key T, value V) {
	m.entries[groupKey(key)] = groupMapEntry[T, V]{key: key, value: value}
}

// remove drops key.
func (m *groupMap[T, V]) remove(key T) {
	delete(m.entries, groupKey(key))
}

// size returns the number of entries.
func (m *groupMap[T, V]) size() int { return len(m.entries) }

// values returns the values. The order is unspecified, as HashMap.values() is.
func (m *groupMap[T, V]) values() []V {
	out := make([]V, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, e.value)
	}
	return out
}

// groupSet renders java.util.HashSet<T> keyed by the group value.
type groupSet[T any] struct {
	entries map[any]T
}

// newGroupSet builds an empty set.
func newGroupSet[T any]() *groupSet[T] {
	return &groupSet[T]{entries: make(map[any]T)}
}

// contains reports whether value is a member.
func (s *groupSet[T]) contains(value T) bool {
	_, ok := s.entries[groupKey(value)]
	return ok
}

// add inserts value and reports whether the set changed.
func (s *groupSet[T]) add(value T) bool {
	k := groupKey(value)
	if _, ok := s.entries[k]; ok {
		return false
	}
	s.entries[k] = value
	return true
}

// addAll inserts every member of other.
func (s *groupSet[T]) addAll(other *groupSet[T]) {
	for k, v := range other.entries {
		if _, ok := s.entries[k]; !ok {
			s.entries[k] = v
		}
	}
}

// size returns the number of members.
func (s *groupSet[T]) size() int { return len(s.entries) }

// values returns the members. The order is unspecified, as HashSet is.
func (s *groupSet[T]) values() []T {
	out := make([]T, 0, len(s.entries))
	for _, v := range s.entries {
		out = append(out, v)
	}
	return out
}

// treeSet renders java.util.TreeSet<E>, the NavigableSet the grouping classes
// build over an explicit Comparator: membership is decided by the comparator
// returning zero, and iteration follows the comparator's order.
type treeSet[E any] struct {
	cmp   func(a, b E) int
	elems []E
}

// newTreeSet builds an empty set ordered by cmp.
func newTreeSet[E any](cmp func(a, b E) int) *treeSet[E] {
	return &treeSet[E]{cmp: cmp}
}

// search locates e, returning its index and whether it is present.
func (s *treeSet[E]) search(e E) (int, bool) {
	i := sort.Search(len(s.elems), func(i int) bool { return s.cmp(s.elems[i], e) >= 0 })
	if i < len(s.elems) && s.cmp(s.elems[i], e) == 0 {
		return i, true
	}
	return i, false
}

// add inserts e and reports whether the set changed.
func (s *treeSet[E]) add(e E) bool {
	i, found := s.search(e)
	if found {
		return false
	}
	s.elems = append(s.elems, e)
	copy(s.elems[i+1:], s.elems[i:])
	s.elems[i] = e
	return true
}

// addAll inserts every element of es.
func (s *treeSet[E]) addAll(es []E) {
	for _, e := range es {
		s.add(e)
	}
}

// remove drops e and reports whether the set changed.
func (s *treeSet[E]) remove(e E) bool {
	i, found := s.search(e)
	if !found {
		return false
	}
	s.elems = append(s.elems[:i], s.elems[i+1:]...)
	return true
}

// first returns the lowest element. The set must not be empty.
func (s *treeSet[E]) first() E { return s.elems[0] }

// last returns the highest element. The set must not be empty.
func (s *treeSet[E]) last() E { return s.elems[len(s.elems)-1] }

// pollFirst removes and returns the lowest element, reporting false when the
// set is empty, as TreeSet.pollFirst() returns null.
func (s *treeSet[E]) pollFirst() (E, bool) {
	var zero E
	if len(s.elems) == 0 {
		return zero, false
	}
	e := s.elems[0]
	s.elems = s.elems[1:]
	return e, true
}

// pollLast removes and returns the highest element, reporting false when the
// set is empty, as TreeSet.pollLast() returns null.
func (s *treeSet[E]) pollLast() (E, bool) {
	var zero E
	if len(s.elems) == 0 {
		return zero, false
	}
	e := s.elems[len(s.elems)-1]
	s.elems = s.elems[:len(s.elems)-1]
	return e, true
}

// higher returns the least element strictly greater than e, mirroring
// java.util.NavigableSet.higher(E); the second result is false when there is
// no such element, which is Java's null.
func (s *treeSet[E]) higher(e E) (E, bool) {
	var zero E
	i, found := s.search(e)
	if found {
		i++
	}
	if i >= len(s.elems) {
		return zero, false
	}
	return s.elems[i], true
}

// size returns the number of elements.
func (s *treeSet[E]) size() int { return len(s.elems) }

// isEmpty reports whether the set holds no elements.
func (s *treeSet[E]) isEmpty() bool { return len(s.elems) == 0 }

// values returns the elements in comparator order.
func (s *treeSet[E]) values() []E { return s.elems }

// javaHashCode renders java.lang.Object.hashCode() for a group value. Lucene
// instantiates the grouping generics with types that all override hashCode
// and that Gocene exposes as a HashCode method; anything else falls back on a
// hash of the value's equality key, which keeps the equals/hashCode contract.
func javaHashCode(v any) int {
	switch t := v.(type) {
	case nil:
		return 0
	case interface{ HashCode() int }:
		return t.HashCode()
	}
	key := groupKey(v)
	if key == nil {
		return 0
	}
	// 32-bit FNV-1a over the rendered key, consistent with groupKey and hence
	// with equals.
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for _, b := range []byte(fmt.Sprintf("%#v", key)) {
		h ^= uint32(b)
		h *= prime32
	}
	return int(int32(h))
}

// javaArraysToString renders java.util.Arrays.toString(Object[]).
func javaArraysToString(values []any) string {
	if values == nil {
		return "null"
	}
	out := "["
	for i, v := range values {
		if i > 0 {
			out += ", "
		}
		if v == nil {
			out += "null"
		} else {
			out += fmt.Sprint(v)
		}
	}
	return out + "]"
}

// javaFloatCompare mirrors java.lang.Float.compare(float, float): NaN is
// considered greater than every other value, including positive infinity, and
// -0.0f is considered less than 0.0f.
func javaFloatCompare(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	// Fall through to the raw-bit comparison Java uses for the -0.0f/0.0f and
	// NaN cases.
	ab := javaFloatToIntBits(a)
	bb := javaFloatToIntBits(b)
	switch {
	case ab == bb:
		return 0
	case ab < bb:
		return -1
	default:
		return 1
	}
}

// javaFloatToIntBits mirrors Float.floatToIntBits, which canonicalises every
// NaN onto 0x7fc00000.
func javaFloatToIntBits(v float32) int32 {
	if math.IsNaN(float64(v)) {
		return 0x7fc00000
	}
	return int32(math.Float32bits(v))
}
