// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

// AffixKind distinguishes prefixes from suffixes.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.AffixKind from Apache Lucene 10.4.0.
type AffixKind int

const (
	// AffixKindPrefix is a PREFIX affix kind.
	AffixKindPrefix AffixKind = iota
	// AffixKindSuffix is a SUFFIX affix kind.
	AffixKindSuffix
)

func (k AffixKind) String() string {
	switch k {
	case AffixKindPrefix:
		return "PREFIX"
	case AffixKindSuffix:
		return "SUFFIX"
	default:
		return "UNKNOWN"
	}
}
