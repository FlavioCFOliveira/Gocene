// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene80

import (
	"fmt"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene80DocValuesConsumer writes doc values in Lucene 8.0 format.
//
// Port of org.apache.lucene.backward_codecs.lucene80.Lucene80DocValuesConsumer
// (Lucene 10.4.0).
//
// The constructor opens .dvd and .dvm files, writes the codec IndexHeader on
// both, and records maxDoc. Close writes the -1 EOF sentinel to .dvm and the
// codec Footer to both files.
//
// DEFERRED: AddNumericField, AddBinaryField, AddSortedField,
// AddSortedSetField, AddSortedNumericField — full per-field encoding requires
// LegacyDirectWriter, IndexedDISI, LegacyDirectMonotonicWriter, and LZ4
// compression, none of which are fully ported yet.
type Lucene80DocValuesConsumer struct {
	mode   Lucene80DVMode
	data   gstore.IndexOutput
	meta   gstore.IndexOutput
	maxDoc int
	closed bool
}

// NewLucene80DocValuesConsumer opens the data and meta files, writes codec
// headers, and returns a ready consumer.
//
// Port of Lucene80DocValuesConsumer(SegmentWriteState, …) (Lucene 10.4.0).
func NewLucene80DocValuesConsumer(
	state *codecs.SegmentWriteState,
	dataCodec, dataExtension, metaCodec, metaExtension string,
	mode Lucene80DVMode,
) (*Lucene80DocValuesConsumer, error) {
	seg := state.SegmentInfo.Name()
	suffix := state.SegmentSuffix
	id := state.SegmentInfo.GetID()

	c := &Lucene80DocValuesConsumer{
		mode:   mode,
		maxDoc: state.SegmentInfo.DocCount(),
	}

	// --- data file ---
	dataName := index.SegmentFileName(seg, suffix, dataExtension)
	dataOut, err := bcstore.CreateOutput(state.Directory, dataName, gstore.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("lucene80 dv consumer: create data %q: %w", dataName, err)
	}
	if err := codecs.WriteIndexHeader(dataOut, dataCodec, lucene80VersionCurrent, id, suffix); err != nil {
		_ = dataOut.Close()
		return nil, fmt.Errorf("lucene80 dv consumer: write data header: %w", err)
	}
	c.data = dataOut

	// --- meta file ---
	metaName := index.SegmentFileName(seg, suffix, metaExtension)
	metaOut, err := bcstore.CreateOutput(state.Directory, metaName, gstore.IOContextWrite)
	if err != nil {
		_ = dataOut.Close()
		return nil, fmt.Errorf("lucene80 dv consumer: create meta %q: %w", metaName, err)
	}
	if err := codecs.WriteIndexHeader(metaOut, metaCodec, lucene80VersionCurrent, id, suffix); err != nil {
		_ = dataOut.Close()
		_ = metaOut.Close()
		return nil, fmt.Errorf("lucene80 dv consumer: write meta header: %w", err)
	}
	c.meta = metaOut

	return c, nil
}

// AddNumericField writes a numeric doc values field.
//
// DEFERRED: requires LegacyDirectWriter and IndexedDISI.
func (c *Lucene80DocValuesConsumer) AddNumericField(_ *index.FieldInfo, _ codecs.NumericDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("lucene80 dv consumer: closed")
	}
	return fmt.Errorf("lucene80 dv consumer: AddNumericField not yet implemented")
}

// AddBinaryField writes a binary doc values field.
//
// DEFERRED: requires LZ4 compression and monotonic writer.
func (c *Lucene80DocValuesConsumer) AddBinaryField(_ *index.FieldInfo, _ codecs.BinaryDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("lucene80 dv consumer: closed")
	}
	return fmt.Errorf("lucene80 dv consumer: AddBinaryField not yet implemented")
}

// AddSortedField writes a sorted doc values field.
//
// DEFERRED: requires terms dict, monotonic writer, and IndexedDISI.
func (c *Lucene80DocValuesConsumer) AddSortedField(_ *index.FieldInfo, _ codecs.SortedDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("lucene80 dv consumer: closed")
	}
	return fmt.Errorf("lucene80 dv consumer: AddSortedField not yet implemented")
}

// AddSortedSetField writes a sorted-set doc values field.
//
// DEFERRED: requires terms dict, monotonic writer, and IndexedDISI.
func (c *Lucene80DocValuesConsumer) AddSortedSetField(_ *index.FieldInfo, _ codecs.SortedSetDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("lucene80 dv consumer: closed")
	}
	return fmt.Errorf("lucene80 dv consumer: AddSortedSetField not yet implemented")
}

// AddSortedNumericField writes a sorted-numeric doc values field.
//
// DEFERRED: requires LegacyDirectWriter and IndexedDISI.
func (c *Lucene80DocValuesConsumer) AddSortedNumericField(_ *index.FieldInfo, _ codecs.SortedNumericDocValuesIterator) error {
	if c.closed {
		return fmt.Errorf("lucene80 dv consumer: closed")
	}
	return fmt.Errorf("lucene80 dv consumer: AddSortedNumericField not yet implemented")
}

// Close writes the EOF sentinel and codec footers, then closes both files.
//
// Port of Lucene80DocValuesConsumer.close() (Lucene 10.4.0).
func (c *Lucene80DocValuesConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true

	var firstErr error
	setErr := func(err error) {
		if firstErr == nil && err != nil {
			firstErr = err
		}
	}

	if c.meta != nil {
		// EOF marker (-1 field number sentinel).
		if err := gstore.WriteInt32(c.meta, -1); err != nil {
			setErr(fmt.Errorf("lucene80 dv consumer: meta EOF sentinel: %w", err))
		} else {
			setErr(codecs.WriteFooter(c.meta))
		}
		setErr(c.meta.Close())
		c.meta = nil
	}
	if c.data != nil {
		setErr(codecs.WriteFooter(c.data))
		setErr(c.data.Close())
		c.data = nil
	}
	return firstErr
}

// compile-time assertion
var _ codecs.DocValuesConsumer = (*Lucene80DocValuesConsumer)(nil)
