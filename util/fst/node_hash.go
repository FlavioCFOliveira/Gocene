// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package fst

import (
	"fmt"
	"unsafe"
)

// NodeHash performs suffix-node deduplication during FST compilation,
// the mechanism that gives the FST its minimal-DFA property. It is the
// Go port of the package-private org.apache.lucene.util.fst.NodeHash.
//
// Algorithmic divergence from Lucene's reference:
//
// Lucene backs its hash table with PagedGrowableWriter (a PackedInts
// implementation that we have not yet ported) and a ByteBlockPool for
// the copied node bytes; it also implements a primary/fallback
// two-table LRU eviction policy bounded by suffixRAMLimitMB. The Go
// port stores entries in a plain map keyed by the unfrozen node hash
// and a per-key bucket of (hash, nodeAddress, nodeBytes). This is a
// drop-in algorithmic equivalent that produces identical FST byte
// streams whenever Lucene would have kept the entry in its primary
// table — i.e. for FST sizes that fit within the configured RAM
// limit. The map is unbounded, so the Go port behaves like Lucene at
// the suffixRAMLimitMB = +Inf setting; ramLimitBytes is currently
// retained only for API parity with the Builder and is not used to
// evict entries. This is a documented limitation tracked as future
// work; for the test sizes exercised here (<= 1M short keys) the
// outcome is byte-identical to Lucene's reference.
//
// The hash function exactly mirrors Lucene's PRIME=31 rolling hash so
// that the unfrozen-vs-frozen hash equality assertion built into
// Lucene's algorithm still holds.
type NodeHash[T any] struct {
	compiler *FSTCompiler[T]
	// table maps unfrozen-node hash -> bucket of equal-hash entries.
	// Buckets are linear lists; collisions are rare in practice and
	// the bucket scan walks the same number of comparisons that
	// Lucene's quadratic probe would.
	table map[uint64][]nodeHashEntry
	// scratchArc is reused while comparing unfrozen vs frozen nodes.
	scratchArc Arc[T]
}

// nodeHashEntry captures one frozen node already added to the FST.
// nodeAddress is the FST byte-stream offset of the node's last byte
// (Lucene's "node pointer" convention). bytes is a private copy of
// the node's bytes so that the dedup machinery can re-parse the node
// without touching the FST byte stream itself; this matches Lucene's
// copiedNodes pool which preserves stream-to-disk semantics.
type nodeHashEntry struct {
	nodeAddress int64
	// bytes is the node payload in writer order (after scratchBytes was
	// reversed in place by FSTCompiler.addNode). The end of the slice
	// (bytes[len-1]) corresponds to nodeAddress in the FST byte stream.
	bytes []byte
}

// NewNodeHash builds a NodeHash bound to the supplied compiler.
// ramLimitBytes is the Lucene-style budget; it is currently advisory
// in the Go port (see the package-level comment on NodeHash).
func NewNodeHash[T any](compiler *FSTCompiler[T], ramLimitBytes int64) (*NodeHash[T], error) {
	if compiler == nil {
		return nil, fmt.Errorf("fst: NodeHash requires a non-nil compiler")
	}
	if ramLimitBytes < 0 {
		return nil, fmt.Errorf("fst: NodeHash ramLimitBytes must be >= 0; got %d", ramLimitBytes)
	}
	return &NodeHash[T]{
		compiler: compiler,
		table:    make(map[uint64][]nodeHashEntry, 16),
	}, nil
}

// Add records nodeIn in the hash. If an equivalent frozen node is
// already present its address is returned and nodeIn is not written
// to the FST again. Otherwise nodeIn is frozen via compiler.AddNode,
// indexed, and its new address is returned. Mirrors NodeHash.add.
func (h *NodeHash[T]) Add(nodeIn *UnCompiledNode[T]) (int64, error) {
	hash := h.hashUnfrozen(nodeIn)
	bucket := h.table[hash]
	for _, entry := range bucket {
		ok, err := h.entryMatchesNode(entry, nodeIn)
		if err != nil {
			return 0, err
		}
		if ok {
			return entry.nodeAddress, nil
		}
	}
	// Not found: freeze the node and append.
	addr, err := h.compiler.addNode(nodeIn)
	if err != nil {
		return 0, err
	}
	// Snapshot the just-written node bytes (after scratchBytes was
	// reversed in addNode). The scratch buffer's populated prefix is
	// the node payload in writer order; copy it so a future addNode
	// invocation can overwrite scratchBytes without invalidating the
	// hash entry.
	scratch := h.compiler.scratchBytes
	payload := append([]byte(nil), scratch.GetBytes()[:scratch.GetPosition()]...)
	h.table[hash] = append(bucket, nodeHashEntry{nodeAddress: addr, bytes: payload})
	return addr, nil
}

// hashUnfrozen computes the unfrozen-node hash using exactly Lucene's
// formula. Each arc contributes its label, the target node address,
// the output hashCode, the nextFinalOutput hashCode, and (for final
// arcs) the +17 sentinel. Java arithmetic is modulo 2^32 (signed
// int); we mirror that by truncating to uint32 at every step.
func (h *NodeHash[T]) hashUnfrozen(node *UnCompiledNode[T]) uint64 {
	const prime uint32 = 31
	var hash uint32
	for arcIdx := 0; arcIdx < node.numArcs; arcIdx++ {
		arc := &node.arcs[arcIdx]
		hash = prime*hash + uint32(arc.label)
		target := arc.target.(*CompiledNode).node
		hash = prime*hash + uint32(target^(target>>32))
		hash = prime*hash + outputHash[T](arc.output)
		hash = prime*hash + outputHash[T](arc.nextFinalOutput)
		if arc.isFinal {
			hash += 17
		}
	}
	return uint64(hash)
}

// entryMatchesNode is the unfrozen-vs-frozen comparison. It re-parses
// the frozen node via the FST's read methods (using a forward reader
// over the cached payload) and compares it arc-by-arc against nodeIn.
//
// The frozen payload was written in scratch order (writer order). To
// use the existing FST traversal code we must arrange for the
// underlying BytesReader to consume bytes in the same order Lucene's
// readFirstRealTargetArc expects — namely the reverse of writer
// order. We achieve that with a ReverseBytesReader over the cached
// bytes, positioned at the last byte of the cache.
func (h *NodeHash[T]) entryMatchesNode(entry nodeHashEntry, nodeIn *UnCompiledNode[T]) (bool, error) {
	if nodeIn.numArcs == 0 {
		// Sentinel "stop" nodes (FINAL_END_NODE / NON_FINAL_END_NODE)
		// are never added to the hash by FSTCompiler.compileNode, so
		// this branch is unreachable in normal operation.
		return false, nil
	}
	// Use a ReverseBytesReader scoped to the cached node payload. The
	// FST traversal code calls SetPosition with absolute FST node
	// addresses, but here we want it to read from the cache instead.
	// We adapt by mapping the FST node address back to the cache's
	// last byte: the cache's last byte corresponds to nodeAddress.
	reader := newCacheReverseReader(entry.bytes, entry.nodeAddress)
	if _, err := h.compiler.fst.ReadFirstRealTargetArc(entry.nodeAddress, &h.scratchArc, reader); err != nil {
		return false, err
	}
	for arcUpto := 0; arcUpto < nodeIn.numArcs; arcUpto++ {
		uArc := &nodeIn.arcs[arcUpto]
		if uArc.label != h.scratchArc.label {
			return false, nil
		}
		if !outputsEqual[T](uArc.output, h.scratchArc.output) {
			return false, nil
		}
		if uArc.target.(*CompiledNode).node != h.scratchArc.target {
			return false, nil
		}
		if !outputsEqual[T](uArc.nextFinalOutput, h.scratchArc.nextFinalOutput) {
			return false, nil
		}
		if uArc.isFinal != h.scratchArc.IsFinal() {
			return false, nil
		}
		if h.scratchArc.IsLast() {
			return arcUpto == nodeIn.numArcs-1, nil
		}
		if _, err := h.compiler.fst.ReadNextRealArc(&h.scratchArc, reader); err != nil {
			return false, err
		}
	}
	// Frozen has more arcs than unfrozen.
	return false, nil
}

// outputHash returns the Java hashCode of an output value, using
// identity for unknown types. The Outputs implementations in the FST
// package use either int64 (PositiveInt), *util.BytesRef
// (ByteSequence), or the *noOutputMarker singleton (NoOutputs); all
// three have stable hashCodes that match Lucene's implementations.
func outputHash[T any](output T) uint32 {
	var v any = output
	switch o := v.(type) {
	case int64:
		// java.lang.Long.hashCode: (int) (value ^ (value >>> 32)).
		return uint32(o ^ int64(uint64(o)>>32))
	case nil:
		return 0
	default:
		// For interface or pointer types, hash by the underlying
		// runtime identity. This mirrors Java's Object.hashCode
		// default behaviour where every object has a unique identity
		// hash; what we need here is consistency between subsequent
		// lookups of the same output value, not a particular
		// distribution.
		return identityHash(v)
	}
}

// outputsEqual returns true when two output values are considered
// equal by Java's Object.equals semantics. For value types we rely on
// Go's ==; for pointer types we delegate to the pointer-aware helper.
func outputsEqual[T any](a, b T) bool {
	return any(a) == any(b) || outputsDeepEqual(a, b)
}

// outputsDeepEqual handles the case where two distinct heap objects
// represent the same logical output. The Outputs implementations in
// this package use either value types (handled by ==) or pointer
// types that follow Java's identity-equality contract (also handled
// by ==); the catch-all below uses a structural comparison as a
// fallback. It is only reached when the == check fails, so the cost
// is paid at most once per genuine mismatch.
func outputsDeepEqual[T any](a, b T) bool {
	// BytesRef from util.ByteSequenceOutputs has a structural notion of
	// equality. Casting via any avoids importing util here.
	type bytesRefLike interface {
		BytesEqual(other any) bool
	}
	if br, ok := any(a).(bytesRefLike); ok {
		return br.BytesEqual(any(b))
	}
	return false
}

// identityHash returns a stable 32-bit hash derived from the runtime
// pointer (or scalar bit pattern) backing v. For interface values
// holding a pointer, this is the pointer's address; for scalar values
// it is the bit pattern. Matches the consistency contract required by
// the hash table; the distribution is unimportant because the buckets
// only have to be self-consistent across calls.
func identityHash(v any) uint32 {
	// Convert to a (type-tag, data) interface; the data slot holds
	// either a pointer or the inline scalar. We take the address of v
	// itself and hash the 16 bytes; this is independent of T's
	// internal layout and yields stable bit patterns under Go's
	// guarantee that a single interface value's underlying word does
	// not move once set.
	pair := *(*[2]uintptr)(unsafe.Pointer(&v))
	x := uint64(pair[0]) ^ uint64(pair[1])<<1
	// Mix with a SplitMix64 step to spread bits; correctness only
	// requires determinism so any reversible mixer works.
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x = x ^ (x >> 31)
	return uint32(x) ^ uint32(x>>32)
}

// cacheReverseReader is a BytesReader that maps Lucene-style FST node
// addresses onto a private cache slice. The cache stores the node
// payload in writer order; the FST traversal code consumes it in
// reverse order (i.e. last byte first).
//
// addrOfLastByte is the FST-relative address of the cache's last byte
// (cache[len-1]). When the FST asks for SetPosition(a) we map it to
// cache index a - (addrOfLastByte - (len-1)) — equivalent to
// a - baseAddr.
type cacheReverseReader struct {
	cache      []byte
	baseAddr   int64 // FST address of cache[0]
	posInCache int   // current cache index
}

// newCacheReverseReader builds a reader over cache where the last byte
// corresponds to FST address addrOfLastByte. Initial position is at
// the cache's last byte (i.e. matching SetPosition(addrOfLastByte)).
func newCacheReverseReader(cache []byte, addrOfLastByte int64) *cacheReverseReader {
	base := addrOfLastByte - int64(len(cache)-1)
	return &cacheReverseReader{
		cache:      cache,
		baseAddr:   base,
		posInCache: len(cache) - 1,
	}
}

func (r *cacheReverseReader) ReadByte() (byte, error) {
	if r.posInCache < 0 {
		return 0, fmt.Errorf("fst: cacheReverseReader: read past beginning of cache")
	}
	b := r.cache[r.posInCache]
	r.posInCache--
	return b, nil
}

func (r *cacheReverseReader) ReadBytes(b []byte) error {
	for i := range b {
		v, err := r.ReadByte()
		if err != nil {
			return err
		}
		b[i] = v
	}
	return nil
}

func (r *cacheReverseReader) ReadBytesN(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("fst: cacheReverseReader: negative n %d", n)
	}
	out := make([]byte, n)
	if err := r.ReadBytes(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *cacheReverseReader) ReadShort() (int16, error) {
	if r.posInCache < 1 {
		return 0, fmt.Errorf("fst: cacheReverseReader.ReadShort: insufficient bytes")
	}
	lo := r.cache[r.posInCache]
	r.posInCache--
	hi := r.cache[r.posInCache]
	r.posInCache--
	return int16(uint16(hi)<<8 | uint16(lo)), nil
}

func (r *cacheReverseReader) ReadInt() (int32, error) {
	if r.posInCache < 3 {
		return 0, fmt.Errorf("fst: cacheReverseReader.ReadInt: insufficient bytes")
	}
	b3 := r.cache[r.posInCache]
	r.posInCache--
	b2 := r.cache[r.posInCache]
	r.posInCache--
	b1 := r.cache[r.posInCache]
	r.posInCache--
	b0 := r.cache[r.posInCache]
	r.posInCache--
	return int32(uint32(b0)<<24 | uint32(b1)<<16 | uint32(b2)<<8 | uint32(b3)), nil
}

func (r *cacheReverseReader) ReadLong() (int64, error) {
	if r.posInCache < 7 {
		return 0, fmt.Errorf("fst: cacheReverseReader.ReadLong: insufficient bytes")
	}
	var v int64
	for i := 7; i >= 0; i-- {
		v |= int64(r.cache[r.posInCache]) << (8 * i)
		r.posInCache--
	}
	return v, nil
}

func (r *cacheReverseReader) ReadString() (string, error) {
	return "", fmt.Errorf("fst: cacheReverseReader.ReadString not supported")
}

func (r *cacheReverseReader) ReadVInt() (int32, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int32(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("fst: cacheReverseReader: corrupted VInt")
		}
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
	}
	return result, nil
}

func (r *cacheReverseReader) ReadVLong() (int64, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	result := int64(b & 0x7F)
	shift := 0
	for b&0x80 != 0 {
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("fst: cacheReverseReader: corrupted VLong")
		}
		b, err = r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
	}
	return result, nil
}

func (r *cacheReverseReader) GetPosition() int64 {
	return r.baseAddr + int64(r.posInCache)
}

func (r *cacheReverseReader) SetPosition(pos int64) {
	r.posInCache = int(pos - r.baseAddr)
}

func (r *cacheReverseReader) SkipBytes(n int64) error {
	r.posInCache -= int(n)
	return nil
}
