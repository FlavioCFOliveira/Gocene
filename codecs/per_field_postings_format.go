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

// PerFieldPostingsFormat name and FieldInfo attribute keys.
//
// These constants mirror Lucene 10.4.0's PerFieldPostingsFormat:
//   - PER_FIELD_POSTINGS_FORMAT_NAME is the format name written to the segment.
//   - PER_FIELD_POSTINGS_FORMAT_KEY is the FieldInfo attribute key for the
//     concrete delegate format's name.
//   - PER_FIELD_POSTINGS_SUFFIX_KEY is the FieldInfo attribute key for the
//     integer suffix that uniquifies the delegate's segment suffix.
//
// Together, the two attributes make each indexed field self-describing on
// the read path; the reader does not need the original formatProvider.
const (
	PER_FIELD_POSTINGS_FORMAT_NAME = "PerField40"
	PER_FIELD_POSTINGS_FORMAT_KEY  = "PerFieldPostingsFormat.format"
	PER_FIELD_POSTINGS_SUFFIX_KEY  = "PerFieldPostingsFormat.suffix"
)

// PostingsFormat registry.
//
// Lucene resolves delegate postings formats by name via the Java SPI
// (PostingsFormat.forName). Gocene uses an explicit registry seeded by
// codec init() functions; tests can register additional formats.
var (
	postingsFormatRegistryMu sync.RWMutex
	postingsFormatRegistry   = make(map[string]PostingsFormat)
)

// RegisterPostingsFormat publishes format under format.Name() in the global
// PostingsFormat registry, replacing any previous registration with the
// same name. It is safe to call concurrently.
func RegisterPostingsFormat(format PostingsFormat) {
	if format == nil {
		return
	}
	postingsFormatRegistryMu.Lock()
	defer postingsFormatRegistryMu.Unlock()
	postingsFormatRegistry[format.Name()] = format
}

// UnregisterPostingsFormat removes the format previously registered under name.
// It is a no-op when no such format exists.
func UnregisterPostingsFormat(name string) {
	postingsFormatRegistryMu.Lock()
	defer postingsFormatRegistryMu.Unlock()
	delete(postingsFormatRegistry, name)
}

// PostingsFormatByName returns the PostingsFormat registered under name.
// It returns an error when no format is registered for name.
func PostingsFormatByName(name string) (PostingsFormat, error) {
	postingsFormatRegistryMu.RLock()
	defer postingsFormatRegistryMu.RUnlock()
	format, ok := postingsFormatRegistry[name]
	if !ok {
		return nil, fmt.Errorf("no PostingsFormat registered with name %q", name)
	}
	return format, nil
}

// perFieldPostingsSuffix returns the per-field segment suffix encoding
// formatName and the integer suffix in Lucene's "<formatName>_<n>" form.
func perFieldPostingsSuffix(formatName, suffix string) string {
	return formatName + "_" + suffix
}

// perFieldPostingsFullSegmentSuffix combines the outer segment suffix with
// the per-format inner suffix. Lucene rejects nested PerField formats; this
// helper preserves that contract: outer must be empty.
func perFieldPostingsFullSegmentSuffix(fieldName, outerSegmentSuffix, innerSegmentSuffix string) (string, error) {
	if outerSegmentSuffix == "" {
		return innerSegmentSuffix, nil
	}
	return "", fmt.Errorf(
		"cannot embed PerFieldPostingsFormat inside itself (field %q returned PerFieldPostingsFormat)",
		fieldName,
	)
}

// PerFieldPostingsFormat is a PostingsFormat that delegates to a different
// PostingsFormat for each field. It is the Go port of Lucene 10.4.0's
// org.apache.lucene.codecs.perfield.PerFieldPostingsFormat.
//
// On write, the field's chosen format name and an integer suffix are recorded
// on the field's FieldInfo via PutCodecAttribute, and each format's output
// files carry the suffix "<formatName>_<n>" (for example "_1_Lucene104_0.pst").
// On read, the reader iterates FieldInfos, reads the attributes, and resolves
// the delegate via PostingsFormatByName — the original formatProvider is not
// required.
type PerFieldPostingsFormat struct {
	*BasePostingsFormat
	formatProvider FieldPostingsFormatProvider
}

// FieldPostingsFormatProvider returns the PostingsFormat that should be used
// for writing new segments of a given field. It is invoked only on the write
// path; the read path is self-describing via FieldInfo attributes.
type FieldPostingsFormatProvider interface {
	GetPostingsFormat(field string) PostingsFormat
}

// FieldPostingsFormatProviderFunc adapts a plain function to the
// FieldPostingsFormatProvider interface.
type FieldPostingsFormatProviderFunc func(field string) PostingsFormat

// GetPostingsFormat implements FieldPostingsFormatProvider.
func (f FieldPostingsFormatProviderFunc) GetPostingsFormat(field string) PostingsFormat {
	return f(field)
}

// NewPerFieldPostingsFormat creates a new PerFieldPostingsFormat that
// resolves the per-field delegate through provider on the write path.
func NewPerFieldPostingsFormat(provider FieldPostingsFormatProvider) *PerFieldPostingsFormat {
	return &PerFieldPostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat(PER_FIELD_POSTINGS_FORMAT_NAME),
		formatProvider:     provider,
	}
}

// NewPerFieldPostingsFormatWithDefault creates a new PerFieldPostingsFormat
// that uses defaultFormat for every field.
func NewPerFieldPostingsFormatWithDefault(defaultFormat PostingsFormat) *PerFieldPostingsFormat {
	return NewPerFieldPostingsFormat(FieldPostingsFormatProviderFunc(func(field string) PostingsFormat {
		return defaultFormat
	}))
}

// FieldsConsumer returns a FieldsConsumer that groups fields by delegate
// PostingsFormat, assigning each format a unique integer suffix and stamping
// the chosen format-name plus suffix onto every field's FieldInfo.
func (f *PerFieldPostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	return NewPerFieldFieldsConsumer(f.formatProvider, state), nil
}

// FieldsProducer returns a FieldsProducer that dispatches per field based on
// the format-name and suffix attributes recorded on each FieldInfo.
func (f *PerFieldPostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	return NewPerFieldFieldsProducer(state)
}

// PerFieldFieldsConsumer writes each field's postings through the delegate
// PostingsFormat returned by the FieldPostingsFormatProvider, recording the
// format/suffix metadata on every FieldInfo it touches.
type PerFieldFieldsConsumer struct {
	formatProvider FieldPostingsFormatProvider
	state          *SegmentWriteState

	// consumersByFormat caches one delegate FieldsConsumer per delegate
	// PostingsFormat instance and pins the integer suffix assigned to it.
	consumersByFormat map[PostingsFormat]*postingsConsumerAndSuffix

	// suffixesByFormatName tracks, for each delegate format name, the highest
	// integer suffix already assigned. Mirrors Java's "suffixes" HashMap.
	suffixesByFormatName map[string]int

	mu     sync.Mutex
	closed bool
}

// postingsConsumerAndSuffix pairs a delegate FieldsConsumer with the integer
// suffix assigned to its delegate format. The pair is reused for every field
// that resolves to the same delegate instance.
type postingsConsumerAndSuffix struct {
	consumer FieldsConsumer
	suffix   int
}

// NewPerFieldFieldsConsumer creates a new PerFieldFieldsConsumer.
func NewPerFieldFieldsConsumer(provider FieldPostingsFormatProvider, state *SegmentWriteState) *PerFieldFieldsConsumer {
	return &PerFieldFieldsConsumer{
		formatProvider:       provider,
		state:                state,
		consumersByFormat:    make(map[PostingsFormat]*postingsConsumerAndSuffix),
		suffixesByFormatName: make(map[string]int),
	}
}

// getInstance returns the delegate FieldsConsumer for fieldName, allocating a
// new one and bumping the format-name suffix counter on first use. It also
// stamps the per-field codec attributes onto the field's FieldInfo.
func (c *PerFieldFieldsConsumer) getInstance(fieldName string) (FieldsConsumer, error) {
	format := c.formatProvider.GetPostingsFormat(fieldName)
	if format == nil {
		return nil, fmt.Errorf("invalid null PostingsFormat for field=%q", fieldName)
	}
	formatName := format.Name()

	fieldInfo := c.state.FieldInfos.GetByName(fieldName)
	if fieldInfo == nil {
		return nil, fmt.Errorf("no FieldInfo for field %q", fieldName)
	}

	cas, ok := c.consumersByFormat[format]
	if !ok {
		// First time seeing this delegate format instance; assign a new
		// integer suffix scoped to formatName and open the delegate.
		suffix := 0
		if prev, seen := c.suffixesByFormatName[formatName]; seen {
			suffix = prev + 1
		}
		c.suffixesByFormatName[formatName] = suffix

		innerSuffix := perFieldPostingsSuffix(formatName, strconv.Itoa(suffix))
		segmentSuffix, err := perFieldPostingsFullSegmentSuffix(fieldName, c.state.SegmentSuffix, innerSuffix)
		if err != nil {
			return nil, err
		}

		delegateState := &SegmentWriteState{
			Directory:     c.state.Directory,
			SegmentInfo:   c.state.SegmentInfo,
			FieldInfos:    c.state.FieldInfos,
			SegmentSuffix: segmentSuffix,
		}

		consumer, err := format.FieldsConsumer(delegateState)
		if err != nil {
			return nil, fmt.Errorf("failed to create FieldsConsumer for field %q: %w", fieldName, err)
		}
		cas = &postingsConsumerAndSuffix{consumer: consumer, suffix: suffix}
		c.consumersByFormat[format] = cas
	}

	fieldInfo.PutCodecAttribute(PER_FIELD_POSTINGS_FORMAT_KEY, formatName)
	fieldInfo.PutCodecAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY, strconv.Itoa(cas.suffix))

	return cas.consumer, nil
}

// Write delegates to the FieldsConsumer chosen for field, recording the
// per-field codec attributes on the matching FieldInfo.
func (c *PerFieldFieldsConsumer) Write(field string, terms index.Terms) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldFieldsConsumer is closed")
	}

	consumer, err := c.getInstance(field)
	if err != nil {
		return err
	}
	return consumer.Write(field, terms)
}

// Close closes every delegate FieldsConsumer that was opened. It returns the
// last error observed and continues closing the remaining consumers so that
// no delegate is leaked when one fails.
func (c *PerFieldFieldsConsumer) Close() error {
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

// PerFieldFieldsProducer reads postings written by PerFieldFieldsConsumer.
// It resolves the delegate format per FieldInfo via PostingsFormatByName and
// caches one underlying FieldsProducer per "<formatName>_<n>" segment suffix.
type PerFieldFieldsProducer struct {
	state *SegmentReadState

	// producersByField maps each indexed field name to the FieldsProducer
	// that holds its postings. Fields with no PerField attributes are absent.
	producersByField map[string]FieldsProducer

	// producersBySuffix de-duplicates open producers across fields that share
	// the same delegate (i.e., the same "<formatName>_<n>" suffix).
	producersBySuffix map[string]FieldsProducer

	mu     sync.RWMutex
	closed bool
}

// NewPerFieldFieldsProducer opens every delegate FieldsProducer referenced
// by the FieldInfos in state and returns a producer that dispatches by field
// name. On error, every producer opened so far is closed to avoid leaks.
func NewPerFieldFieldsProducer(state *SegmentReadState) (*PerFieldFieldsProducer, error) {
	p := &PerFieldFieldsProducer{
		state:             state,
		producersByField:  make(map[string]FieldsProducer),
		producersBySuffix: make(map[string]FieldsProducer),
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
		if !fi.IndexOptions().IsIndexed() {
			continue
		}
		formatName := fi.GetAttribute(PER_FIELD_POSTINGS_FORMAT_KEY)
		if formatName == "" {
			// Field is in FieldInfos but carries no postings.
			continue
		}
		suffix := fi.GetAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY)
		if suffix == "" {
			closeAll()
			return nil, fmt.Errorf(
				"missing attribute: %s for field: %s",
				PER_FIELD_POSTINGS_SUFFIX_KEY, fi.Name(),
			)
		}

		innerSuffix := perFieldPostingsSuffix(formatName, suffix)
		segmentSuffix, err := perFieldPostingsFullSegmentSuffix(fi.Name(), state.SegmentSuffix, innerSuffix)
		if err != nil {
			closeAll()
			return nil, err
		}

		producer, ok := p.producersBySuffix[segmentSuffix]
		if !ok {
			format, err := PostingsFormatByName(formatName)
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
				return nil, fmt.Errorf("failed to create FieldsProducer for field %q: %w", fi.Name(), err)
			}
			p.producersBySuffix[segmentSuffix] = producer
		}

		p.producersByField[fi.Name()] = producer
	}

	return p, nil
}

// Terms returns the terms for field, or (nil, nil) when no delegate producer
// claims it. Mirrors the Java FieldsReader.terms behaviour.
func (p *PerFieldFieldsProducer) Terms(field string) (index.Terms, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, fmt.Errorf("PerFieldFieldsProducer is closed")
	}
	producer, ok := p.producersByField[field]
	if !ok {
		return nil, nil
	}
	return producer.Terms(field)
}

// Close closes every underlying FieldsProducer exactly once and returns the
// last error observed.
func (p *PerFieldFieldsProducer) Close() error {
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

// MapFieldPostingsFormatProvider routes per-field requests through an
// explicit field-to-PostingsFormat map, falling back to a default format.
type MapFieldPostingsFormatProvider struct {
	mu            sync.RWMutex
	fieldFormats  map[string]PostingsFormat
	defaultFormat PostingsFormat
}

// NewMapFieldPostingsFormatProvider creates a provider that returns
// defaultFormat for any field that is not in the explicit map.
func NewMapFieldPostingsFormatProvider(defaultFormat PostingsFormat) *MapFieldPostingsFormatProvider {
	return &MapFieldPostingsFormatProvider{
		fieldFormats:  make(map[string]PostingsFormat),
		defaultFormat: defaultFormat,
	}
}

// SetFormat associates field with format, replacing any prior mapping.
func (p *MapFieldPostingsFormatProvider) SetFormat(field string, format PostingsFormat) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.fieldFormats[field] = format
}

// GetPostingsFormat implements FieldPostingsFormatProvider.
func (p *MapFieldPostingsFormatProvider) GetPostingsFormat(field string) PostingsFormat {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if format, ok := p.fieldFormats[field]; ok {
		return format
	}
	return p.defaultFormat
}

// Ensure implementations satisfy the interfaces.
var (
	_ PostingsFormat              = (*PerFieldPostingsFormat)(nil)
	_ FieldsConsumer              = (*PerFieldFieldsConsumer)(nil)
	_ FieldsProducer              = (*PerFieldFieldsProducer)(nil)
	_ FieldPostingsFormatProvider = (*MapFieldPostingsFormatProvider)(nil)
	_ FieldPostingsFormatProvider = (FieldPostingsFormatProviderFunc)(nil)
)
