// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"io"
	"sync"
)

// CharFilterFactory is the interface for factories that create CharFilter instances.
// This is the Go port of Lucene's org.apache.lucene.analysis.CharFilterFactory.
type CharFilterFactory interface {
	// Create wraps the given Reader with a CharFilter.
	Create(input io.Reader) io.Reader
	// Normalize normalizes the specified input Reader.
	// While the default implementation returns input unchanged, char filters
	// that should be applied at normalization time can delegate to Create method.
	Normalize(input io.Reader) io.Reader
}

var (
	charFilterRegistry = make(map[string]func(map[string]string) CharFilterFactory)
	charFilterMu       sync.RWMutex
)

// RegisterCharFilterFactory registers a char filter factory creator.
func RegisterCharFilterFactory(name string, creator func(map[string]string) CharFilterFactory) {
	charFilterMu.Lock()
	defer charFilterMu.Unlock()
	charFilterRegistry[name] = creator
}

// CharFilterForName looks up a char filter factory by name from the registry.
func CharFilterForName(name string, args map[string]string) (CharFilterFactory, error) {
	charFilterMu.RLock()
	creator, ok := charFilterRegistry[name]
	charFilterMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("char filter factory not found: %s", name)
	}
	return creator(args), nil
}

// AvailableCharFilters returns a list of all available char filter names.
func AvailableCharFilters() []string {
	charFilterMu.RLock()
	defer charFilterMu.RUnlock()
	names := make([]string, 0, len(charFilterRegistry))
	for name := range charFilterRegistry {
		names = append(names, name)
	}
	return names
}

// BaseCharFilterFactory is a base implementation of CharFilterFactory.
type BaseCharFilterFactory struct {
	args map[string]string
}

// NewBaseCharFilterFactory creates a new BaseCharFilterFactory.
func NewBaseCharFilterFactory(args map[string]string) *BaseCharFilterFactory {
	return &BaseCharFilterFactory{
		args: args,
	}
}

// Normalize returns the input unchanged by default.
func (f *BaseCharFilterFactory) Normalize(input io.Reader) io.Reader {
	return input
}

// GetArg returns the value of the specified argument.
func (f *BaseCharFilterFactory) GetArg(key string) string {
	return f.args[key]
}
