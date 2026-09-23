// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"testing"
)

// Java assertions.
//
// Lucene relies on Java `assert` statements as internal consistency checks.
// The JVM evaluates them only when assertions are enabled (`-ea`), and the
// Lucene test framework always runs with `-ea`; production JVMs normally run
// without it. Go has no assertion switch, so Gocene maps the JVM setting to
// the kind of binary that is running: assertions are enabled inside a test
// binary built by `go test` and disabled everywhere else.
//
// A Java statement `assert cond : detail;` is ported as
//
//	if util.AssertsEnabled() && !(cond) {
//		panic(util.NewAssertionError(detail))
//	}
//
// The condition sits behind AssertsEnabled so that, as on a JVM without
// `-ea`, it is never evaluated in production. The panic mirrors the unchecked
// java.lang.AssertionError.

// assertsEnabled is fixed at program start: testing.Testing reports whether
// the binary was built by `go test`.
var assertsEnabled = testing.Testing()

// AssertsEnabled reports whether Java-style assertions are evaluated. It is
// true only inside a `go test` binary, which mirrors Lucene's tests running
// with `-ea`.
func AssertsEnabled() bool {
	return assertsEnabled
}

// AssertionError is the Go rendering of java.lang.AssertionError. It is the
// value passed to panic when an enabled assertion fails.
type AssertionError struct {
	// Detail is the optional detail expression of the Java assert statement
	// (the part after the colon); nil when the statement has none.
	Detail any
}

// NewAssertionError returns an AssertionError carrying detail, which may be
// nil, matching `new AssertionError(detail)`.
func NewAssertionError(detail any) *AssertionError {
	return &AssertionError{Detail: detail}
}

// Error returns the message Java's AssertionError would expose: the string
// form of the detail, or the class name alone when there is no detail.
func (e *AssertionError) Error() string {
	if e.Detail == nil {
		return "java.lang.AssertionError"
	}
	return fmt.Sprintf("java.lang.AssertionError: %v", e.Detail)
}
