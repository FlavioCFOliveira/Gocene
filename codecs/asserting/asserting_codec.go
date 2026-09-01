// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package asserting

import (
	"fmt"
)

// AssertingCodec provides utility assertions for codec components.
// In Lucene, this is used to verify thread-affinity. In Go, since goroutine
// IDs are not exposed, AssertThread is provided for API compatibility but
// is currently a no-op.
type AssertingCodec struct{}

// AssertThread verifies that the object is being consumed in the same
// thread/goroutine in which it was acquired.
func AssertThread(object string, creationThread interface{}) {
	// NOTE: Goroutine IDs are not exposed in Go.
	// This is a no-op in Gocene.
	_ = object
	_ = creationThread
}

// AssertState throws a panic if the condition is not met.
// This is used to implement the state machine checks in asserting formats.
func AssertState(condition bool, message string) {
	if !condition {
		panic(fmt.Sprintf("Assertion failed: %s", message))
	}
}
