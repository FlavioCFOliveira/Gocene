package config

import (
	"errors"
)

var (
	ErrKeyMustNotBeNull = errors.New("key must not be null")
)

// AbstractQueryConfig is the base of QueryConfigHandler and FieldConfig.
// It provides operations to set, unset and get configuration values.
type AbstractQueryConfig struct {
	configMap map[any]any
}

// NewAbstractQueryConfig creates a new instance of AbstractQueryConfig.
func NewAbstractQueryConfig() AbstractQueryConfig {
	return AbstractQueryConfig{
		configMap: make(map[any]any),
	}
}

// Get returns the value held by the given key.
func (a *AbstractQueryConfig) Get(key any) (any, error) {
	if key == nil {
		return nil, ErrKeyMustNotBeNull
	}

	return a.configMap[key], nil
}

// Has returns true if there is a value set with the given key, otherwise false.
func (a *AbstractQueryConfig) Has(key any) bool {
	if key == nil {
		return false
	}

	_, ok := a.configMap[key]
	return ok
}

// Set sets a key and its value. If value is nil, it calls Unset.
func (a *AbstractQueryConfig) Set(key any, value any) error {
	if key == nil {
		return ErrKeyMustNotBeNull
	}

	if value == nil {
		a.Unset(key)
	} else {
		a.configMap[key] = value
	}
	return nil
}

// Unset removes the given key and its value.
// Returns true if the key and value were set and removed, otherwise false.
func (a *AbstractQueryConfig) Unset(key any) bool {
	if key == nil {
		return false
	}

	val, ok := a.configMap[key]
	delete(a.configMap, key)
	return ok && val != nil
}
