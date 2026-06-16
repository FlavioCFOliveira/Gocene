// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

package bkd

import (
	"testing"
)

// TestBKD_RandomBinaryBig ports testRandomBinaryBig (@Nightly in Java):
// doTestRandomBinary(200000). Built only when the gocene_monsters build tag is
// set (equivalent to GOCENE_RUN_MONSTERS=1 in CI), because it allocates a very
// large BKD tree.
func TestBKD_RandomBinaryBig(t *testing.T) {
	rng := verifyRNG(t)
	doTestRandomBinary(t, rng, 200000)
}
