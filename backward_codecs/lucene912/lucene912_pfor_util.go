// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// pforUtil encodes blocks of 128 small positive integers using PFOR (Patched
// Frame Of Reference) encoding.  It is the direct Go port of
// org.apache.lucene.backward_codecs.lucene912.PForUtil (Lucene 10.4.0).
type pforUtil struct {
	fu forUtil
}

//lint:ignore U1000 write-path constant; used by pforUtil.encode and forDeltaUtil.encodeDeltas (PostingsWriter sprint).
const pforMaxExceptions = 7

// pforUtilAllEqual reports whether all BlockSize values in l are equal to l[0].
//
//lint:ignore U1000 write-path helper; used by pforUtil.encode and forDeltaUtil.encodeDeltas (PostingsWriter sprint).
func pforUtilAllEqual(l []int64) bool {
	for i := 1; i < BlockSize; i++ {
		if l[i] != l[0] {
			return false
		}
	}
	return true
}

// encode encodes 128 integers from longs into out.
//
//lint:ignore U1000 write-path entry point; called by Lucene912PostingsWriter (PostingsWriter sprint).
func (p *pforUtil) encode(longs []int64, out store.DataOutput) error {
	// Find the top (pforMaxExceptions+1) values using a simple min-heap
	// equivalent (Java uses LongHeap; we replicate the same logic inline).
	top := make([]int64, pforMaxExceptions+1)
	copy(top, longs[:pforMaxExceptions+1])
	// Build min-heap.
	for i := len(top)/2 - 1; i >= 0; i-- {
		pforSiftDown(top, i)
	}
	topValue := top[0] // heap minimum = smallest of top-(k+1)
	for i := pforMaxExceptions + 1; i < BlockSize; i++ {
		if longs[i] > topValue {
			top[0] = longs[i]
			pforSiftDown(top, 0)
			topValue = top[0]
		}
	}
	// max = largest among the top values.
	var max int64
	for _, v := range top {
		if v > max {
			max = v
		}
	}

	maxBitsRequired := bitsRequired(max)
	patchedBitsRequired := maxBitsRequired - 8
	if topValueBits := bitsRequired(topValue); topValueBits > patchedBitsRequired {
		patchedBitsRequired = topValueBits
	}
	if patchedBitsRequired < 1 {
		patchedBitsRequired = 1
	}

	numExceptions := 0
	maxUnpatchedValue := (int64(1) << uint(patchedBitsRequired)) - 1
	for i := 0; i < len(top); i++ {
		if top[i] > maxUnpatchedValue {
			numExceptions++
		}
	}

	exceptions := make([]byte, numExceptions*2)
	if numExceptions > 0 {
		exceptionCount := 0
		for i := 0; i < BlockSize; i++ {
			if longs[i] > maxUnpatchedValue {
				exceptions[exceptionCount*2] = byte(i)
				exceptions[exceptionCount*2+1] = byte(int64(uint64(longs[i]) >> uint(patchedBitsRequired)))
				longs[i] &= maxUnpatchedValue
				exceptionCount++
			}
		}
	}

	if pforUtilAllEqual(longs) && maxBitsRequired <= 8 {
		for i := 0; i < numExceptions; i++ {
			exceptions[2*i+1] = byte(int64(exceptions[2*i+1]) << uint(patchedBitsRequired))
		}
		if err := out.WriteByte(byte(numExceptions << 5)); err != nil {
			return err
		}
		if err2 := store.WriteVLong(out, longs[0]); err2 != nil {
			return err2
		}
	} else {
		token := (numExceptions << 5) | patchedBitsRequired
		if err := out.WriteByte(byte(token)); err != nil {
			return err
		}
		if err := p.fu.encode(longs, patchedBitsRequired, out); err != nil {
			return err
		}
	}
	return out.WriteBytes(exceptions)
}

// decode decodes 128 integers into longs.
func (p *pforUtil) decode(in store.IndexInput, longs []int64) error {
	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(tokenByte) & 0xFF
	bitsPerValue := token & 0x1f
	if bitsPerValue == 0 {
		v, err := store.ReadVLong(in)
		if err != nil {
			return err
		}
		for i := 0; i < BlockSize; i++ {
			longs[i] = v
		}
	} else {
		if err := p.fu.decode(bitsPerValue, in, longs); err != nil {
			return err
		}
	}
	numExceptions := token >> 5
	for i := 0; i < numExceptions; i++ {
		idxByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		patchByte, err := in.ReadByte()
		if err != nil {
			return err
		}
		longs[int(idxByte)&0xFF] |= int64(patchByte&0xFF) << uint(bitsPerValue)
	}
	return nil
}

// skip skips 128 integers.
func pforUtilSkip(in store.IndexInput) error {
	tokenByte, err := in.ReadByte()
	if err != nil {
		return err
	}
	token := int(tokenByte) & 0xFF
	bitsPerValue := token & 0x1f
	numExceptions := token >> 5
	if bitsPerValue == 0 {
		if _, err := store.ReadVLong(in); err != nil {
			return err
		}
		return in.SetPosition(in.GetFilePointer() + int64(numExceptions*2))
	}
	return in.SetPosition(in.GetFilePointer() + int64(forUtilNumBytes(bitsPerValue)+numExceptions*2))
}

// ---------- helper: min-heap sift-down ----------

//lint:ignore U1000 write-path helper; used by pforUtil.encode (PostingsWriter sprint).
func pforSiftDown(h []int64, i int) {
	n := len(h)
	for {
		smallest := i
		l := 2*i + 1
		r := 2*i + 2
		if l < n && h[l] < h[smallest] {
			smallest = l
		}
		if r < n && h[r] < h[smallest] {
			smallest = r
		}
		if smallest == i {
			break
		}
		h[i], h[smallest] = h[smallest], h[i]
		i = smallest
	}
}
