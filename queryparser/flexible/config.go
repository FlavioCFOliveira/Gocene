// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "fmt"

// ConfigurationKey is a typed key for use in AbstractQueryConfig.
// It carries a human-readable name used in error messages and String output.
// This is the Go equivalent of Lucene's ConfigurationKey<V> (erased to any).
type ConfigurationKey struct {
	name string
}

// NewConfigurationKey creates a new ConfigurationKey with the given name.
func NewConfigurationKey(name string) *ConfigurationKey {
	return &ConfigurationKey{name: name}
}

// String returns the key name.
func (k *ConfigurationKey) String() string {
	return k.name
}

// AbstractQueryConfig is the base type for query configuration containers.
// Configuration values are stored as a map keyed by *ConfigurationKey.
// This is the Go equivalent of Lucene's AbstractQueryConfig.
type AbstractQueryConfig struct {
	values map[*ConfigurationKey]interface{}
}

// newAbstractQueryConfig initialises the internal map.
func newAbstractQueryConfig() AbstractQueryConfig {
	return AbstractQueryConfig{values: make(map[*ConfigurationKey]interface{})}
}

// Get returns the value associated with key, or nil if not set.
func (c *AbstractQueryConfig) Get(key *ConfigurationKey) interface{} {
	if c.values == nil {
		return nil
	}
	return c.values[key]
}

// Set stores value under key. Passing a nil value removes the entry.
func (c *AbstractQueryConfig) Set(key *ConfigurationKey, value interface{}) {
	if c.values == nil {
		c.values = make(map[*ConfigurationKey]interface{})
	}
	if value == nil {
		delete(c.values, key)
		return
	}
	c.values[key] = value
}

// Has reports whether a value is stored under key.
func (c *AbstractQueryConfig) Has(key *ConfigurationKey) bool {
	if c.values == nil {
		return false
	}
	_, ok := c.values[key]
	return ok
}

// FieldConfigListener is notified when a FieldConfig is created or modified.
// This is the Go equivalent of Lucene's FieldConfigListener.
type FieldConfigListener interface {
	// BuildFieldConfig is called whenever a FieldConfig for fieldName is needed.
	// Implementations should populate the configuration on the provided FieldConfig.
	BuildFieldConfig(fieldConfig *FieldConfig)
}

// FieldConfig holds per-field configuration for the query parser.
// It embeds AbstractQueryConfig so any *ConfigurationKey may be used.
// This is the Go equivalent of Lucene's FieldConfig.
type FieldConfig struct {
	AbstractQueryConfig
	fieldName string
}

// NewFieldConfig creates a new FieldConfig for the given field name.
func NewFieldConfig(fieldName string) *FieldConfig {
	if fieldName == "" {
		panic("fieldName must not be empty")
	}
	return &FieldConfig{
		AbstractQueryConfig: newAbstractQueryConfig(),
		fieldName:           fieldName,
	}
}

// GetField returns the field name this config applies to.
func (fc *FieldConfig) GetField() string {
	return fc.fieldName
}

// String returns a debug representation.
func (fc *FieldConfig) String() string {
	return fmt.Sprintf("FieldConfig{field=%s}", fc.fieldName)
}

// QueryConfigHandler manages global query configuration and per-field configuration.
// It extends AbstractQueryConfig with per-field overrides via registered FieldConfigListeners.
// This is the Go equivalent of Lucene's QueryConfigHandler.
type QueryConfigHandler struct {
	AbstractQueryConfig
	listeners []FieldConfigListener
}

// NewQueryConfigHandler creates a new QueryConfigHandler.
func NewQueryConfigHandler() *QueryConfigHandler {
	return &QueryConfigHandler{
		AbstractQueryConfig: newAbstractQueryConfig(),
		listeners:           make([]FieldConfigListener, 0),
	}
}

// AddFieldConfigListener adds a listener that will be consulted when a FieldConfig is requested.
func (h *QueryConfigHandler) AddFieldConfigListener(listener FieldConfigListener) {
	h.listeners = append(h.listeners, listener)
}

// GetFieldConfig returns the accumulated FieldConfig for the named field.
// All registered listeners are given a chance to populate the config.
// Returns nil if fieldName is empty.
func (h *QueryConfigHandler) GetFieldConfig(fieldName string) *FieldConfig {
	if fieldName == "" {
		return nil
	}
	fc := NewFieldConfig(fieldName)
	for _, l := range h.listeners {
		l.BuildFieldConfig(fc)
	}
	return fc
}
