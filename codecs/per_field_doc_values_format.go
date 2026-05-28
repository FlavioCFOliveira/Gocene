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

// PerFieldDocValuesFormat name and FieldInfo attribute keys.
//
// These constants mirror Lucene 10.4.0's PerFieldDocValuesFormat:
//   - PER_FIELD_DOC_VALUES_FORMAT_NAME is the format name written to the segment.
//   - PER_FIELD_DOC_VALUES_FORMAT_KEY is the FieldInfo attribute key for the
//     concrete delegate format's name.
//   - PER_FIELD_DOC_VALUES_SUFFIX_KEY is the FieldInfo attribute key for the
//     integer suffix that uniquifies the delegate's segment suffix.
//
// Together, the two attributes make each doc-values field self-describing on
// the read path; the reader does not need the original formatProvider.
const (
	PER_FIELD_DOC_VALUES_FORMAT_NAME = "PerFieldDV40"
	PER_FIELD_DOC_VALUES_FORMAT_KEY  = "PerFieldDocValuesFormat.format"
	PER_FIELD_DOC_VALUES_SUFFIX_KEY  = "PerFieldDocValuesFormat.suffix"
)

// DocValuesFormat registry.
//
// Lucene resolves delegate doc-values formats by name via the Java SPI
// (DocValuesFormat.forName). Gocene uses an explicit registry seeded by
// codec init() functions; tests can register additional formats.
var (
	docValuesFormatRegistryMu sync.RWMutex
	docValuesFormatRegistry   = make(map[string]DocValuesFormat)
)

// RegisterDocValuesFormat publishes format under format.Name() in the global
// DocValuesFormat registry, replacing any previous registration with the
// same name. It is safe to call concurrently.
func RegisterDocValuesFormat(format DocValuesFormat) {
	if format == nil {
		return
	}
	docValuesFormatRegistryMu.Lock()
	defer docValuesFormatRegistryMu.Unlock()
	docValuesFormatRegistry[format.Name()] = format
}

// UnregisterDocValuesFormat removes the format previously registered under
// name. It is a no-op when no such format exists.
func UnregisterDocValuesFormat(name string) {
	docValuesFormatRegistryMu.Lock()
	defer docValuesFormatRegistryMu.Unlock()
	delete(docValuesFormatRegistry, name)
}

// DocValuesFormatByName returns the DocValuesFormat registered under name.
// It returns an error when no format is registered for name.
func DocValuesFormatByName(name string) (DocValuesFormat, error) {
	docValuesFormatRegistryMu.RLock()
	defer docValuesFormatRegistryMu.RUnlock()
	format, ok := docValuesFormatRegistry[name]
	if !ok {
		return nil, fmt.Errorf("no DocValuesFormat registered with name %q", name)
	}
	return format, nil
}

// perFieldDocValuesSuffix returns the per-field segment suffix encoding
// formatName and the integer suffix in Lucene's "<formatName>_<n>" form.
func perFieldDocValuesSuffix(formatName, suffix string) string {
	return formatName + "_" + suffix
}

// perFieldDocValuesFullSegmentSuffix combines the outer segment suffix with
// the per-format inner suffix. Unlike PerFieldPostingsFormat, PerField docs
// allow non-empty outer suffixes by concatenating them with an underscore,
// matching Java's getFullSegmentSuffix(outerSegmentSuffix, segmentSuffix).
func perFieldDocValuesFullSegmentSuffix(outerSegmentSuffix, innerSegmentSuffix string) string {
	if outerSegmentSuffix == "" {
		return innerSegmentSuffix
	}
	return outerSegmentSuffix + "_" + innerSegmentSuffix
}

// PerFieldDocValuesFormat is a DocValuesFormat that delegates to a different
// DocValuesFormat for each field. It is the Go port of Lucene 10.4.0's
// org.apache.lucene.codecs.perfield.PerFieldDocValuesFormat.
//
// On write, the field's chosen format name and an integer suffix are recorded
// on the field's FieldInfo via PutCodecAttribute, and each format's output
// files carry the suffix "<formatName>_<n>" (for example "_1_Lucene90_0.dvd").
// On read, the reader iterates FieldInfos, reads the attributes, and resolves
// the delegate via DocValuesFormatByName — the original formatProvider is not
// required.
type PerFieldDocValuesFormat struct {
	*BaseDocValuesFormat
	formatProvider FieldDocValuesFormatProvider
}

// FieldDocValuesFormatProvider returns the DocValuesFormat that should be
// used for writing new segments of a given field. It is invoked only on the
// write path; the read path is self-describing via FieldInfo attributes.
type FieldDocValuesFormatProvider interface {
	GetDocValuesFormat(field string) DocValuesFormat
}

// FieldDocValuesFormatProviderFunc adapts a plain function to the
// FieldDocValuesFormatProvider interface.
type FieldDocValuesFormatProviderFunc func(field string) DocValuesFormat

// GetDocValuesFormat implements FieldDocValuesFormatProvider.
func (f FieldDocValuesFormatProviderFunc) GetDocValuesFormat(field string) DocValuesFormat {
	return f(field)
}

// NewPerFieldDocValuesFormat creates a new PerFieldDocValuesFormat that
// resolves the per-field delegate through provider on the write path.
func NewPerFieldDocValuesFormat(provider FieldDocValuesFormatProvider) *PerFieldDocValuesFormat {
	return &PerFieldDocValuesFormat{
		BaseDocValuesFormat: NewBaseDocValuesFormat(PER_FIELD_DOC_VALUES_FORMAT_NAME),
		formatProvider:      provider,
	}
}

// NewPerFieldDocValuesFormatWithDefault creates a new PerFieldDocValuesFormat
// that uses defaultFormat for every field.
func NewPerFieldDocValuesFormatWithDefault(defaultFormat DocValuesFormat) *PerFieldDocValuesFormat {
	return NewPerFieldDocValuesFormat(FieldDocValuesFormatProviderFunc(func(field string) DocValuesFormat {
		return defaultFormat
	}))
}

// FieldsConsumer returns a DocValuesConsumer that groups fields by delegate
// DocValuesFormat, assigning each format a unique integer suffix and stamping
// the chosen format-name plus suffix onto every field's FieldInfo.
func (f *PerFieldDocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	return NewPerFieldDocValuesConsumer(f.formatProvider, state), nil
}

// FieldsProducer returns a DocValuesProducer that dispatches per field based
// on the format-name and suffix attributes recorded on each FieldInfo.
func (f *PerFieldDocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	return NewPerFieldDocValuesProducer(state)
}

// PerFieldDocValuesConsumer writes each field's doc-values through the
// delegate DocValuesFormat returned by the FieldDocValuesFormatProvider,
// recording the format/suffix metadata on every FieldInfo it touches.
type PerFieldDocValuesConsumer struct {
	formatProvider FieldDocValuesFormatProvider
	state          *SegmentWriteState

	// consumersByFormat caches one delegate DocValuesConsumer per delegate
	// DocValuesFormat instance and pins the integer suffix assigned to it.
	consumersByFormat map[DocValuesFormat]*docValuesConsumerAndSuffix

	// suffixesByFormatName tracks, for each delegate format name, the highest
	// integer suffix already assigned. Mirrors Java's "suffixes" HashMap.
	suffixesByFormatName map[string]int

	mu     sync.Mutex
	closed bool
}

// docValuesConsumerAndSuffix pairs a delegate DocValuesConsumer with the
// integer suffix assigned to its delegate format. The pair is reused for
// every field that resolves to the same delegate instance.
type docValuesConsumerAndSuffix struct {
	consumer DocValuesConsumer
	suffix   int
}

// NewPerFieldDocValuesConsumer creates a new PerFieldDocValuesConsumer.
func NewPerFieldDocValuesConsumer(provider FieldDocValuesFormatProvider, state *SegmentWriteState) *PerFieldDocValuesConsumer {
	return &PerFieldDocValuesConsumer{
		formatProvider:       provider,
		state:                state,
		consumersByFormat:    make(map[DocValuesFormat]*docValuesConsumerAndSuffix),
		suffixesByFormatName: make(map[string]int),
	}
}

// getInstance returns the delegate DocValuesConsumer for field, allocating
// a new one and bumping the format-name suffix counter on first use. It
// also stamps the per-field codec attributes onto the field's FieldInfo.
//
// Unlike PerFieldPostingsFormat, doc-values fields may carry an existing
// PerFieldDocValuesFormat attribute when the field already exists in a
// segment that is being updated. When the field's DocValuesGen is non-zero
// and the existing attribute is set, the suffix is honoured so the update
// lands in the same delegate file as the prior generation.
func (c *PerFieldDocValuesConsumer) getInstance(field *index.FieldInfo) (DocValuesConsumer, error) {
	format := c.formatProvider.GetDocValuesFormat(field.Name())
	if format == nil {
		return nil, fmt.Errorf("invalid null DocValuesFormat for field=%q", field.Name())
	}
	formatName := format.Name()

	field.PutCodecAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY, formatName)

	cas, ok := c.consumersByFormat[format]
	if !ok {
		// First time we are seeing this format instance.
		var suffix int
		var pinned bool

		if field.DocValuesGen() != -1 {
			// Updated field: respect any pre-existing suffix attribute so the
			// new generation lands in the same delegate file.
			if existing := field.GetAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY); existing != "" {
				if parsed, err := strconv.Atoi(existing); err == nil {
					suffix = parsed
					pinned = true
				}
			}
		}

		if !pinned {
			if prev, seen := c.suffixesByFormatName[formatName]; seen {
				suffix = prev + 1
			}
		}
		c.suffixesByFormatName[formatName] = suffix

		innerSuffix := perFieldDocValuesSuffix(formatName, strconv.Itoa(suffix))
		segmentSuffix := perFieldDocValuesFullSegmentSuffix(c.state.SegmentSuffix, innerSuffix)

		delegateState := &SegmentWriteState{
			Directory:     c.state.Directory,
			SegmentInfo:   c.state.SegmentInfo,
			FieldInfos:    c.state.FieldInfos,
			SegmentSuffix: segmentSuffix,
		}

		consumer, err := format.FieldsConsumer(delegateState)
		if err != nil {
			return nil, fmt.Errorf("failed to create DocValuesConsumer for field %q: %w", field.Name(), err)
		}
		cas = &docValuesConsumerAndSuffix{consumer: consumer, suffix: suffix}
		c.consumersByFormat[format] = cas
	}

	field.PutCodecAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY, strconv.Itoa(cas.suffix))
	return cas.consumer, nil
}

// AddNumericField writes a numeric doc values field through the delegate
// chosen for field, recording the format/suffix metadata on field.
func (c *PerFieldDocValuesConsumer) AddNumericField(field *index.FieldInfo, values NumericDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}
	consumer, err := c.getInstance(field)
	if err != nil {
		return err
	}
	return consumer.AddNumericField(field, values)
}

// AddBinaryField writes a binary doc values field through the delegate
// chosen for field, recording the format/suffix metadata on field.
func (c *PerFieldDocValuesConsumer) AddBinaryField(field *index.FieldInfo, values BinaryDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}
	consumer, err := c.getInstance(field)
	if err != nil {
		return err
	}
	return consumer.AddBinaryField(field, values)
}

// AddSortedField writes a sorted doc values field through the delegate
// chosen for field, recording the format/suffix metadata on field.
func (c *PerFieldDocValuesConsumer) AddSortedField(field *index.FieldInfo, values SortedDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}
	consumer, err := c.getInstance(field)
	if err != nil {
		return err
	}
	return consumer.AddSortedField(field, values)
}

// AddSortedSetField writes a sorted-set doc values field through the
// delegate chosen for field, recording the format/suffix metadata on field.
func (c *PerFieldDocValuesConsumer) AddSortedSetField(field *index.FieldInfo, values SortedSetDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}
	consumer, err := c.getInstance(field)
	if err != nil {
		return err
	}
	return consumer.AddSortedSetField(field, values)
}

// AddSortedNumericField writes a sorted-numeric doc values field through
// the delegate chosen for field, recording the format/suffix metadata on
// field.
func (c *PerFieldDocValuesConsumer) AddSortedNumericField(field *index.FieldInfo, values SortedNumericDocValuesIterator) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("PerFieldDocValuesConsumer is closed")
	}
	consumer, err := c.getInstance(field)
	if err != nil {
		return err
	}
	return consumer.AddSortedNumericField(field, values)
}

// Close closes every delegate DocValuesConsumer that was opened. It
// returns the last error observed and continues closing the remaining
// consumers so that no delegate is leaked when one fails.
func (c *PerFieldDocValuesConsumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	var lastErr error
	for format, cas := range c.consumersByFormat {
		if err := cas.consumer.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close consumer for format %q: %w", format.Name(), err)
		}
	}
	c.consumersByFormat = nil
	return lastErr
}

// PerFieldDocValuesProducer reads doc-values written by
// PerFieldDocValuesConsumer. It resolves the delegate format per FieldInfo
// via DocValuesFormatByName and caches one underlying DocValuesProducer per
// "<formatName>_<n>" segment suffix.
type PerFieldDocValuesProducer struct {
	state *SegmentReadState

	// producersByField maps each field number (mirroring Java's
	// IntObjectHashMap keyed by FieldInfo.number) to the DocValuesProducer
	// that holds its doc-values. Fields with no PerField attributes are
	// absent.
	producersByField map[int]DocValuesProducer

	// producersBySuffix de-duplicates open producers across fields that
	// share the same delegate (i.e., the same "<formatName>_<n>" suffix).
	producersBySuffix map[string]DocValuesProducer

	mu     sync.RWMutex
	closed bool
}

// NewPerFieldDocValuesProducer opens every delegate DocValuesProducer
// referenced by the FieldInfos in state and returns a producer that
// dispatches by field number. On error, every producer opened so far is
// closed to avoid leaks.
func NewPerFieldDocValuesProducer(state *SegmentReadState) (*PerFieldDocValuesProducer, error) {
	p := &PerFieldDocValuesProducer{
		state:             state,
		producersByField:  make(map[int]DocValuesProducer),
		producersBySuffix: make(map[string]DocValuesProducer),
	}

	closeAll := func() {
		for _, prod := range p.producersBySuffix {
			_ = prod.Close()
		}
	}

	it := state.FieldInfos.Iterator()
	for {
		fi := it.Next()
		if fi == nil {
			break
		}
		if !fi.DocValuesType().HasDocValues() {
			continue
		}
		formatName := fi.GetAttribute(PER_FIELD_DOC_VALUES_FORMAT_KEY)
		if formatName == "" {
			// Field is in FieldInfos but carries no doc-values.
			continue
		}
		suffix := fi.GetAttribute(PER_FIELD_DOC_VALUES_SUFFIX_KEY)
		if suffix == "" {
			closeAll()
			return nil, fmt.Errorf(
				"missing attribute: %s for field: %s",
				PER_FIELD_DOC_VALUES_SUFFIX_KEY, fi.Name(),
			)
		}

		innerSuffix := perFieldDocValuesSuffix(formatName, suffix)
		segmentSuffix := perFieldDocValuesFullSegmentSuffix(state.SegmentSuffix, innerSuffix)

		producer, ok := p.producersBySuffix[segmentSuffix]
		if !ok {
			format, err := DocValuesFormatByName(formatName)
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
			producer, err = format.FieldsProducer(delegateState)
			if err != nil {
				closeAll()
				return nil, fmt.Errorf("failed to create DocValuesProducer for field %q: %w", fi.Name(), err)
			}
			p.producersBySuffix[segmentSuffix] = producer
		}

		p.producersByField[fi.Number()] = producer
	}

	return p, nil
}

// GetNumeric returns the NumericDocValues for field through the delegate
// producer that claims field's number; (nil, nil) when no delegate claims
// it.
func (p *PerFieldDocValuesProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	producer, err := p.producerFor(field)
	if err != nil || producer == nil {
		return nil, err
	}
	return producer.GetNumeric(field)
}

// GetBinary returns the BinaryDocValues for field through the delegate
// producer that claims field's number; (nil, nil) when no delegate claims
// it.
func (p *PerFieldDocValuesProducer) GetBinary(field *index.FieldInfo) (BinaryDocValues, error) {
	producer, err := p.producerFor(field)
	if err != nil || producer == nil {
		return nil, err
	}
	return producer.GetBinary(field)
}

// GetSorted returns the SortedDocValues for field through the delegate
// producer that claims field's number; (nil, nil) when no delegate claims
// it.
func (p *PerFieldDocValuesProducer) GetSorted(field *index.FieldInfo) (SortedDocValues, error) {
	producer, err := p.producerFor(field)
	if err != nil || producer == nil {
		return nil, err
	}
	return producer.GetSorted(field)
}

// GetSortedSet returns the SortedSetDocValues for field through the
// delegate producer that claims field's number; (nil, nil) when no
// delegate claims it.
func (p *PerFieldDocValuesProducer) GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error) {
	producer, err := p.producerFor(field)
	if err != nil || producer == nil {
		return nil, err
	}
	return producer.GetSortedSet(field)
}

// GetSortedNumeric returns the SortedNumericDocValues for field through
// the delegate producer that claims field's number; (nil, nil) when no
// delegate claims it.
func (p *PerFieldDocValuesProducer) GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error) {
	producer, err := p.producerFor(field)
	if err != nil || producer == nil {
		return nil, err
	}
	return producer.GetSortedNumeric(field)
}

// GetSkipper dispatches to the delegate producer that owns the field,
// returning whatever skipper that delegate exposes (typically nil when
// the delegate format does not write a sparse skipper).
//
// Required by spi.DocValuesProducer since rmp #4708 lifted the
// doc-values family onto the SPI.
func (p *PerFieldDocValuesProducer) GetSkipper(field *index.FieldInfo) (DocValuesSkipper, error) {
	producer, err := p.producerFor(field)
	if err != nil || producer == nil {
		return nil, err
	}
	return producer.GetSkipper(field)
}

// CheckIntegrity runs an integrity check on every underlying delegate
// producer.
func (p *PerFieldDocValuesProducer) CheckIntegrity() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return fmt.Errorf("PerFieldDocValuesProducer is closed")
	}
	for suffix, producer := range p.producersBySuffix {
		if err := producer.CheckIntegrity(); err != nil {
			return fmt.Errorf("integrity check failed for suffix %q: %w", suffix, err)
		}
	}
	return nil
}

// Close closes every underlying DocValuesProducer exactly once and returns
// the last error observed.
func (p *PerFieldDocValuesProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true

	var lastErr error
	for suffix, producer := range p.producersBySuffix {
		if err := producer.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close producer for suffix %q: %w", suffix, err)
		}
	}
	p.producersByField = nil
	p.producersBySuffix = nil
	return lastErr
}

// producerFor returns the delegate DocValuesProducer for field, or nil
// when no delegate claims field's number. It surfaces a closed-producer
// error to keep the Get* accessors uniform.
func (p *PerFieldDocValuesProducer) producerFor(field *index.FieldInfo) (DocValuesProducer, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return nil, fmt.Errorf("PerFieldDocValuesProducer is closed")
	}
	return p.producersByField[field.Number()], nil
}

// MapFieldDocValuesFormatProvider routes per-field requests through an
// explicit field-to-DocValuesFormat map, falling back to a default format.
type MapFieldDocValuesFormatProvider struct {
	mu            sync.RWMutex
	fieldFormats  map[string]DocValuesFormat
	defaultFormat DocValuesFormat
}

// NewMapFieldDocValuesFormatProvider creates a provider that returns
// defaultFormat for any field that is not in the explicit map.
func NewMapFieldDocValuesFormatProvider(defaultFormat DocValuesFormat) *MapFieldDocValuesFormatProvider {
	return &MapFieldDocValuesFormatProvider{
		fieldFormats:  make(map[string]DocValuesFormat),
		defaultFormat: defaultFormat,
	}
}

// SetFormat associates field with format, replacing any prior mapping.
func (p *MapFieldDocValuesFormatProvider) SetFormat(field string, format DocValuesFormat) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fieldFormats[field] = format
}

// GetDocValuesFormat implements FieldDocValuesFormatProvider.
func (p *MapFieldDocValuesFormatProvider) GetDocValuesFormat(field string) DocValuesFormat {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if format, ok := p.fieldFormats[field]; ok {
		return format
	}
	return p.defaultFormat
}

// Ensure implementations satisfy the interfaces.
var (
	_ DocValuesFormat              = (*PerFieldDocValuesFormat)(nil)
	_ DocValuesConsumer            = (*PerFieldDocValuesConsumer)(nil)
	_ DocValuesProducer            = (*PerFieldDocValuesProducer)(nil)
	_ FieldDocValuesFormatProvider = (*MapFieldDocValuesFormatProvider)(nil)
	_ FieldDocValuesFormatProvider = (FieldDocValuesFormatProviderFunc)(nil)
)
