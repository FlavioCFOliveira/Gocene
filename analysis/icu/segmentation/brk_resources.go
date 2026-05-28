// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	_ "embed"
	"fmt"
)

// Compiled ICU RuleBasedBreakIterator blobs bundled as production resources.
//
// These are the exact binary artefacts Apache Lucene 10.4.0's analysis-icu
// module ships as JAR resources (org/apache/lucene/analysis/icu/segmentation/
// Default.brk and MyanmarSyllable.brk). They are byte-identical to the Lucene
// 10.4.0 reference (verified by TestEmbeddedBRKByteIdentity). Embedding them in
// the package — rather than reading them from testdata — lets the production
// tokenizer resolve the dictionaries without external files.
//
// The blobs are ICU4J-derived; see the NOTICE file for the ICU license and
// derivation statement. They are treated as read-only: callers parse them via
// LoadEmbeddedBRK and must not mutate the returned dictionary's backing data.
//
// NOTE: executing these compiled rules (the ICU RBBI engine) is tracked
// separately (rmp #4703). Until that lands, BRKDictionary.AsBreakIterator still
// falls back to the UAX#29 approximation; this file only ships and exposes the
// blobs so the engine has something to run.
//
//go:embed Default.brk
var defaultBRK []byte

//go:embed MyanmarSyllable.brk
var myanmarSyllableBRK []byte

// Names of the bundled ICU break-rule dictionaries.
const (
	// EmbeddedDefaultBRKName is the CJK / Thai / Lao / Khmer word-break dictionary.
	EmbeddedDefaultBRKName = "Default.brk"
	// EmbeddedMyanmarSyllableBRKName is the Myanmar syllable-break dictionary.
	EmbeddedMyanmarSyllableBRKName = "MyanmarSyllable.brk"
)

// LoadEmbeddedBRK parses the named bundled ICU .brk blob into a BRKDictionary.
// Valid names are EmbeddedDefaultBRKName and EmbeddedMyanmarSyllableBRKName.
func LoadEmbeddedBRK(name string) (*BRKDictionary, error) {
	switch name {
	case EmbeddedDefaultBRKName:
		return ParseBRKDictionary(defaultBRK)
	case EmbeddedMyanmarSyllableBRKName:
		return ParseBRKDictionary(myanmarSyllableBRK)
	default:
		return nil, fmt.Errorf("segmentation: no embedded .brk dictionary named %q", name)
	}
}
