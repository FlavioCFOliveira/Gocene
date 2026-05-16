// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// ReadAdvice is the Go port of org.apache.lucene.store.ReadAdvice.
//
// It expresses the read access pattern hint that the system can use to
// optimise paging behaviour (analogous to posix_madvise / fadvise on Linux).
//
// Unlike the hints defined in the FileOpenHint family, ReadAdvice is a
// stand-alone enum and is *not* a FileOpenHint: it is used directly by
// MMapDirectory and friends to drive page-cache advisories.
type ReadAdvice uint8

const (
	// ReadAdviceNormal indicates normal behaviour: data is expected to be read
	// mostly sequentially and the system is expected to cache the hottest
	// pages.
	ReadAdviceNormal ReadAdvice = iota
	// ReadAdviceRandom indicates data will be accessed in random-access
	// fashion, either by seeking and reading relatively short sequences or by
	// reading through RandomAccessInput in random order.
	ReadAdviceRandom
	// ReadAdviceSequential indicates data will be read sequentially with very
	// little seeking. The system may read ahead aggressively and free pages
	// soon after they are accessed.
	ReadAdviceSequential
)

// String returns the Lucene-equivalent constant name for the advice.
func (a ReadAdvice) String() string {
	switch a {
	case ReadAdviceNormal:
		return "NORMAL"
	case ReadAdviceRandom:
		return "RANDOM"
	case ReadAdviceSequential:
		return "SEQUENTIAL"
	default:
		return "UNKNOWN"
	}
}
