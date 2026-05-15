// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.ByteRunnable from Apache Lucene
// 10.4.0 (Apache License 2.0).

package automaton

// ByteRunnable is the Go analogue of Lucene's ByteRunnable interface: any
// automaton-like structure that can step over UTF-8 bytes.
type ByteRunnable interface {
	// Step returns the new state after reading byte c from state, or -1.
	Step(state, c int) int
	// IsAccept reports whether state is accepting.
	IsAccept(state int) bool
	// GetSize returns the number of states (or a logical upper bound for NFAs).
	GetSize() int
	// Run reports whether the bytes in s[offset:offset+length] are accepted.
	Run(s []byte, offset, length int) bool
}

// RunBytes is a convenience helper providing the default Run implementation
// for ByteRunnable adapters that only override Step/IsAccept/GetSize.
func RunBytes(r ByteRunnable, s []byte, offset, length int) bool {
	p := 0
	limit := offset + length
	for i := offset; i < limit; i++ {
		p = r.Step(p, int(s[i])&0xFF)
		if p == -1 {
			return false
		}
	}
	return r.IsAccept(p)
}
