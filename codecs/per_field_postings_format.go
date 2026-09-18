// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sort"
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
//
// Mirrors the inner class PerFieldPostingsFormat.FieldsWriter
// (PerFieldPostingsFormat.java:130-269).
type PerFieldFieldsConsumer struct {
	formatProvider FieldPostingsFormatProvider
	state          *SegmentWriteState

	// toClose holds every delegate FieldsConsumer opened by Write, in the
	// order they were opened. Mirrors FieldsWriter.toClose.
	toClose []FieldsConsumer

	mu     sync.Mutex
	closed bool
}

// perFieldsGroup is the Go rendering of the record
// PerFieldPostingsFormat.FieldsGroup (PerFieldPostingsFormat.java:87): the set
// of fields assigned to one delegate format, the integer suffix that
// uniquifies that delegate, and the SegmentWriteState the delegate writes with.
type perFieldsGroup struct {
	fields []string
	suffix int
	state  *SegmentWriteState
}

// perFieldsGroupBuilder is the Go rendering of the nested class
// PerFieldPostingsFormat.FieldsGroup.Builder.
type perFieldsGroupBuilder struct {
	suffix int
	state  *SegmentWriteState
	fields map[string]struct{}
}

func newPerFieldsGroupBuilder(suffix int, state *SegmentWriteState) *perFieldsGroupBuilder {
	return &perFieldsGroupBuilder{suffix: suffix, state: state, fields: make(map[string]struct{})}
}

func (b *perFieldsGroupBuilder) addField(field string) {
	b.fields[field] = struct{}{}
}

func (b *perFieldsGroupBuilder) build() perFieldsGroup {
	fieldList := make([]string, 0, len(b.fields))
	for f := range b.fields {
		fieldList = append(fieldList, f)
	}
	sort.Strings(fieldList)
	return perFieldsGroup{fields: fieldList, suffix: b.suffix, state: b.state}
}

// maskedFields exposes only the fields of one FieldsGroup while delegating
// Terms and Size to the underlying Fields. Mirrors the anonymous FilterFields
// subclass PerFieldPostingsFormat.FieldsWriter.write builds
// (PerFieldPostingsFormat.java:151-157), which overrides iterator() alone.
type maskedFields struct {
	in     index.Fields
	fields []string
}

func (m *maskedFields) Iterator() (index.FieldIterator, error) {
	return &maskedFieldIterator{fields: m.fields}, nil
}

func (m *maskedFields) Terms(field string) (index.Terms, error) { return m.in.Terms(field) }

func (m *maskedFields) Size() int { return m.in.Size() }

// maskedFieldIterator walks the group's field names.
type maskedFieldIterator struct {
	fields []string
	pos    int
}

func (it *maskedFieldIterator) Next() (string, error) {
	if it.pos >= len(it.fields) {
		return "", nil
	}
	name := it.fields[it.pos]
	it.pos++
	return name, nil
}

func (it *maskedFieldIterator) HasNext() bool { return it.pos < len(it.fields) }

// NewPerFieldFieldsConsumer creates a new PerFieldFieldsConsumer.
func NewPerFieldFieldsConsumer(provider FieldPostingsFormatProvider, state *SegmentWriteState) *PerFieldFieldsConsumer {
	return &PerFieldFieldsConsumer{
		formatProvider: provider,
		state:          state,
	}
}

// buildFieldsGroupMapping assigns every indexed field name to its delegate
// PostingsFormat, allocating one FieldsGroup (and its segment suffix) per
// distinct format instance and stamping the per-field codec attributes on the
// matching FieldInfo.
//
// Mirrors the private
// PerFieldPostingsFormat.FieldsWriter.buildFieldsGroupMapping(Iterable<String>)
// (PerFieldPostingsFormat.java:204-254).
func (c *PerFieldFieldsConsumer) buildFieldsGroupMapping(indexedFieldNames []string) ([]PostingsFormat, map[PostingsFormat]perFieldsGroup, error) {
	// Maps a PostingsFormat instance to the suffix it should use.
	formatToGroupBuilders := make(map[PostingsFormat]*perFieldsGroupBuilder)
	// Preserves the order in which formats were first seen, so the delegates
	// are opened deterministically.
	var formatOrder []PostingsFormat
	// Holds the last suffix of each PostingsFormat name.
	suffixes := make(map[string]int)

	for _, field := range indexedFieldNames {
		fieldInfo := c.state.FieldInfos.GetByName(field)
		if fieldInfo == nil {
			return nil, nil, fmt.Errorf("no FieldInfo for field %q", field)
		}
		format := c.formatProvider.GetPostingsFormat(field)
		if format == nil {
			return nil, nil, fmt.Errorf("invalid null PostingsFormat for field=%q", field)
		}
		formatName := format.Name()

		groupBuilder, seen := formatToGroupBuilders[format]
		if !seen {
			// First time we are seeing this format; create a new instance.
			suffix := 0
			if prev, ok := suffixes[formatName]; ok {
				suffix = prev + 1
			}
			suffixes[formatName] = suffix

			segmentSuffix, err := perFieldPostingsFullSegmentSuffix(
				field, c.state.SegmentSuffix, perFieldPostingsSuffix(formatName, strconv.Itoa(suffix)))
			if err != nil {
				return nil, nil, err
			}
			groupBuilder = newPerFieldsGroupBuilder(suffix, &SegmentWriteState{
				Directory:     c.state.Directory,
				SegmentInfo:   c.state.SegmentInfo,
				FieldInfos:    c.state.FieldInfos,
				SegmentSuffix: segmentSuffix,
			})
			formatToGroupBuilders[format] = groupBuilder
			formatOrder = append(formatOrder, format)
		} else if _, ok := suffixes[formatName]; !ok {
			return nil, nil, fmt.Errorf("no suffix for format name: %s, expected: %d", formatName, groupBuilder.suffix)
		}

		groupBuilder.addField(field)

		fieldInfo.PutCodecAttribute(PER_FIELD_POSTINGS_FORMAT_KEY, formatName)
		fieldInfo.PutCodecAttribute(PER_FIELD_POSTINGS_SUFFIX_KEY, strconv.Itoa(groupBuilder.suffix))
	}

	formatToGroups := make(map[PostingsFormat]perFieldsGroup, len(formatToGroupBuilders))
	for format, builder := range formatToGroupBuilders {
		formatToGroups[format] = builder.build()
	}
	return formatOrder, formatToGroups, nil
}

// Write groups the fields by delegate PostingsFormat and drives one delegate
// FieldsConsumer per group over a Fields view masked to that group.
//
// Mirrors PerFieldPostingsFormat.FieldsWriter.write(Fields, NormsProducer)
// (PerFieldPostingsFormat.java:140-169).
func (c *PerFieldFieldsConsumer) Write(fields index.Fields, norms NormsProducer) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("PerFieldFieldsConsumer is closed")
	}

	names, err := perFieldIndexedFieldNames(fields)
	if err != nil {
		return err
	}
	formatOrder, formatToGroups, err := c.buildFieldsGroupMapping(names)
	if err != nil {
		return err
	}

	// Write postings.
	success := false
	defer func() {
		if !success {
			for _, consumer := range c.toClose {
				_ = consumer.Close()
			}
			c.toClose = nil
		}
	}()
	for _, format := range formatOrder {
		group := formatToGroups[format]
		consumer, err := format.FieldsConsumer(group.state)
		if err != nil {
			return fmt.Errorf("failed to create FieldsConsumer for format %q: %w", format.Name(), err)
		}
		c.toClose = append(c.toClose, consumer)
		// Exposes only the fields from this group.
		if err := consumer.Write(&maskedFields{in: fields, fields: group.fields}, norms); err != nil {
			return err
		}
	}
	success = true
	return nil
}

// perFieldIndexedFieldNames drains a Fields iterator into a slice, preserving
// its order. Java iterates the Iterable<String> directly.
func perFieldIndexedFieldNames(fields index.Fields) ([]string, error) {
	if fields == nil {
		return nil, nil
	}
	it, err := fields.Iterator()
	if err != nil {
		return nil, err
	}
	var names []string
	for {
		name, err := it.Next()
		if err != nil {
			return nil, err
		}
		if name == "" {
			return names, nil
		}
		names = append(names, name)
	}
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
	for _, consumer := range c.toClose {
		if err := consumer.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close delegate FieldsConsumer: %w", err)
		}
	}
	c.toClose = nil
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

// CheckIntegrity walks every delegate FieldsProducer and validates its
// checksums.
//
// Port of
// org.apache.lucene.codecs.perfield.PerFieldPostingsFormat.FieldsReader#checkIntegrity
// (Lucene 10.5.0):
//
//	for (FieldsProducer producer : formats.values()) { producer.checkIntegrity(); }
//
// formats is keyed by format suffix in Java, so the Go loop walks
// producersBySuffix — the map that likewise holds one entry per open delegate.
func (p *PerFieldFieldsProducer) CheckIntegrity() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, producer := range p.producersBySuffix {
		if err := producer.CheckIntegrity(); err != nil {
			return err
		}
	}
	return nil
}

// Iterator returns the names of the fields that carry postings, in sorted
// order. Mirrors PerFieldPostingsFormat.FieldsReader.iterator(), which walks
// the key set of a TreeMap.
func (p *PerFieldFieldsProducer) Iterator() (index.FieldIterator, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.producersByField))
	for name := range p.producersByField {
		names = append(names, name)
	}
	sort.Strings(names)
	return index.NewMemoryFieldIterator(names), nil
}

// Size returns the number of fields that carry postings. Mirrors
// PerFieldPostingsFormat.FieldsReader.size().
func (p *PerFieldFieldsProducer) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.producersByField)
}

// GetMergeInstance returns a new producer holding the merge instance of every
// delegate. Mirrors PerFieldPostingsFormat.FieldsReader.getMergeInstance(),
// which returns new FieldsReader(this).
func (p *PerFieldFieldsProducer) GetMergeInstance() FieldsProducer {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return newPerFieldFieldsProducerForMerge(p)
}

// newPerFieldFieldsProducerForMerge is the "clone for merge" constructor
// FieldsReader(FieldsReader other) of PerFieldPostingsFormat.
func newPerFieldFieldsProducerForMerge(other *PerFieldFieldsProducer) *PerFieldFieldsProducer {
	p := &PerFieldFieldsProducer{
		state:             other.state,
		producersByField:  make(map[string]FieldsProducer, len(other.producersByField)),
		producersBySuffix: make(map[string]FieldsProducer, len(other.producersBySuffix)),
	}
	oldToNew := make(map[FieldsProducer]FieldsProducer, len(other.producersBySuffix))
	// First clone all formats
	for suffix, producer := range other.producersBySuffix {
		values := producer.GetMergeInstance()
		p.producersBySuffix[suffix] = values
		oldToNew[producer] = values
	}
	// Then rebuild fields:
	for field, producer := range other.producersByField {
		newProducer, ok := oldToNew[producer]
		// assert producer != null;
		if !ok {
			panic(fmt.Sprintf("PerFieldFieldsProducer: no merge instance for the producer of field %q", field))
		}
		p.producersByField[field] = newProducer
	}
	return p
}
