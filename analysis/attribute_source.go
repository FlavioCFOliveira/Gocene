// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"sync"
)

// AttributeSource manages a collection of Attribute implementations.
//
// This is the Go port of Lucene's org.apache.lucene.util.AttributeSource.
//
// AttributeSource is used by TokenStreams to store and retrieve attributes.
// It provides a type-safe way to access attributes by their type.
//
// In Go, we use reflect.Type as the key since we don't have Java's class literals.
type AttributeSource struct {
	// attributes holds the attribute implementations by type
	attributes map[reflect.Type]AttributeImpl

	// factories holds custom factories for creating attributes
	factories map[reflect.Type]func() AttributeImpl

	// mu protects mutable fields
	mu sync.RWMutex
}

// NewAttributeSource creates a new empty AttributeSource.
func NewAttributeSource() *AttributeSource {
	return &AttributeSource{
		attributes: make(map[reflect.Type]AttributeImpl),
		factories:  make(map[reflect.Type]func() AttributeImpl),
	}
}

// AddAttribute adds an attribute implementation to this source.
// If an attribute of this type already exists, it is replaced.
func (as *AttributeSource) AddAttribute(attr AttributeImpl) {
	if attr == nil {
		return
	}

	as.mu.Lock()
	defer as.mu.Unlock()

	attrType := reflect.TypeOf(attr)
	as.attributes[attrType] = attr
}

// GetAttribute retrieves an attribute by its type.
// Returns nil if the attribute doesn't exist.
func (as *AttributeSource) GetAttribute(name string) AttributeImpl {
	as.mu.RLock()
	defer as.mu.RUnlock()

	// Try to find by type name
	for attrType, attr := range as.attributes {
		if attrType.String() == name || attrType.Elem().Name() == name {
			return attr
		}
	}
	return nil
}

// GetAttributeByType retrieves an attribute by its reflect.Type.
// Returns nil if the attribute doesn't exist.
func (as *AttributeSource) GetAttributeByType(attrType reflect.Type) AttributeImpl {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.attributes[attrType]
}

// HasAttribute checks if an attribute of the given type exists.
func (as *AttributeSource) HasAttribute(attrType reflect.Type) bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	_, exists := as.attributes[attrType]
	return exists
}

// ClearAttributes clears all attributes.
func (as *AttributeSource) ClearAttributes() {
	as.mu.Lock()
	defer as.mu.Unlock()

	for _, attr := range as.attributes {
		attr.Clear()
	}
}

// RemoveAttribute removes an attribute by type.
func (as *AttributeSource) RemoveAttribute(attrType reflect.Type) {
	as.mu.Lock()
	defer as.mu.Unlock()
	delete(as.attributes, attrType)
}

// GetAttributeClasses returns all attribute types currently stored.
func (as *AttributeSource) GetAttributeClasses() []reflect.Type {
	as.mu.RLock()
	defer as.mu.RUnlock()

	classes := make([]reflect.Type, 0, len(as.attributes))
	for attrType := range as.attributes {
		classes = append(classes, attrType)
	}
	return classes
}

// CaptureState captures the current state of all attributes.
// Returns a State that can be restored later.
func (as *AttributeSource) CaptureState() *State {
	as.mu.RLock()
	defer as.mu.RUnlock()

	state := &State{
		attributes: make(map[reflect.Type]AttributeImpl, len(as.attributes)),
	}

	for attrType, attr := range as.attributes {
		// Clone the attribute
		// Note: This is a shallow copy; implementations should implement Clone if needed
		state.attributes[attrType] = attr
	}

	return state
}

// RestoreState restores the attribute state from a captured state.
func (as *AttributeSource) RestoreState(state *State) {
	if state == nil {
		return
	}

	as.mu.Lock()
	defer as.mu.Unlock()

	for attrType, attr := range state.attributes {
		if existing, ok := as.attributes[attrType]; ok {
			// Copy values from state to existing attribute
			attr.CopyTo(existing)
		} else {
			// Add the attribute from state
			as.attributes[attrType] = attr
		}
	}
}

// RegisterFactory registers a factory function for creating attributes.
// This allows lazy creation of attributes.
func (as *AttributeSource) RegisterFactory(attrType reflect.Type, factory func() AttributeImpl) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.factories[attrType] = factory
}

// GetOrCreateAttribute gets an existing attribute or creates one using the registered factory.
func (as *AttributeSource) GetOrCreateAttribute(attrType reflect.Type) AttributeImpl {
	as.mu.RLock()
	attr, exists := as.attributes[attrType]
	as.mu.RUnlock()

	if exists {
		return attr
	}

	as.mu.Lock()
	defer as.mu.Unlock()

	// Double-check after acquiring write lock
	if attr, exists = as.attributes[attrType]; exists {
		return attr
	}

	// Create using factory if available
	if factory, ok := as.factories[attrType]; ok {
		attr = factory()
		as.attributes[attrType] = attr
		return attr
	}

	return nil
}

// Clone creates a shallow copy of this AttributeSource.
func (as *AttributeSource) Clone() *AttributeSource {
	as.mu.RLock()
	defer as.mu.RUnlock()

	clone := NewAttributeSource()
	for attrType, attr := range as.attributes {
		clone.attributes[attrType] = attr
	}
	for attrType, factory := range as.factories {
		clone.factories[attrType] = factory
	}
	return clone
}

// State captures the state of attributes for later restoration.
type State struct {
	attributes map[reflect.Type]AttributeImpl
}
