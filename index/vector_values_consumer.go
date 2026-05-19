// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrVectorValuesConsumerNoKnnFormat is returned by VectorValuesConsumer.AddField
// when AddField is invoked before a KnnVectorsFormat has been wired in. It
// mirrors the IllegalStateException emitted by the Java reference when the
// codec does not provide a KnnVectorsFormat for an indexed vector field.
var ErrVectorValuesConsumerNoKnnFormat = errors.New("index: VectorValuesConsumer: field was indexed as vectors but codec does not support vectors")

// KnnVectorsFormatFactory mirrors a single method of
// org.apache.lucene.codecs.KnnVectorsFormat used by VectorValuesConsumer
// (Codec.knnVectorsFormat().fieldsWriter(state)).
//
// It is declared in package index to keep the consumer self-contained and
// to avoid an index -> codecs import cycle. Concrete factories live in
// package codecs (or downstream codec packages) and are injected via
// VectorValuesConsumer.SetKnnVectorsFormat.
//
// This is the local equivalent of calling
// codec.knnVectorsFormat().fieldsWriter(state) in Lucene.
type KnnVectorsFormatFactory interface {
	// FieldsWriter constructs the per-segment KnnVectorsWriter for the
	// given write state. The returned writer is owned by the consumer:
	// the consumer is responsible for invoking Flush / Finish / Close.
	FieldsWriter(state *SegmentWriteState) (KnnVectorsConsumerWriter, error)
}

// KnnVectorsConsumerWriter is the narrow contract VectorValuesConsumer
// requires from the codec's KnnVectorsWriter. It mirrors the methods the
// Java reference invokes on org.apache.lucene.codecs.KnnVectorsWriter
// (addField, flush, finish, close); RAM accounting is observed through the
// util.Accountable contract returned by GetAccountable().
//
// Declared locally in package index to keep the consumer free of an index
// -> codecs import. Codec writers in the codecs package satisfy this
// contract by exposing AddField / Flush / Finish / Close with matching
// signatures.
type KnnVectorsConsumerWriter interface {
	util.Accountable

	// AddField registers a new vector field for indexing. The returned
	// per-field writer accumulates vectors for that field; concrete codec
	// implementations return a typed KnnFieldVectorsWriter parameterised
	// by float32 or byte.
	AddField(fi *FieldInfo) (any, error)

	// Flush writes every buffered vector field to disk. sortMap is nil
	// for un-sorted segments and non-nil for index-sorted segments.
	Flush(maxDoc int, sortMap SorterDocMap) error

	// Finish is invoked once after Flush to write any trailing metadata
	// (footers, integrity markers).
	Finish() error

	// Close releases all writer resources. Always invoked after Flush /
	// Finish, including on the abort path.
	Close() error
}

// VectorValuesConsumer streams vector values for indexing to the given
// codec's KnnVectorsWriter. The codec's vectors writer is responsible for
// buffering and processing vectors.
//
// This is the Go port of org.apache.lucene.index.VectorValuesConsumer from
// Apache Lucene 10.4.0. The Java class is package-private and so is the
// Go type (lower-case identifier mirrors the original visibility).
//
// Deviations from Lucene 10.4.0:
//   - Lucene resolves the KnnVectorsFormat from Codec.knnVectorsFormat();
//     the Gocene Codec interface does not yet expose KnnVectorsFormat()
//     (extending it would be a cross-cutting change touching every Codec
//     implementation). The format is therefore injected via
//     SetKnnVectorsFormat, mirroring the bridge already used by
//     SortingStoredFieldsConsumer.SetTempStoredFieldsFormat. Once a
//     Codec.KnnVectorsFormat() accessor lands, the setter is removed and
//     the consumer reads through the Codec directly.
//   - The accountable handle defaults to a zero-bytes Accountable instead
//     of Accountable.NULL_ACCOUNTABLE (Gocene's util.Accountable contract
//     has no shared NULL singleton).
type vectorValuesConsumer struct {
	codec      Codec
	directory  store.Directory
	info       *SegmentInfo
	infoStream util.InfoStream

	// knnFormat is the per-consumer KnnVectorsFormatFactory bridge used
	// in place of codec.knnVectorsFormat() (see deviation note above).
	knnFormat KnnVectorsFormatFactory

	writer      KnnVectorsConsumerWriter
	accountable util.Accountable
}

// newVectorValuesConsumer mirrors the Java constructor
// VectorValuesConsumer(Codec, Directory, SegmentInfo, InfoStream).
//
// A KnnVectorsFormatFactory must be wired in via SetKnnVectorsFormat
// before the first call to AddField; otherwise AddField returns
// ErrVectorValuesConsumerNoKnnFormat (mirroring the IllegalStateException
// path in Lucene when codec.knnVectorsFormat() is null).
func newVectorValuesConsumer(codec Codec, directory store.Directory, info *SegmentInfo, infoStream util.InfoStream) *vectorValuesConsumer {
	return &vectorValuesConsumer{
		codec:       codec,
		directory:   directory,
		info:        info,
		infoStream:  infoStream,
		accountable: nullAccountable{},
	}
}

// setKnnVectorsFormat injects the KnnVectorsFormat used to construct the
// per-segment KnnVectorsWriter. Exists to bridge the gap left by
// Codec.KnnVectorsFormat() not being part of the Gocene Codec interface
// (see type doc deviation note). Safe to call only before AddField.
func (c *vectorValuesConsumer) setKnnVectorsFormat(f KnnVectorsFormatFactory) {
	c.knnFormat = f
}

// initKnnVectorsWriter lazily constructs the codec's KnnVectorsWriter on
// the first AddField call. Mirrors initKnnVectorsWriter in Lucene.
func (c *vectorValuesConsumer) initKnnVectorsWriter(fieldName string) error {
	if c.writer != nil {
		return nil
	}
	if c.knnFormat == nil {
		return fmt.Errorf("%w (field=%q)", ErrVectorValuesConsumerNoKnnFormat, fieldName)
	}
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
	c.accountable = w
	return nil
}

// AddField registers fieldInfo with the codec's KnnVectorsWriter and
// returns the per-field writer. The returned value is the codec-specific
// KnnFieldVectorsWriter (typed by the codec as float32 or byte); callers
// type-assert to the expected concrete shape.
//
// Mirrors VectorValuesConsumer.addField(FieldInfo).
func (c *vectorValuesConsumer) AddField(fieldInfo *FieldInfo) (any, error) {
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
// codec writer is created this is a zero-bytes Accountable; afterwards it
// is the writer itself. Mirrors VectorValuesConsumer.getAccountable.
func (c *vectorValuesConsumer) GetAccountable() util.Accountable {
	return c.accountable
}

// nullAccountable is the zero-bytes Accountable used as the initial value
// of VectorValuesConsumer.accountable. It mirrors Lucene's
// Accountable.NULL_ACCOUNTABLE for which Gocene has no shared singleton.
type nullAccountable struct{}

// RamBytesUsed always reports zero.
func (nullAccountable) RamBytesUsed() int64 { return 0 }
