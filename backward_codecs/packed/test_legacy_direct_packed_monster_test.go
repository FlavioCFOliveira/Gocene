// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/packed/TestLegacyDirectPacked.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled ("N-2 formats are only tested on
// nightly runs").
//
// Blocker: every test method writes with LegacyDirectWriter, which Gocene declares only as a placeholder struct, through EndiannessReverserUtil, which is not ported.

package packed

import "testing"

const TestLegacyDirectPackedBlocker = "requires org.apache.lucene.backward_codecs.packed.LegacyDirectWriter (Gocene has a Name/Version placeholder struct) and org.apache.lucene.backward_codecs.store.EndiannessReverserUtil (not ported)"

func TestLegacyDirectPacked_testSimple(t *testing.T) {
	t.Fatal(TestLegacyDirectPackedBlocker)
}

func TestLegacyDirectPacked_testNotEnoughValues(t *testing.T) {
	t.Fatal(TestLegacyDirectPackedBlocker)
}

func TestLegacyDirectPacked_testRandom(t *testing.T) {
	t.Fatal(TestLegacyDirectPackedBlocker)
}

func TestLegacyDirectPacked_testRandomWithOffset(t *testing.T) {
	t.Fatal(TestLegacyDirectPackedBlocker)
}
