// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

// PagedMutable is a paged PackedInts.Mutable: it splits the value space
// across fixed-size pages that share the same bitsPerValue. Use it
// when you need a Mutable with size > 2^31 values.
//
// This is the Go port of org.apache.lucene.util.packed.PagedMutable
// in Apache Lucene 10.4.0.
type PagedMutable struct {
	*AbstractPagedMutable
	format Format
}

// NewPagedMutable creates a new PagedMutable for size values, using
// pages of pageSize entries at bitsPerValue width. The acceptable
// overhead ratio influences the chosen format (PACKED vs
// PACKED_SINGLE_BLOCK) on a per-page basis.
func NewPagedMutable(size int64, pageSize, bitsPerValue int, acceptableOverheadRatio float32) (*PagedMutable, error) {
	fab := FastestFormatAndBits(pageSize, bitsPerValue, acceptableOverheadRatio)
	return newPagedMutableFromFormat(size, pageSize, fab.BitsPerValue, fab.Format)
}

func newPagedMutableFromFormat(size int64, pageSize, bitsPerValue int, format Format) (*PagedMutable, error) {
	base, err := initAbstractPagedMutable(bitsPerValue, size, pageSize)
	if err != nil {
		return nil, err
	}
	p := &PagedMutable{AbstractPagedMutable: base, format: format}
	base.newMutable = func(valueCount, _ int) Mutable {
		// Per Lucene's PagedMutable.newMutable: always use the parent
		// bitsPerValue and chosen format for new pages.
		return GetMutableForFormat(valueCount, p.bitsPerValue, format)
	}
	base.newUnfilledCopy = func(newSize int64) *AbstractPagedMutable {
		copy, _ := newPagedMutableFromFormatUnfilled(newSize, pageSize, bitsPerValue, format)
		return copy.AbstractPagedMutable
	}
	base.fillPages()
	return p, nil
}

func newPagedMutableFromFormatUnfilled(size int64, pageSize, bitsPerValue int, format Format) (*PagedMutable, error) {
	base, err := initAbstractPagedMutable(bitsPerValue, size, pageSize)
	if err != nil {
		return nil, err
	}
	p := &PagedMutable{AbstractPagedMutable: base, format: format}
	base.newMutable = func(valueCount, _ int) Mutable {
		return GetMutableForFormat(valueCount, p.bitsPerValue, format)
	}
	base.newUnfilledCopy = func(newSize int64) *AbstractPagedMutable {
		copy, _ := newPagedMutableFromFormatUnfilled(newSize, pageSize, bitsPerValue, format)
		return copy.AbstractPagedMutable
	}
	return p, nil
}

// Format returns the format used by every page.
func (p *PagedMutable) Format() Format { return p.format }
