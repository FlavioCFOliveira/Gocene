// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

// PagedGrowableWriter is a paged Mutable whose pages each grow their
// bitsPerValue independently. Use it instead of PackedLongValues only
// when you need random write-access.
//
// This is the Go port of org.apache.lucene.util.packed.PagedGrowableWriter
// in Apache Lucene 10.4.0.
type PagedGrowableWriter struct {
	*AbstractPagedMutable
	acceptableOverheadRatio float32
}

// NewPagedGrowableWriter creates a new PagedGrowableWriter for size
// values, pages of pageSize entries, starting at startBitsPerValue.
func NewPagedGrowableWriter(size int64, pageSize, startBitsPerValue int, acceptableOverheadRatio float32) (*PagedGrowableWriter, error) {
	return newPagedGrowableWriter(size, pageSize, startBitsPerValue, acceptableOverheadRatio, true)
}

func newPagedGrowableWriter(size int64, pageSize, startBitsPerValue int, acceptableOverheadRatio float32, fillPages bool) (*PagedGrowableWriter, error) {
	base, err := initAbstractPagedMutable(startBitsPerValue, size, pageSize)
	if err != nil {
		return nil, err
	}
	p := &PagedGrowableWriter{
		AbstractPagedMutable:    base,
		acceptableOverheadRatio: acceptableOverheadRatio,
	}
	base.newMutable = func(valueCount, bitsPerValue int) Mutable {
		return NewGrowableWriter(bitsPerValue, valueCount, acceptableOverheadRatio)
	}
	base.newUnfilledCopy = func(newSize int64) *AbstractPagedMutable {
		copy, _ := newPagedGrowableWriter(newSize, pageSize, p.bitsPerValue, acceptableOverheadRatio, false)
		return copy.AbstractPagedMutable
	}
	if fillPages {
		base.fillPages()
	}
	return p, nil
}

// AcceptableOverheadRatio returns the configured overhead ratio.
func (p *PagedGrowableWriter) AcceptableOverheadRatio() float32 { return p.acceptableOverheadRatio }
