// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package packed provides Go-side helpers for the PackedInts byte-level
// compatibility fixtures registered in Tools/lucene-fixtures.
//
// Three scenarios are defined:
//   - "packed-ints-packed64":   Packed64 / Packed64SingleBlock raw payloads
//   - "block-packed-writer":    BlockPackedWriter + MonotonicBlockPackedWriter
//   - "direct-monotonic":       DirectMonotonicWriter encoded meta+data streams
//
// Each scenario is accompanied by a Java-side CorpusScenario that uses
// Lucene 10.4.0 to produce the reference bytes, and Go-side tests that
// re-produce the same bytes via Gocene and assert byte-equality.
package packed

// Scenario names for the Java harness.
const (
	ScenarioPacked64          = "packed-ints-packed64"
	ScenarioBlockPackedWriter = "block-packed-writer"
	ScenarioDirectMonotonic   = "direct-monotonic"
)
