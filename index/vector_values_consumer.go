// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrVectorValuesConsumerNoKnnFormat is returned by VectorValuesConsumer.AddField
// when AddField is invoked before a KnnVectorsFormat has been wired in. It
// mirrors the IllegalStateException emitted by the Java reference when the
// codec does not provide a KnnVectorsFormat for an indexed vector field.
var ErrVectorValuesConsumerNoKnnFormat = errors.New("index: VectorValuesConsumer: field was indexed as vectors but codec does not support vectors")

// VectorValuesConsumer streams vector values for indexing to the given
// codec's KnnVectorsWriter. The codec's vectors writer is responsible for
// buffering and processing vectors.
//
// This is the Go port of org.apache.lucene.index.VectorValuesConsumer from
// Apache Lucene 10.4.0. The Java class is package-private and so is the
// Go type (lower-case identifier mirrors the original visibility).
//
// History: before rmp #4707 this consumer talked to a narrow
// KnnVectorsConsumerWriter abstraction backed by an index-side
// KnnVectorsFormatFactory shim. T4707 lifted the wide
// spi.KnnVectorsFormat / spi.KnnVectorsWriter contracts into the SPI,
// dropped the narrow shim, and rewired this consumer onto the wide
// writer directly — the same path Lucene uses (codec.knnVectorsFormat()
// .fieldsWriter(state)).
//
// Deviations from Lucene 10.4.0:
//   - The accountable handle defaults to a zero-bytes Accountable instead
//     of Accountable.NULL_ACCOUNTABLE (Gocene's util.Accountable contract
//     has no shared NULL singleton).
//   - SetKnnVectorsFormat is preserved as a test/integration override so
//     consumers can swap in a custom KnnVectorsFormat without having to
//     replace the entire Codec. The Java reference always resolves
//     through codec.knnVectorsFormat(); when this consumer is wired with
//     a non-nil Codec the override is consulted first, falling back to
//     codec.KnnVectorsFormat() when no override is set.
type vectorValuesConsumer struct {
	codec      Codec
	directory  store.Directory
	info       *SegmentInfo
	infoStream util.InfoStream

	// knnFormat is an optional explicit KnnVectorsFormat override; when
	// nil the consumer falls back to codec.KnnVectorsFormat(). Mirrors
	// the test-only injection path used by SortingStoredFieldsConsumer.
	knnFormat spi.KnnVectorsFormat

	writer spi.KnnVectorsWriter
}

// newVectorValuesConsumer mirrors the Java constructor
// VectorValuesConsumer(Codec, Directory, SegmentInfo, InfoStream).
//
// A KnnVectorsFormat must be available via either SetKnnVectorsFormat or
// the supplied Codec before the first call to AddField; otherwise
// AddField returns ErrVectorValuesConsumerNoKnnFormat (mirroring the
// IllegalStateException path in Lucene when codec.knnVectorsFormat() is
// null).
func newVectorValuesConsumer(codec Codec, directory store.Directory, info *SegmentInfo, infoStream util.InfoStream) *vectorValuesConsumer {
	return &vectorValuesConsumer{
		codec:      codec,
		directory:  directory,
		info:       info,
		infoStream: infoStream,
	}
}

// setKnnVectorsFormat injects the KnnVectorsFormat used to construct the
// per-segment KnnVectorsWriter. Provides the test/integration override
// described on vectorValuesConsumer; production paths normally rely on
// the Codec instead. Safe to call only before AddField.
func (c *vectorValuesConsumer) setKnnVectorsFormat(f spi.KnnVectorsFormat) {
	c.knnFormat = f
}

// initKnnVectorsWriter lazily constructs the codec's KnnVectorsWriter on
// the first AddField call. Mirrors initKnnVectorsWriter in Lucene.
//
// Resolution order:
//  1. The format injected via setKnnVectorsFormat (explicit override,
//     used by tests and custom pipelines).
//  2. codec.KnnVectorsFormat() when a Codec was supplied to
//     newVectorValuesConsumer. This mirrors the Java path where
//     VectorValuesConsumer.initKnnVectorsWriter calls
//     codec.knnVectorsFormat() directly.
//  3. If neither source yields a non-nil format, returns
//     ErrVectorValuesConsumerNoKnnFormat, matching the Java
//     IllegalStateException path when codec.knnVectorsFormat() is null.
func (c *vectorValuesConsumer) initKnnVectorsWriter(fieldName string) error {
	if c.writer != nil {
		return nil
	}
	resolved := c.knnFormat
	if resolved == nil && c.codec != nil {
		resolved = c.codec.KnnVectorsFormat()
	}
	if resolved == nil {
		return fmt.Errorf("%w (field=%q)", ErrVectorValuesConsumerNoKnnFormat, fieldName)
	}
	c.knnFormat = resolved // cache for subsequent AddField calls
	// Mirrors the Java construction of a SegmentWriteState seeded with
	// infoStream, directory, segmentInfo, null fieldInfos, null suffix
	// and IOContext.DEFAULT.
	state := &SegmentWriteState{
		Directory:     c.directory,
		SegmentInfo:   c.info,
		FieldInfos:    nil,
		SegmentSuffix: "",
	}
	w, err := c.knnFormat.FieldsWriter(state)
	if err != nil {
		return fmt.Errorf("index: VectorValuesConsumer: open knn writer for field %q: %w", fieldName, err)
	}
	c.writer = w
	return nil
}

// AddField registers fieldInfo with the codec's KnnVectorsWriter and
// returns the per-field writer as a KnnFieldVectorsWriterHandle. The
// underlying spi.KnnFieldVectorsWriter already exposes the wide
// AddValue(int, any) shape, so the indexing chain can stream vector
// values through it without further adaptation.
//
// Mirrors VectorValuesConsumer.addField(FieldInfo).
func (c *vectorValuesConsumer) AddField(fieldInfo *FieldInfo) (KnnFieldVectorsWriterHandle, error) {
	if fieldInfo == nil {
		return nil, errors.New("index: VectorValuesConsumer.AddField requires a non-nil FieldInfo")
	}
	if err := c.initKnnVectorsWriter(fieldInfo.Name()); err != nil {
		return nil, err
	}
	w, err := c.writer.AddField(fieldInfo)
	if err != nil {
		return nil, fmt.Errorf("index: VectorValuesConsumer.AddField %q: %w", fieldInfo.Name(), err)
	}
	return w, nil
}

// Flush writes every buffered vector field. It mirrors the try/finally
// chain in the Java reference: flush followed by finish, with the writer
// always closed afterwards (errors during close are surfaced when the
// flush path itself succeeded).
//
// Mirrors VectorValuesConsumer.flush(SegmentWriteState, Sorter.DocMap).
func (c *vectorValuesConsumer) Flush(state *SegmentWriteState, sortMap SorterDocMap) error {
	if c.writer == nil {
		return nil
	}
	if state == nil || state.SegmentInfo == nil {
		// Match the Java NPE that would surface on state.segmentInfo.maxDoc(),
		// surfaced here as a typed error so callers can react without a panic.
		return errors.New("index: VectorValuesConsumer.Flush requires a non-nil state with SegmentInfo")
	}

	flushErr := c.writer.Flush(state.SegmentInfo.DocCount(), sortMap)
	var finishErr error
	if flushErr == nil {
		finishErr = c.writer.Finish()
	}
	closeErr := c.writer.Close()
	c.writer = nil

	switch {
	case flushErr != nil:
		return flushErr
	case finishErr != nil:
		return finishErr
	case closeErr != nil:
		return fmt.Errorf("index: VectorValuesConsumer.Flush close: %w", closeErr)
	}
	return nil
}

// Abort closes the writer while suppressing any error. Mirrors
// VectorValuesConsumer.abort() (IOUtils.closeWhileHandlingException).
func (c *vectorValuesConsumer) Abort() {
	if c.writer == nil {
		return
	}
	_ = util.CloseWhileHandlingException(c.writer, "VectorValuesConsumer writer")
	c.writer = nil
}

// GetAccountable returns the Accountable for RAM accounting. Before the
// codec writer is created this is a zero-bytes Accountable; afterwards
// it forwards to the writer's RamBytesUsed. Mirrors
// VectorValuesConsumer.getAccountable.
func (c *vectorValuesConsumer) GetAccountable() util.Accountable {
	if c.writer == nil {
		return nullAccountable{}
	}
	return knnWriterAccountable{w: c.writer}
}

// RamBytesUsed reports the writer's in-memory footprint, or zero if the
// writer has not been constructed yet. Surface preserved for the
// IndexingChain bookkeeping.
func (c *vectorValuesConsumer) RamBytesUsed() int64 {
	if c.writer == nil {
		return 0
	}
	return c.writer.RamBytesUsed()
}

// nullAccountable is the zero-bytes Accountable used as the initial value
// of VectorValuesConsumer.accountable. It mirrors Lucene's
// Accountable.NULL_ACCOUNTABLE for which Gocene has no shared singleton.
type nullAccountable struct{}

// RamBytesUsed always reports zero.
func (nullAccountable) RamBytesUsed() int64 { return 0 }

// knnWriterAccountable adapts a spi.KnnVectorsWriter to util.Accountable
// for callers that need the Accountable contract without taking a hard
// dependency on the wide writer interface.
type knnWriterAccountable struct{ w spi.KnnVectorsWriter }

// RamBytesUsed forwards to the underlying writer.
func (a knnWriterAccountable) RamBytesUsed() int64 { return a.w.RamBytesUsed() }
