// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/packed/TestLegacyDirectMonotonic.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled ("N-2 formats are only tested on
// nightly runs").
//
// Blocker: every test method writes with LegacyDirectMonotonicWriter, which Gocene declares only as a placeholder struct, through EndiannessReverserUtil, which is not ported.

package packed

import "testing"

const TestLegacyDirectMonotonicBlocker = "requires org.apache.lucene.backward_codecs.packed.LegacyDirectMonotonicWriter (Gocene has a Name/Version placeholder struct) and org.apache.lucene.backward_codecs.store.EndiannessReverserUtil (not ported)"

func TestLegacyDirectMonotonic_testValidation(t *testing.T) {
	t.Fatal(TestLegacyDirectMonotonicBlocker)
}

func TestLegacyDirectMonotonic_testEmpty(t *testing.T) {
	t.Fatal(TestLegacyDirectMonotonicBlocker)
}

func TestLegacyDirectMonotonic_testSimple(t *testing.T) {
	t.Fatal(TestLegacyDirectMonotonicBlocker)
}

func TestLegacyDirectMonotonic_testConstantSlope(t *testing.T) {
	t.Fatal(TestLegacyDirectMonotonicBlocker)
}

func TestLegacyDirectMonotonic_testRandom(t *testing.T) {
	t.Fatal(TestLegacyDirectMonotonicBlocker)
}

func TestLegacyDirectMonotonic_testMonotonicBinarySearch(t *testing.T) {
	t.Fatal(TestLegacyDirectMonotonicBlocker)
}

func TestLegacyDirectMonotonic_testMonotonicBinarySearchRandom(t *testing.T) {
	t.Fatal(TestLegacyDirectMonotonicBlocker)
}
