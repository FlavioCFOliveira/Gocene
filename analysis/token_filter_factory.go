// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"sync"
)

// TokenFilterFactory is the interface for factories that create TokenFilter instances.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TokenFilterFactory.
type TokenFilterFactory interface {
	// Create creates a TokenFilter wrapping the given input.
	Create(input TokenStream) TokenFilter

	// Normalize normalizes the specified input TokenStream.
	// While the default implementation returns input unchanged, filters
	// that should be applied at normalization time can delegate to Create.
	Normalize(input TokenStream) TokenStream
}

// BaseTokenFilterFactory provides a base implementation for TokenFilterFactory.
//
// Embed this struct in concrete TokenFilterFactory implementations to inherit
// the default Normalize behavior.
type BaseTokenFilterFactory struct{}

// Normalize normalizes the specified input TokenStream.
// The default implementation returns input unchanged.
func (f *BaseTokenFilterFactory) Normalize(input TokenStream) TokenStream {
	return input
}

var (
	tokenFilterRegistry = make(map[string]func(map[string]string) TokenFilterFactory)
	tokenFilterMu       sync.RWMutex
)

// RegisterTokenFilterFactory registers a token filter factory creator.
func RegisterTokenFilterFactory(name string, creator func(map[string]string) TokenFilterFactory) {
	tokenFilterMu.Lock()
	defer tokenFilterMu.Unlock()
	tokenFilterRegistry[name] = creator
}

// TokenFilterForName looks up a token filter factory by name from the registry.
func TokenFilterForName(name string, args map[string]string) (TokenFilterFactory, error) {
	tokenFilterMu.RLock()
	creator, ok := tokenFilterRegistry[name]
	tokenFilterMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("token filter factory not found: %s", name)
	}
	return creator(args), nil
}

// AvailableTokenFilters returns a list of all available token filter names from the registry.
func AvailableTokenFilters() []string {
	tokenFilterMu.RLock()
	defer tokenFilterMu.RUnlock()
	names := make([]string, 0, len(tokenFilterRegistry))
	for name := range tokenFilterRegistry {
		names = append(names, name)
	}
	return names
}
