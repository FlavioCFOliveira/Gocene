// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
)

// TokenFilterFactory is an alias for the TokenFilterFactory interface in the api package.
type TokenFilterFactory = api.TokenFilterFactory

// BaseTokenFilterFactory is a base implementation of TokenFilterFactory.
type BaseTokenFilterFactory struct {
	args map[string]string
}

// NewBaseTokenFilterFactory creates a new BaseTokenFilterFactory.
func NewBaseTokenFilterFactory(args map[string]string) *BaseTokenFilterFactory {
	return &BaseTokenFilterFactory{
		args: args,
	}
}

// Normalize returns the input unchanged by default.
func (f *BaseTokenFilterFactory) Normalize(input api.TokenStream) api.TokenStream {
	return input
}

// GetArg returns the value of the specified argument.
func (f *BaseTokenFilterFactory) GetArg(key string) string {
	return f.args[key]
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

// AvailableTokenFilters returns a list of all available token filter names.
func AvailableTokenFilters() []string {
	tokenFilterMu.RLock()
	defer tokenFilterMu.RUnlock()
	names := make([]string, 0, len(tokenFilterRegistry))
	for name := range tokenFilterRegistry {
		names = append(names, name)
	}
	return names
}
