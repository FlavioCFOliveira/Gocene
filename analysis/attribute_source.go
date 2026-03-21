// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

// AttributeSource manages a collection of Attribute implementations.
//
// This is the Go port of Lucene's org.apache.lucene.util.AttributeSource.
//
// AttributeSource is used by TokenStreams to store and retrieve attributes.
// It provides a type-safe way to access attributes by their type.
//
// In Go, we use reflect.Type as the key since we don't have Java's class literals.
// Optimized with pre-computed indices for lock-free read operations.
type AttributeSource struct {
	// attributes holds the attribute implementations by type
	attributes map[reflect.Type]AttributeImpl

	// factories holds custom factories for creating attributes
	factories map[reflect.Type]func() AttributeImpl

	// mu protects mutable fields
	mu sync.RWMutex

	// Pre-computed indices for fast lookup (atomic for thread-safe cache)
	nameIndex     atomic.Pointer[map[string]reflect.Type]
	elemNameIndex atomic.Pointer[map[string]reflect.Type]

	// dirty flag indicates if indices need rebuild
	dirty atomic.Bool
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
	attrType := reflect.TypeOf(attr)
	as.attributes[attrType] = attr
	as.dirty.Store(true)
	as.rebuildIndices()
	as.mu.Unlock()
}

// GetAttribute retrieves an attribute by its type.
// Returns nil if the attribute doesn't exist.
// Uses pre-computed indices for lock-free lookup on hot path.
func (as *AttributeSource) GetAttribute(name string) AttributeImpl {
	// Fast path: use pre-computed index
	nameIdx := as.nameIndex.Load()
	elemIdx := as.elemNameIndex.Load()

	if nameIdx != nil && elemIdx != nil && !as.dirty.Load() {
		// Lock-free lookup using cached indices
		if attrType, ok := (*nameIdx)[name]; ok {
			as.mu.RLock()
			attr := as.attributes[attrType]
			as.mu.RUnlock()
			return attr
		}
		if attrType, ok := (*elemIdx)[name]; ok {
			as.mu.RLock()
			attr := as.attributes[attrType]
			as.mu.RUnlock()
			return attr
		}
		// Try case-insensitive match
		lowerName := strings.ToLower(name)
		if attrType, ok := (*elemIdx)[lowerName]; ok {
			as.mu.RLock()
			attr := as.attributes[attrType]
			as.mu.RUnlock()
			return attr
		}
		return nil
	}

	// Slow path: rebuild indices and search
	as.mu.RLock()
	defer as.mu.RUnlock()

	// Try to find by type name
	for attrType, attr := range as.attributes {
		// Match full type string
		if attrType.String() == name {
			return attr
		}
		// Match element name
		if attrType.Elem().Name() == name {
			return attr
		}
		// Case-insensitive match for element name
		if strings.EqualFold(attrType.Elem().Name(), name) {
			return attr
		}
	}
	return nil
}

// rebuildIndices rebuilds the pre-computed lookup indices.
// Must be called with write lock held.
func (as *AttributeSource) rebuildIndices() {
	nameIdx := make(map[string]reflect.Type, len(as.attributes))
	elemIdx := make(map[string]reflect.Type, len(as.attributes)*2)

	for attrType := range as.attributes {
		nameIdx[attrType.String()] = attrType
		elemIdx[attrType.Elem().Name()] = attrType
		elemIdx[strings.ToLower(attrType.Elem().Name())] = attrType
	}

	as.nameIndex.Store(&nameIdx)
	as.elemNameIndex.Store(&elemIdx)
	as.dirty.Store(false)
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
	delete(as.attributes, attrType)
	as.dirty.Store(true)
	as.rebuildIndices()
	as.mu.Unlock()
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
// The returned State contains copies of attribute values, not references.
func (as *AttributeSource) CaptureState() *State {
	as.mu.RLock()
	defer as.mu.RUnlock()

	state := &State{
		attributes: make(map[reflect.Type]AttributeImpl, len(as.attributes)),
	}

	for attrType, attr := range as.attributes {
		// Create a copy of the attribute value
		// Use factory if available, otherwise create a new instance
		var copy AttributeImpl
		if factory, ok := as.factories[attrType]; ok {
			copy = factory()
		} else {
			// Create new instance using reflection for known types
			copy = newAttributeInstance(attrType)
		}
		if copy != nil {
			attr.CopyTo(copy)
			state.attributes[attrType] = copy
		}
	}

	return state
}

// newAttributeInstance creates a new instance of an attribute type.
func newAttributeInstance(attrType reflect.Type) AttributeImpl {
	// Create a new instance by reflectively calling the constructor
	// This handles the known attribute types
	switch attrType {
	case reflect.TypeOf(&charTermAttribute{}):
		return NewCharTermAttribute()
	case reflect.TypeOf(&offsetAttribute{}):
		return NewOffsetAttribute()
	case reflect.TypeOf(&positionIncrementAttribute{}):
		return NewPositionIncrementAttribute()
	default:
		// Try to create via reflection
		newVal := reflect.New(attrType.Elem())
		if impl, ok := newVal.Interface().(AttributeImpl); ok {
			return impl
		}
		return nil
	}
}

// RestoreState restores the attribute state from a captured state.
func (as *AttributeSource) RestoreState(state *State) {
	if state == nil {
		return
	}

	as.mu.Lock()
	for attrType, attr := range state.attributes {
		if existing, ok := as.attributes[attrType]; ok {
			// Copy values from state to existing attribute
			attr.CopyTo(existing)
		} else {
			// Add the attribute from state
			as.attributes[attrType] = attr
		}
	}
	as.dirty.Store(true)
	as.rebuildIndices()
	as.mu.Unlock()
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
