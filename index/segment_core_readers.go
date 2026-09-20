// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentCoreReaders holds core readers that are shared (unchanged) when a
// SegmentReader is cloned or reopened.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentCoreReaders.
//
// The per-format readers are opened in the same order as the Java constructor
// (postings, term vectors, stored fields, KNN vectors, doc values, norms,
// points) and are reference counted: the last DecRef closes every reader and
// notifies the registered close listeners.
//
// PORT NOTE: Lucene resolves the codec and reads the core FieldInfos inside the
// constructor. Gocene's openSegmentReader resolves both beforehand (it has to
// fall back to the default codec when the segment carries no registered codec
// name, and it re-reads a newer .fnm generation for the exposed reader), so the
// already-resolved SegmentInfo, core FieldInfos and Codec are passed in.
type SegmentCoreReaders struct {
	// refCount is the reference count for this core.
	refCount atomic.Int32

	// fields is the postings reader.
	fields FieldsProducer

	// termVectorsReader is the term vectors reader.
	termVectorsReader TermVectorsReader

	// storedFieldsReader is the stored fields reader.
	storedFieldsReader StoredFieldsReader

	// docValuesProducer is the doc-values producer.
	docValuesProducer DocValuesProducer

	// normsProducer is the norms producer.
	normsProducer NormsProducer

	// pointsReader is the points (BKD) reader.
	pointsReader PointsReader

	// vectorsReader is the KNN vectors reader.
	vectorsReader KnnVectorsReader

	// fieldInfos is the core FieldInfos the readers above were opened with.
	fieldInfos *FieldInfos

	// segmentName is the name of the segment.
	segmentName string

	// directory is the directory containing the segment.
	directory store.Directory

	// cfsReader is the compound file reader (nil when the segment is not
	// stored in a compound file).
	cfsReader CompoundDirectory

	// cacheHelper is the core-level CacheHelper. Mirrors the anonymous
	// IndexReader.CacheHelper Lucene installs on SegmentCoreReaders.
	cacheHelper *spi.ReaderCacheHelper

	// closedListeners are notified when the core is closed.
	closedListeners []func()

	// mu protects closedListeners and closed.
	mu sync.Mutex

	// closed reports whether the core has already been closed.
	closed bool
}

// NewSegmentCoreReaders opens every per-format reader for a segment.
//
// directory is the segment's directory, segmentInfo its .si metadata,
// fieldInfos the core (base generation) field metadata the data files were
// written against, codec the resolved codec and context the IO context used
// for the per-format opens.
func NewSegmentCoreReaders(
	directory store.Directory,
	segmentInfo *SegmentInfo,
	fieldInfos *FieldInfos,
	codec Codec,
	context store.IOContext,
) (*SegmentCoreReaders, error) {
	core := &SegmentCoreReaders{
		fieldInfos:  fieldInfos,
		segmentName: segmentInfo.Name(),
		directory:   directory,
		cacheHelper: spi.NewReaderCacheHelper(),
	}
	core.refCount.Store(1)

	// Determine the directory to read the per-format files from: the compound
	// file view when the segment is compound, the raw directory otherwise.
	var cfsDir store.Directory
	if segmentInfo.GetUseCompoundFile() {
		compoundFormat := codec.CompoundFormat()
		if compoundFormat == nil {
			core.decRef()
			return nil, fmt.Errorf("segment %q uses a compound file but codec %q has no CompoundFormat", segmentInfo.Name(), codec.Name())
		}
		cfsReader, err := compoundFormat.GetCompoundReader(directory, segmentInfo)
		if err != nil {
			core.decRef()
			return nil, fmt.Errorf("opening compound reader for segment %q: %w", segmentInfo.Name(), err)
		}
		core.cfsReader = cfsReader
		cfsDir = cfsReader
	} else {
		cfsDir = directory
	}

	readState := NewSegmentReadState(cfsDir, segmentInfo, fieldInfos, context)

	// Postings.
	if fieldInfos.HasPostings() {
		if postingsFormat := codec.PostingsFormat(); postingsFormat != nil {
			fieldsProducer, err := postingsFormat.FieldsProducer(readState)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating fields producer: %w", err)
			}
			core.fields = fieldsProducer
		}
	}

	// Term vectors.
	if fieldInfos.HasTermVectors() {
		if termVectorsFormat := codec.TermVectorsFormat(); termVectorsFormat != nil {
			tvReader, err := termVectorsFormat.VectorsReader(cfsDir, segmentInfo, fieldInfos, context)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating term vectors reader: %w", err)
			}
			core.termVectorsReader = tvReader
		}
	}

	// Stored fields. A failure here is non-fatal: the segment may carry no
	// stored fields at all (a taxonomy directory stores only doc values), in
	// which case callers receive a nil reader and take the empty path.
	if storedFieldsFormat := codec.StoredFieldsFormat(); storedFieldsFormat != nil {
		sfReader, err := storedFieldsFormat.FieldsReader(cfsDir, segmentInfo, fieldInfos, context)
		if err == nil {
			core.storedFieldsReader = sfReader
		}
	}

	// KNN vectors.
	if fieldInfos.HasVectorValues() {
		if knnFormat := codec.KnnVectorsFormat(); knnFormat != nil {
			vrReader, err := knnFormat.FieldsReader(readState)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating knn vectors reader: %w", err)
			}
			core.vectorsReader = vrReader
		}
	}

	// Doc values.
	if fieldInfos.HasDocValues() {
		if docValuesFormat := codec.DocValuesFormat(); docValuesFormat != nil {
			dvProducer, err := docValuesFormat.FieldsProducer(readState)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating doc values producer: %w", err)
			}
			core.docValuesProducer = dvProducer
		}
	}

	// Norms.
	if fieldInfos.HasNorms() {
		if normsFormat := codec.NormsFormat(); normsFormat != nil {
			normsProducer, err := normsFormat.NormsProducer(readState)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating norms producer: %w", err)
			}
			core.normsProducer = normsProducer
		}
	}

	// Points.
	if fieldInfos.HasPointValues() {
		if pointsFormat := codec.PointsFormat(); pointsFormat != nil {
			ptReader, err := pointsFormat.FieldsReader(readState)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating points reader: %w", err)
			}
			core.pointsReader = ptReader
		}
	}

	return core, nil
}

// IncRef increments the reference count.
// It returns an error when the core has already been closed.
func (core *SegmentCoreReaders) IncRef() error {
	for {
		count := core.refCount.Load()
		if count <= 0 {
			return fmt.Errorf("SegmentCoreReaders is already closed")
		}
		if core.refCount.CompareAndSwap(count, count+1) {
			return nil
		}
	}
}

// TryIncRef increments the reference count and reports whether it succeeded.
// Unlike IncRef it does not report an error when the core is already closed.
func (core *SegmentCoreReaders) TryIncRef() bool {
	for {
		count := core.refCount.Load()
		if count <= 0 {
			return false
		}
		if core.refCount.CompareAndSwap(count, count+1) {
			return true
		}
	}
}

// DecRef decrements the reference count, closing every reader when it reaches
// zero.
func (core *SegmentCoreReaders) DecRef() error {
	if core.refCount.Add(-1) == 0 {
		return core.close()
	}
	return nil
}

// decRef is the error-free variant used on the constructor's failure paths.
func (core *SegmentCoreReaders) decRef() {
	if core.refCount.Add(-1) == 0 {
		// The failure path has nothing to report the close error to; the
		// constructor error already describes why the core is being torn down.
		_ = core.close()
	}
}

// close releases every reader and notifies the close listeners.
func (core *SegmentCoreReaders) close() error {
	core.mu.Lock()
	defer core.mu.Unlock()

	if core.closed {
		return nil
	}
	core.closed = true

	var lastErr error
	closeOne := func(c interface{ Close() error }) {
		if err := c.Close(); err != nil {
			lastErr = err
		}
	}

	if core.fields != nil {
		closeOne(core.fields)
	}
	if core.termVectorsReader != nil {
		closeOne(core.termVectorsReader)
	}
	if core.storedFieldsReader != nil {
		closeOne(core.storedFieldsReader)
	}
	if core.docValuesProducer != nil {
		closeOne(core.docValuesProducer)
	}
	if core.normsProducer != nil {
		closeOne(core.normsProducer)
	}
	if core.pointsReader != nil {
		closeOne(core.pointsReader)
	}
	if core.vectorsReader != nil {
		closeOne(core.vectorsReader)
	}
	if core.cfsReader != nil {
		closeOne(core.cfsReader)
	}

	if core.cacheHelper != nil {
		core.cacheHelper.SetClosed()
		core.cacheHelper.NotifyClosedListeners()
	}
	for _, listener := range core.closedListeners {
		listener()
	}
	core.closedListeners = nil

	return lastErr
}

// GetRefCount returns the current reference count.
func (core *SegmentCoreReaders) GetRefCount() int32 {
	return core.refCount.Load()
}

// GetFields returns the postings reader, or nil when the segment has no
// indexed fields.
func (core *SegmentCoreReaders) GetFields() FieldsProducer {
	return core.fields
}

// GetTermVectorsReader returns the term vectors reader, or nil when the
// segment stores no term vectors.
func (core *SegmentCoreReaders) GetTermVectorsReader() TermVectorsReader {
	return core.termVectorsReader
}

// GetStoredFieldsReader returns the stored fields reader.
func (core *SegmentCoreReaders) GetStoredFieldsReader() StoredFieldsReader {
	return core.storedFieldsReader
}

// GetDocValuesProducer returns the doc-values producer, or nil when the
// segment carries no doc values.
func (core *SegmentCoreReaders) GetDocValuesProducer() DocValuesProducer {
	return core.docValuesProducer
}

// SetDocValuesProducer replaces the doc-values producer held by this core.
// openSegmentReader uses it to overlay a SegmentDocValuesProducer on top of
// the base producer so that per-generation doc-values updates resolve.
func (core *SegmentCoreReaders) SetDocValuesProducer(dvp DocValuesProducer) {
	core.docValuesProducer = dvp
}

// GetNormsProducer returns the norms producer, or nil when no field has norms.
func (core *SegmentCoreReaders) GetNormsProducer() NormsProducer {
	return core.normsProducer
}

// GetPointsReader returns the points reader, or nil when no field indexes
// point values.
func (core *SegmentCoreReaders) GetPointsReader() PointsReader {
	return core.pointsReader
}

// GetVectorReader returns the KNN vectors reader, or nil when no field carries
// vector values.
func (core *SegmentCoreReaders) GetVectorReader() KnnVectorsReader {
	return core.vectorsReader
}

// GetFieldInfos returns the core FieldInfos.
func (core *SegmentCoreReaders) GetFieldInfos() *FieldInfos {
	return core.fieldInfos
}

// GetSegmentName returns the segment name.
func (core *SegmentCoreReaders) GetSegmentName() string {
	return core.segmentName
}

// GetDirectory returns the directory containing the segment.
func (core *SegmentCoreReaders) GetDirectory() store.Directory {
	return core.directory
}

// GetCacheHelper returns the core-level CacheHelper, whose CacheKey stays
// stable for the lifetime of the shared core.
func (core *SegmentCoreReaders) GetCacheHelper() CacheHelper {
	return core.cacheHelper
}

// CheckIntegrity validates the checksum framing of every per-format file this
// core has open. Mirrors org.apache.lucene.index.CodecReader.checkIntegrity,
// which walks the same readers in the same order.
func (core *SegmentCoreReaders) CheckIntegrity() error {
	if core.storedFieldsReader != nil {
		if err := core.storedFieldsReader.CheckIntegrity(); err != nil {
			return err
		}
	}
	if core.termVectorsReader != nil {
		if err := core.termVectorsReader.CheckIntegrity(); err != nil {
			return err
		}
	}
	if core.fields != nil {
		if err := core.fields.CheckIntegrity(); err != nil {
			return err
		}
	}
	if core.normsProducer != nil {
		if err := core.normsProducer.CheckIntegrity(); err != nil {
			return err
		}
	}
	if core.docValuesProducer != nil {
		if err := core.docValuesProducer.CheckIntegrity(); err != nil {
			return err
		}
	}
	if core.pointsReader != nil {
		if err := core.pointsReader.CheckIntegrity(); err != nil {
			return err
		}
	}
	if core.vectorsReader != nil {
		if err := core.vectorsReader.CheckIntegrity(); err != nil {
			return err
		}
	}
	return nil
}

// AddClosedListener registers a callback fired when the core is closed.
func (core *SegmentCoreReaders) AddClosedListener(listener func()) {
	core.mu.Lock()
	defer core.mu.Unlock()
	core.closedListeners = append(core.closedListeners, listener)
}

// IsClosed reports whether the core has been closed.
func (core *SegmentCoreReaders) IsClosed() bool {
	core.mu.Lock()
	defer core.mu.Unlock()
	return core.closed
}
