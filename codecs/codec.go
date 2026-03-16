// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"
)

// Codec abstracts index format encoding/decoding.
// A Codec provides codecs for different components
// (postings, stored fields, field infos, segment infos, term vectors).
//
// This is the Go port of Lucene's org.apache.lucene.codecs.Codec.
type Codec interface {
	// Name returns the name of this codec.
	Name() string

	// PostingsFormat returns the postings format.
	PostingsFormat() PostingsFormat

	// StoredFieldsFormat returns the stored fields format.
	StoredFieldsFormat() StoredFieldsFormat

	// FieldInfosFormat returns the field infos format.
	FieldInfosFormat() FieldInfosFormat

	// SegmentInfosFormat returns the segment infos format.
	SegmentInfosFormat() SegmentInfosFormat

	// TermVectorsFormat returns the term vectors format.
	TermVectorsFormat() TermVectorsFormat

	// DocValuesFormat returns the doc values format.
	DocValuesFormat() DocValuesFormat
}

// BaseCodec provides common functionality.
type BaseCodec struct {
	name string
}

// NewBaseCodec creates a new BaseCodec.
func NewBaseCodec(name string) *BaseCodec {
	return &BaseCodec{name: name}
}

// Name returns the codec name.
func (c *BaseCodec) Name() string {
	return c.name
}

// PostingsFormat returns the postings format.
func (c *BaseCodec) PostingsFormat() PostingsFormat {
	return nil
}

// StoredFieldsFormat returns the stored fields format.
func (c *BaseCodec) StoredFieldsFormat() StoredFieldsFormat {
	return nil
}

// FieldInfosFormat returns the field infos format.
func (c *BaseCodec) FieldInfosFormat() FieldInfosFormat {
	return nil
}

// SegmentInfosFormat returns the segment infos format.
func (c *BaseCodec) SegmentInfosFormat() SegmentInfosFormat {
	return nil
}

// TermVectorsFormat returns the term vectors format.
func (c *BaseCodec) TermVectorsFormat() TermVectorsFormat {
	return nil
}

// DocValuesFormat returns the doc values format.
func (c *BaseCodec) DocValuesFormat() DocValuesFormat {
	return nil
}

// CodecRegistry manages registered codecs.
// This is a simple registry that allows looking up codecs by name.
type CodecRegistry struct {
	codecs       map[string]Codec
	mu           sync.RWMutex
	defaultCodec Codec
}

// Global codec registry instance.
var defaultRegistry = &CodecRegistry{
	codecs: make(map[string]Codec),
}

// RegisterCodec registers a codec with the given name.
// Returns an error if a codec with the same name is already registered.
func RegisterCodec(name string, codec Codec) error {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()

	if existing, ok := defaultRegistry.codecs[name]; ok && existing != nil {
		return fmt.Errorf("codec '%s' is already registered", name)
	}

	defaultRegistry.codecs[name] = codec
	return nil
}

// UnregisterCodec removes a codec from the registry.
func UnregisterCodec(name string) {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()

	delete(defaultRegistry.codecs, name)

	// Clear default if it was the unregistered codec
	if defaultRegistry.defaultCodec != nil && defaultRegistry.defaultCodec.Name() == name {
		defaultRegistry.defaultCodec = nil
	}
}

// ForName returns the codec registered with the given name.
// Returns an error if no codec is registered with that name.
func ForName(name string) (Codec, error) {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	if codec, ok := defaultRegistry.codecs[name]; ok && codec != nil {
		return codec, nil
	}

	return nil, fmt.Errorf("no codec registered with name '%s'", name)
}

// GetDefault returns the default codec.
// If no default is set, it returns the "Lucene104" codec if registered.
// Returns an error if no default codec is available.
func GetDefault() (Codec, error) {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	if defaultRegistry.defaultCodec != nil {
		return defaultRegistry.defaultCodec, nil
	}

	// Try to get Lucene104 codec as fallback
	if codec, ok := defaultRegistry.codecs["Lucene104"]; ok && codec != nil {
		return codec, nil
	}

	return nil, fmt.Errorf("no default codec set and no Lucene104 codec registered")
}

// SetDefault sets the default codec.
func SetDefault(codec Codec) {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()

	defaultRegistry.defaultCodec = codec
}

// AvailableCodecs returns a list of registered codec names.
func AvailableCodecs() []string {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	names := make([]string, 0, len(defaultRegistry.codecs))
	for name := range defaultRegistry.codecs {
		names = append(names, name)
	}
	return names
}

// init registers the built-in codecs.
func init() {
	// Register Lucene104 codec as the default
	lucene104 := NewLucene104Codec()
	RegisterCodec("Lucene104", lucene104)
	SetDefault(lucene104)
}
