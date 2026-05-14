// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package util

import (
	"fmt"
	"sort"
	"strings"
)

// Accountables is the Go port of org.apache.lucene.util.Accountables.
//
// It is a collection of helper functions for constructing nested resource
// descriptions and debugging RAM usage. Lucene exposes them as static
// methods on a final class with a private constructor; in Go they are
// exposed as package-level functions on the [util] package.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/Accountables.java

// Size constants mirroring org.apache.lucene.util.RamUsageEstimator.
const (
	oneKB int64 = 1024
	oneMB int64 = oneKB * oneKB
	oneGB int64 = oneKB * oneMB
)

// lineSeparator is the trailing terminator emitted between entries in
// [AccountableToString]. Lucene uses System.lineSeparator(); the Go port
// commits to "\n" so output is reproducible across platforms (Lucene's
// own tests run on Unix CI where the value is identical).
const lineSeparator = "\n"

// humanReadableUnits returns size in human-readable units (GB, MB, KB or
// bytes), mirroring org.apache.lucene.util.RamUsageEstimator#humanReadableUnits
// for a Locale.ROOT DecimalFormat("0.#").
//
// Output format matches the Java reference byte-for-byte: values are
// formatted with at most one fractional digit, trailing zero suppressed,
// using "." as decimal separator and a single space before the unit.
func humanReadableUnits(bytes int64) string {
	switch {
	case bytes/oneGB > 0:
		return formatDecimal1(float64(bytes)/float64(oneGB)) + " GB"
	case bytes/oneMB > 0:
		return formatDecimal1(float64(bytes)/float64(oneMB)) + " MB"
	case bytes/oneKB > 0:
		return formatDecimal1(float64(bytes)/float64(oneKB)) + " KB"
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// formatDecimal1 formats v with at most one fractional digit, suppressing
// the fractional component entirely when zero. It is the Go equivalent of
// java.text.DecimalFormat("0.#", DecimalFormatSymbols.getInstance(Locale.ROOT))
// applied to the Java float-cast ratio computed in RamUsageEstimator.
func formatDecimal1(v float64) string {
	// Java casts long/long to float (32-bit) prior to formatting; mimic
	// that precision so output is byte-identical to the reference.
	rounded := float64(float32(v))
	// One fractional digit, then strip trailing zero/period to match
	// DecimalFormat("0.#") which omits the fraction when it would be 0.
	s := fmt.Sprintf("%.1f", rounded)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// AccountableToString returns a String description of an Accountable and
// any nested resources. This is intended for development and debugging,
// and mirrors org.apache.lucene.util.Accountables#toString(Accountable).
//
// The format reproduces the Java reference output, including the four-space
// indentation per depth level, the "|-- " prefix for nested entries, the
// "<toString>: <human-size>" payload, and the platform-native line
// separator (\n on Unix, matching System.lineSeparator() on the same host).
func AccountableToString(a Accountable) string {
	var sb strings.Builder
	accountableToString(&sb, a, 0)
	return sb.String()
}

func accountableToString(dest *strings.Builder, a Accountable, depth int) {
	for i := 1; i < depth; i++ {
		dest.WriteString("    ")
	}
	if depth > 0 {
		dest.WriteString("|-- ")
	}
	dest.WriteString(accountableLabel(a))
	dest.WriteString(": ")
	dest.WriteString(humanReadableUnits(a.RamBytesUsed()))
	dest.WriteString(lineSeparator)
	for _, child := range GetChildResources(a) {
		accountableToString(dest, child, depth+1)
	}
}

// AccountableLabeler is implemented by Accountables that wish to provide
// a custom textual label for [AccountableToString] / [NamedAccountable],
// mirroring Java's Object#toString() override semantics. When absent the
// Go default "%v" rendering is used.
type AccountableLabeler interface {
	AccountableLabel() string
}

func accountableLabel(a Accountable) string {
	if l, ok := a.(AccountableLabeler); ok {
		return l.AccountableLabel()
	}
	return fmt.Sprintf("%v", a)
}

// NamedAccountable augments an existing accountable with the provided
// description, mirroring
// org.apache.lucene.util.Accountables#namedAccountable(String, Accountable).
//
// The resource description is constructed in the format:
//
//	description [toString()]
//
// This is a point-in-time, type-safe view: consumers cannot cast or
// manipulate the wrapped resource.
func NamedAccountable(description string, in Accountable) Accountable {
	return NamedAccountableWithChildren(
		description+" ["+accountableLabel(in)+"]",
		GetChildResources(in),
		in.RamBytesUsed(),
	)
}

// NamedAccountableBytes returns an [Accountable] with the provided
// description and bytes, no children. Mirrors the Java overload
// {@code Accountables.namedAccountable(String, long)}.
func NamedAccountableBytes(description string, bytes int64) Accountable {
	return NamedAccountableWithChildren(description, nil, bytes)
}

// NamedAccountableWithChildren returns an [Accountable] with the provided
// description, children and bytes. Mirrors the Java overload
// {@code Accountables.namedAccountable(String, Collection<Accountable>, long)}.
func NamedAccountableWithChildren(description string, children []Accountable, bytes int64) Accountable {
	return &namedAccountable{
		description: description,
		children:    children,
		bytes:       bytes,
	}
}

// NamedAccountables converts a map of resources to a sorted slice,
// mirroring org.apache.lucene.util.Accountables#namedAccountables.
//
// Each entry's description is "{prefix} '{key}' [toString()]" and the
// returned slice is sorted ascending by description (the Java reference
// uses String#compareTo, which is lexicographic on UTF-16 code units; for
// ASCII keys/prefixes this is identical to Go's < on strings).
func NamedAccountables(prefix string, in map[string]Accountable) []Accountable {
	resources := make([]Accountable, 0, len(in))
	for key, value := range in {
		resources = append(resources, NamedAccountable(prefix+" '"+key+"'", value))
	}
	sort.Slice(resources, func(i, j int) bool {
		return accountableLabel(resources[i]) < accountableLabel(resources[j])
	})
	return resources
}

// namedAccountable is the concrete implementation returned by the
// NamedAccountable* constructors. It is unexported because, like the
// anonymous inner class in Lucene, the only stable surface is the
// [Accountable] / [AccountableWithChildren] / [AccountableLabeler]
// interfaces it satisfies.
type namedAccountable struct {
	description string
	children    []Accountable
	bytes       int64
}

func (n *namedAccountable) RamBytesUsed() int64 { return n.bytes }

func (n *namedAccountable) GetChildResources() []Accountable { return n.children }

func (n *namedAccountable) AccountableLabel() string { return n.description }

func (n *namedAccountable) String() string { return n.description }
