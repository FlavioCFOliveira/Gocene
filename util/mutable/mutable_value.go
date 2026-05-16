// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.mutable.MutableValue from Apache
// Lucene 10.4.0 (Apache License 2.0).

// Package mutable provides the MutableValue family of value holders
// used by Lucene's group/sort facets and other components that need to
// pass a typed, comparable, optionally-absent scalar around without
// allocating per-document boxes.
//
// In the Java reference, MutableValue is an abstract class extended by
// MutableValueBool, MutableValueDate, MutableValueDouble,
// MutableValueFloat, MutableValueInt, MutableValueLong, and
// MutableValueStr. This package mirrors that layout via an interface
// (MutableValue) plus a tiny embeddable BaseMutableValue carrying the
// shared `exists` flag; each concrete type lives in its own file.
//
// Lucene exposes `exists` as a public field. Go does not allow a
// method and a field on the same struct to share a name, so the port
// exposes the flag only through the Exists() and SetExists() methods
// promoted from the embedded BaseMutableValue. Semantics survive
// intact: both forms read or mutate the same underlying bool.
//
// Per-type contracts (mirroring the Java source) require that callers
// keep the typed `value` field at its zero/empty value whenever Exists
// is false; the Java reference encodes this with assertions, and the
// Go port keeps the same rule in each concrete type's documentation.
package mutable

import "reflect"

// MutableValue is the Go analogue of Lucene's abstract MutableValue
// class. Every concrete implementation must:
//
//   - expose its typed `Value` field directly (mirroring Java field
//     access patterns);
//   - implement Copy (deep-copy of the source's payload), Duplicate
//     (factory of a new instance with the same content),
//     EqualsSameType (assumes both sides are the same concrete type),
//     CompareSameType (same), ToObject (the boxed Go value or nil when
//     absent), HashCode (Java-compatible integer hash), and String (the
//     stringified value, or "(null)" when absent).
//
// Polymorphic helpers Equals and CompareTo are exposed as package-level
// functions instead of methods on the interface to keep concrete types'
// implementations focused on the abstract-method surface of the Java
// base class.
type MutableValue interface {
	// Exists reports whether the value is present.
	Exists() bool

	// SetExists toggles the Exists flag. Callers must also clear the
	// concrete `Value` field whenever Exists becomes false; see the
	// per-type documentation.
	SetExists(bool)

	// Copy copies the payload (Value + Exists) from source. Panics if
	// source is not of the same concrete type as the receiver.
	Copy(source MutableValue)

	// Duplicate returns a fresh independent MutableValue of the same
	// concrete type with the same payload.
	Duplicate() MutableValue

	// EqualsSameType reports payload equality with other; other must be
	// of the same concrete type as the receiver. Panics otherwise.
	EqualsSameType(other MutableValue) bool

	// CompareSameType orders the receiver against other; other must be
	// of the same concrete type as the receiver. Panics otherwise.
	// Returns negative, zero, or positive following Java's
	// Comparable.compareTo contract.
	CompareSameType(other MutableValue) int

	// ToObject returns the boxed Go value when Exists is true, or nil
	// when Exists is false. Mirrors Lucene's MutableValue#toObject.
	ToObject() any

	// HashCode returns a Java-compatible 32-bit hash of the payload.
	// Mirrors MutableValue#hashCode in the Java reference.
	HashCode() int

	// String returns the stringified payload, or "(null)" when Exists is
	// false. Mirrors MutableValue#toString in the Java reference.
	String() string
}

// BaseMutableValue carries the shared `exists` flag and provides the
// Exists / SetExists accessors that satisfy the corresponding
// MutableValue interface methods. Concrete implementations embed this
// struct by value:
//
//	type MutableValueInt struct {
//	    BaseMutableValue
//	    Value int
//	}
//
// The flag is initialised to false by the zero value; the New* helpers
// in each concrete type set it to true to mirror Java's
// {@code public boolean exists = true;}.
type BaseMutableValue struct {
	// exists tracks whether the value is present. Private to avoid
	// shadowing the Exists() method on the embedded type.
	exists bool
}

// Exists returns the embedded flag. Satisfies MutableValue.Exists.
func (b *BaseMutableValue) Exists() bool { return b.exists }

// SetExists assigns the embedded flag. Satisfies MutableValue.SetExists.
func (b *BaseMutableValue) SetExists(v bool) { b.exists = v }

// sameType reports whether a and b have identical concrete types.
// Mirrors Java's `getClass() == other.getClass()` check.
func sameType(a, b MutableValue) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b)
}

// Equals returns true when a and b are of the same concrete type and
// their payloads compare equal via EqualsSameType. Mirrors Java's
// MutableValue#equals override (a.getClass() == b.getClass() &&
// a.equalsSameType(b)).
func Equals(a, b MutableValue) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if !sameType(a, b) {
		return false
	}
	return a.EqualsSameType(b)
}

// CompareTo orders a against b. Mirrors Java's MutableValue#compareTo:
// when both operands share a concrete type the result is delegated to
// CompareSameType; otherwise the result is the difference of the type
// hash codes (or, on hash collision, the lexicographic order of the
// fully-qualified Go type names). Returns 0 only when both operands
// share both type and payload.
func CompareTo(a, b MutableValue) int {
	ta := reflect.TypeOf(a)
	tb := reflect.TypeOf(b)
	if ta != tb {
		// Java compares Class<?>.hashCode() differences first; JVM
		// Class.hashCode is not reproducible across runs anyway, so
		// behavioural parity requires only that the ordering be a total
		// order, which the name-based fallback guarantees.
		ha := goTypeHash(ta)
		hb := goTypeHash(tb)
		if c := ha - hb; c != 0 {
			return c
		}
		na, nb := ta.String(), tb.String()
		switch {
		case na < nb:
			return -1
		case na > nb:
			return 1
		default:
			return 0
		}
	}
	return a.CompareSameType(b)
}

// goTypeHash returns a deterministic 32-bit hash for a reflect.Type. We
// derive it from the qualified type name; this is intentionally simple
// since the spec only requires a stable total order across runs of the
// same binary.
func goTypeHash(t reflect.Type) int {
	if t == nil {
		return 0
	}
	s := t.String()
	h := 0
	for i := 0; i < len(s); i++ {
		h = 31*h + int(s[i])
	}
	return h
}
