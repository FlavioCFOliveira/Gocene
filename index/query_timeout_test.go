// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

type fixedQueryTimeout struct{ exit bool }

func (f fixedQueryTimeout) ShouldExit() bool { return f.exit }

func TestQueryTimeout_InterfaceContract(t *testing.T) {
	var qt QueryTimeout = fixedQueryTimeout{exit: false}
	if qt.ShouldExit() {
		t.Errorf("ShouldExit() = true, want false")
	}
	qt = fixedQueryTimeout{exit: true}
	if !qt.ShouldExit() {
		t.Errorf("ShouldExit() = false, want true")
	}
}
