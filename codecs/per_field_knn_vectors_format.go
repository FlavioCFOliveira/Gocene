// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PerFieldKnnVectorsFormat name and FieldInfo attribute keys.
//
// These constants mirror Lucene 10.4.0's PerFieldKnnVectorsFormat:
//   - PER_FIELD_KNN_VECTORS_FORMAT_NAME is the format name written to the
//     segment.
//   - PER_FIELD_KNN_VECTORS_FORMAT_KEY is the FieldInfo attribute key for
//     the concrete delegate format's name.
//   - PER_FIELD_KNN_VECTORS_SUFFIX_KEY is the FieldInfo attribute key for
//     the integer suffix that uniquifies the delegate's segment suffix.
//
// Together, the two attributes make each KNN-vectors field self-describing
// on the read path; the reader does not need the original formatProvider.
const (
	PER_FIELD_KNN_VECTORS_FORMAT_NAME = "PerFieldVectors90"
	PER_FIELD_KNN_VECTORS_FORMAT_KEY  = "PerFieldKnnVectorsFormat.format"
	PER_FIELD_KNN_VECTORS_SUFFIX_KEY  = "PerFieldKnnVectorsFormat.suffix"
)

// KnnVectorsFormat registry.
//
// Lucene resolves delegate KnnVectorsFormats by name via the Java SPI
// (KnnVectorsFormat.forName). Gocene uses an explicit registry seeded by
// codec init() functions; tests can register additional formats.
var (
	knnVectorsFormatRegistryMu sync.RWMutex
	knnVectorsFormatRegistry   = make(map[string]KnnVectorsFormat)
)

// RegisterKnnVectorsFormat publishes format under format.Name() in the
// global KnnVectorsFormat registry, replacing any previous registration
// with the same name. It is safe to call concurrently.
func RegisterKnnVectorsFormat(format KnnVectorsFormat) {
	if format == nil {
		return
	}
	knnVectorsFormatRegistryMu.Lock()
	defer knnVectorsFormatRegistryMu.Unlock()
	knnVectorsFormatRegistry[format.Name()] = format
}

// UnregisterKnnVectorsFormat removes the format previously registered
// under name. It is a no-op when no such format exists.
func UnregisterKnnVectorsFormat(name string) {
	knnVectorsFormatRegistryMu.Lock()
	defer knnVectorsFormatRegistryMu.Unlock()
	delete(knnVectorsFormatRegistry, name)
}

// KnnVectorsFormatByName returns the KnnVectorsFormat registered under
// name. It returns an error when no format is registered for name.
func KnnVectorsFormatByName(name string) (KnnVectorsFormat, error) {
	knnVectorsFormatRegistryMu.RLock()
	defer knnVectorsFormatRegistryMu.RUnlock()
	format, ok := knnVectorsFormatRegistry[name]
	if !ok {
		return nil, fmt.Errorf("no KnnVectorsFormat registered with name %q", name)
	}
	return format, nil
}

// perFieldKnnVectorsSuffix returns the per-field segment suffix encoding
// formatName and the integer suffix in Lucene's "<formatName>_<n>" form.
func perFieldKnnVectorsSuffix(formatName, suffix string) string {
	return formatName + "_" + suffix
}

// perFieldKnnVectorsFullSegmentSuffix combines the outer segment suffix
// with the per-format inner suffix. Like PerFieldDocValuesFormat, the KNN
// variant supports outer-suffix nesting by joining with an underscore;
// matches Java's getFullSegmentSuffix(outerSegmentSuffix, segmentSuffix).
func perFieldKnnVectorsFullSegmentSuffix(outerSegmentSuffix, innerSegmentSuffix string) string {
	if outerSegmentSuffix == "" {
		return innerSegmentSuffix
	}
	return outerSegmentSuffix + "_" + innerSegmentSuffix
}

// fieldHasVectorValues mirrors Java's FieldInfo.hasVectorValues(): a field
// carries KNN-vector values when its declared dimension is positive.
func fieldHasVectorValues(fi *index.FieldInfo) bool {
	return fi.VectorDimension() > 0
}

// PerFieldKnnVectorsFormat is a KnnVectorsFormat that delegates to a
// different KnnVectorsFormat for each field. It is the Go port of Lucene
// 10.4.0's org.apache.lucene.codecs.perfield.PerFieldKnnVectorsFormat.
//
// On write, the field's chosen format name and an integer suffix are
// recorded on the field's FieldInfo via PutCodecAttribute, and each
// format's output files carry the suffix "<formatName>_<n>" (for example
// "_1_Lucene99HnswVectorsFormat_0.vec"). On read, the reader iterates
// FieldInfos, reads the attributes, and resolves the delegate via
// KnnVectorsFormatByName — the original formatProvider is not required.
type PerFieldKnnVectorsFormat struct {
	*BaseKnnVectorsFormat
	formatProvider FieldKnnVectorsFormatProvider
}

// FieldKnnVectorsFormatProvider returns the KnnVectorsFormat that should
// be used for writing new segments of a given field. It is invoked only
// on the write path; the read path is self-describing via FieldInfo
// attributes.
type FieldKnnVectorsFormatProvider interface {
	GetKnnVectorsFormat(field string) KnnVectorsFormat
}

// FieldKnnVectorsFormatProviderFunc adapts a plain function to the
// FieldKnnVectorsFormatProvider interface.
type FieldKnnVectorsFormatProviderFunc func(field string) KnnVectorsFormat

// GetKnnVectorsFormat implements FieldKnnVectorsFormatProvider.
func (f FieldKnnVectorsFormatProviderFunc) GetKnnVectorsFormat(field string) KnnVectorsFormat {
	return f(field)
}

// NewPerFieldKnnVectorsFormat creates a new PerFieldKnnVectorsFormat that
// resolves the per-field delegate through provider on the write path.
func NewPerFieldKnnVectorsFormat(provider FieldKnnVectorsFormatProvider) *PerFieldKnnVectorsFormat {
	return &PerFieldKnnVectorsFormat{
		BaseKnnVectorsFormat: NewBaseKnnVectorsFormat(PER_FIELD_KNN_VECTORS_FORMAT_NAME),
		formatProvider:       provider,
	}
}

// NewPerFieldKnnVectorsFormatWithDefault creates a new
// PerFieldKnnVectorsFormat that uses defaultFormat for every field.
func NewPerFieldKnnVectorsFormatWithDefault(defaultFormat KnnVectorsFormat) *PerFieldKnnVectorsFormat {
	return NewPerFieldKnnVectorsFormat(FieldKnnVectorsFormatProviderFunc(func(field string) KnnVectorsFormat {
		return defaultFormat
	}))
}

// FieldsWriter returns a KnnVectorsWriter that groups fields by delegate
// KnnVectorsFormat, assigning each format a unique integer suffix and
// stamping the chosen format-name plus suffix onto every field's
// FieldInfo.
func (f *PerFieldKnnVectorsFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error) {
	return NewPerFieldKnnVectorsWriter(f.formatProvider, state), nil
}

// FieldsReader returns a KnnVectorsReader that dispatches per field based
// on the format-name and suffix attributes recorded on each FieldInfo.
func (f *PerFieldKnnVectorsFormat) FieldsReader(state *SegmentReadState) (KnnVectorsReader, error) {
	return NewPerFieldKnnVectorsReader(state)
}

// PerFieldKnnVectorsWriter writes each field's KNN-vector values through
// the delegate KnnVectorsFormat returned by the
// FieldKnnVectorsFormatProvider, recording the format/suffix metadata on
// every FieldInfo it touches.
type PerFieldKnnVectorsWriter struct {
	formatProvider FieldKnnVectorsFormatProvider
	state          *SegmentWriteState

	// writersByFormat caches one delegate KnnVectorsWriter per delegate
	// KnnVectorsFormat instance and pins the integer suffix assigned to
	// it.
	writersByFormat map[KnnVectorsFormat]*knnWriterAndSuffix

	// suffixesByFormatName tracks, for each delegate format name, the
	// highest integer suffix already assigned. Mirrors Java's "suffixes"
	// HashMap.
	suffixesByFormatName map[string]int

	mu     sync.Mutex
	closed bool
}

// knnWriterAndSuffix pairs a delegate KnnVectorsWriter with the integer
// suffix assigned to its delegate format. The pair is reused for every
// field that resolves to the same delegate instance.
type knnWriterAndSuffix struct {
	writer KnnVectorsWriter
	suffix int
}

// NewPerFieldKnnVectorsWriter creates a new PerFieldKnnVectorsWriter.
func NewPerFieldKnnVectorsWriter(provider FieldKnnVectorsFormatProvider, state *SegmentWriteState) *PerFieldKnnVectorsWriter {
	return &PerFieldKnnVectorsWriter{
		formatProvider:       provider,
		state:                state,
		writersByFormat:      make(map[KnnVectorsFormat]*knnWriterAndSuffix),
		suffixesByFormatName: make(map[string]int),
	}
}

// getInstance returns the delegate KnnVectorsWriter for field, allocating
// a new one and bumping the format-name suffix counter on first use. It
// also stamps the per-field codec attributes onto the field's FieldInfo.
func (w *PerFieldKnnVectorsWriter) getInstance(field *index.FieldInfo) (KnnVectorsWriter, error) {
	format := w.formatProvider.GetKnnVectorsFormat(field.Name())
	if format == nil {
		return nil, fmt.Errorf("invalid null KnnVectorsFormat for field=%q", field.Name())
	}
	formatName := format.Name()

	field.PutCodecAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY, formatName)

	was, ok := w.writersByFormat[format]
	if !ok {
		suffix := 0
		if prev, seen := w.suffixesByFormatName[formatName]; seen {
			suffix = prev + 1
		}
		w.suffixesByFormatName[formatName] = suffix

		innerSuffix := perFieldKnnVectorsSuffix(formatName, strconv.Itoa(suffix))
		segmentSuffix := perFieldKnnVectorsFullSegmentSuffix(w.state.SegmentSuffix, innerSuffix)

		delegateState := &SegmentWriteState{
			Directory:     w.state.Directory,
			SegmentInfo:   w.state.SegmentInfo,
			FieldInfos:    w.state.FieldInfos,
			SegmentSuffix: segmentSuffix,
		}

		writer, err := format.FieldsWriter(delegateState)
		if err != nil {
			return nil, fmt.Errorf("failed to create KnnVectorsWriter for field %q: %w", field.Name(), err)
		}
		was = &knnWriterAndSuffix{writer: writer, suffix: suffix}
		w.writersByFormat[format] = was
	}

	field.PutCodecAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY, strconv.Itoa(was.suffix))
	return was.writer, nil
}

// WriteField writes a KNN-vector field through the delegate chosen for
// fieldInfo, recording the format/suffix metadata on fieldInfo.
func (w *PerFieldKnnVectorsWriter) WriteField(fieldInfo *index.FieldInfo, reader KnnVectorsReader) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("PerFieldKnnVectorsWriter is closed")
	}
	writer, err := w.getInstance(fieldInfo)
	if err != nil {
		return err
	}
	return writer.WriteField(fieldInfo, reader)
}

// Finish flushes every delegate KnnVectorsWriter that was opened.
func (w *PerFieldKnnVectorsWriter) Finish() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("PerFieldKnnVectorsWriter is closed")
	}
	var lastErr error
	for format, was := range w.writersByFormat {
		if err := was.writer.Finish(); err != nil {
			lastErr = fmt.Errorf("failed to finish writer for format %q: %w", format.Name(), err)
		}
	}
	return lastErr
}

// Close closes every delegate KnnVectorsWriter that was opened. It
// returns the last error observed and continues closing the remaining
// writers so that no delegate is leaked when one fails.
func (w *PerFieldKnnVectorsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true

	var lastErr error
	for format, was := range w.writersByFormat {
		if err := was.writer.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close writer for format %q: %w", format.Name(), err)
		}
	}
	w.writersByFormat = nil
	return lastErr
}

// PerFieldKnnVectorsReader reads KNN-vector values written by
// PerFieldKnnVectorsWriter. It resolves the delegate format per FieldInfo
// via KnnVectorsFormatByName and caches one underlying KnnVectorsReader
// per "<formatName>_<n>" segment suffix.
type PerFieldKnnVectorsReader struct {
	state *SegmentReadState

	// readersByField maps each field number (mirroring Java's
	// IntObjectHashMap keyed by FieldInfo.number) to the KnnVectorsReader
	// that holds its vectors. Fields with no PerField attributes are
	// absent.
	readersByField map[int]KnnVectorsReader

	// readersBySuffix de-duplicates open readers across fields that share
	// the same delegate (i.e., the same "<formatName>_<n>" suffix).
	readersBySuffix map[string]KnnVectorsReader

	mu     sync.RWMutex
	closed bool
}

// NewPerFieldKnnVectorsReader opens every delegate KnnVectorsReader
// referenced by the FieldInfos in state and returns a reader that
// dispatches by field number. On error, every reader opened so far is
// closed to avoid leaks.
func NewPerFieldKnnVectorsReader(state *SegmentReadState) (*PerFieldKnnVectorsReader, error) {
	r := &PerFieldKnnVectorsReader{
		state:           state,
		readersByField:  make(map[int]KnnVectorsReader),
		readersBySuffix: make(map[string]KnnVectorsReader),
	}

	closeAll := func() {
		for _, rd := range r.readersBySuffix {
			_ = rd.Close()
		}
	}

	it := state.FieldInfos.Iterator()
	for {
		fi := it.Next()
		if fi == nil {
			break
		}
		if !fieldHasVectorValues(fi) {
			continue
		}
		formatName := fi.GetAttribute(PER_FIELD_KNN_VECTORS_FORMAT_KEY)
		if formatName == "" {
			// Field is in FieldInfos but carries no vectors.
			continue
		}
		suffix := fi.GetAttribute(PER_FIELD_KNN_VECTORS_SUFFIX_KEY)
		if suffix == "" {
			closeAll()
			return nil, fmt.Errorf(
				"missing attribute: %s for field: %s",
				PER_FIELD_KNN_VECTORS_SUFFIX_KEY, fi.Name(),
			)
		}

		innerSuffix := perFieldKnnVectorsSuffix(formatName, suffix)
		segmentSuffix := perFieldKnnVectorsFullSegmentSuffix(state.SegmentSuffix, innerSuffix)

		reader, ok := r.readersBySuffix[segmentSuffix]
		if !ok {
			format, err := KnnVectorsFormatByName(formatName)
			if err != nil {
				closeAll()
				return nil, fmt.Errorf("field %q: %w", fi.Name(), err)
			}
			delegateState := &SegmentReadState{
				Directory:     state.Directory,
				SegmentInfo:   state.SegmentInfo,
				FieldInfos:    state.FieldInfos,
				SegmentSuffix: segmentSuffix,
			}
			reader, err = format.FieldsReader(delegateState)
			if err != nil {
				closeAll()
				return nil, fmt.Errorf("failed to create KnnVectorsReader for field %q: %w", fi.Name(), err)
			}
			r.readersBySuffix[segmentSuffix] = reader
		}
		r.readersByField[fi.Number()] = reader
	}

	return r, nil
}

// GetFieldReader returns the underlying KnnVectorsReader for the given
// field name, or nil when no delegate claims it. It mirrors Java's
// FieldsReader.getFieldReader.
func (r *PerFieldKnnVectorsReader) GetFieldReader(field string) KnnVectorsReader {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fi := r.state.FieldInfos.GetByName(field)
	if fi == nil {
		return nil
	}
	return r.readersByField[fi.Number()]
}

// CheckIntegrity runs an integrity check on every underlying delegate
// reader.
func (r *PerFieldKnnVectorsReader) CheckIntegrity() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return fmt.Errorf("PerFieldKnnVectorsReader is closed")
	}
	for suffix, reader := range r.readersBySuffix {
		if err := reader.CheckIntegrity(); err != nil {
			return fmt.Errorf("integrity check failed for suffix %q: %w", suffix, err)
		}
	}
	return nil
}

// Close closes every underlying KnnVectorsReader exactly once and returns
// the last error observed.
func (r *PerFieldKnnVectorsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	var lastErr error
	for suffix, reader := range r.readersBySuffix {
		if err := reader.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close reader for suffix %q: %w", suffix, err)
		}
	}
	r.readersByField = nil
	r.readersBySuffix = nil
	return lastErr
}

// MapFieldKnnVectorsFormatProvider routes per-field requests through an
// explicit field-to-KnnVectorsFormat map, falling back to a default
// format.
type MapFieldKnnVectorsFormatProvider struct {
	mu            sync.RWMutex
	fieldFormats  map[string]KnnVectorsFormat
	defaultFormat KnnVectorsFormat
}

// NewMapFieldKnnVectorsFormatProvider creates a provider that returns
// defaultFormat for any field that is not in the explicit map.
func NewMapFieldKnnVectorsFormatProvider(defaultFormat KnnVectorsFormat) *MapFieldKnnVectorsFormatProvider {
	return &MapFieldKnnVectorsFormatProvider{
		fieldFormats:  make(map[string]KnnVectorsFormat),
		defaultFormat: defaultFormat,
	}
}

// SetFormat associates field with format, replacing any prior mapping.
func (p *MapFieldKnnVectorsFormatProvider) SetFormat(field string, format KnnVectorsFormat) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fieldFormats[field] = format
}

// GetKnnVectorsFormat implements FieldKnnVectorsFormatProvider.
func (p *MapFieldKnnVectorsFormatProvider) GetKnnVectorsFormat(field string) KnnVectorsFormat {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if format, ok := p.fieldFormats[field]; ok {
		return format
	}
	return p.defaultFormat
}

// Ensure implementations satisfy the interfaces.
var (
	_ KnnVectorsFormat              = (*PerFieldKnnVectorsFormat)(nil)
	_ KnnVectorsWriter              = (*PerFieldKnnVectorsWriter)(nil)
	_ KnnVectorsReader              = (*PerFieldKnnVectorsReader)(nil)
	_ FieldKnnVectorsFormatProvider = (*MapFieldKnnVectorsFormatProvider)(nil)
	_ FieldKnnVectorsFormatProvider = (FieldKnnVectorsFormatProviderFunc)(nil)
)
