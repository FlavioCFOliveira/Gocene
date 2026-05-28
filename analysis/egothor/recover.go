// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"runtime"
	"strings"
)

// recoverBounds is the deferred recovery used by the patch-application and
// trie-lookup routines that mirror Java code which deliberately swallows
// IndexOutOfBoundsException (and its StringIndexOutOfBoundsException /
// ArrayIndexOutOfBoundsException subclasses).
//
// It recovers only runtime out-of-range panics — Go's equivalent of those Java
// exceptions, whose messages contain "index out of range" or "slice bounds out
// of range" — and re-panics anything else (for example nil dereferences or
// runtime errors raised by the runtime for other reasons) so that genuine bugs
// keep their diagnostics instead of being silently suppressed.
//
// It must be invoked directly via defer (for example `defer recoverBounds()`)
// so that recover observes the panicking goroutine.
func recoverBounds() {
	r := recover()
	if r == nil {
		return
	}
	if re, ok := r.(runtime.Error); ok {
		msg := re.Error()
		if strings.Contains(msg, "index out of range") || strings.Contains(msg, "out of range") {
			// Swallow: mirrors Java's catch of IndexOutOfBoundsException.
			return
		}
	}
	panic(r)
}
