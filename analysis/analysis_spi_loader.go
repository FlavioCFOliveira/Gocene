// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var serviceNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]+$`)

// GenericAnalysisSPILoader is a generic loader for SPIs.
type GenericAnalysisSPILoader[S any] struct {
	mu            sync.RWMutex
	services      map[string]func(map[string]string) S
	originalNames []string
}

// NewGenericAnalysisSPILoader creates a new GenericAnalysisSPILoader.
func NewGenericAnalysisSPILoader[S any]() *GenericAnalysisSPILoader[S] {
	return &GenericAnalysisSPILoader[S]{
		services: make(map[string]func(map[string]string) S),
	}
}

// Reload reloads the internal SPI list.
func (l *GenericAnalysisSPILoader[S]) Reload() {
	l.mu.Lock()
	defer l.mu.Unlock()
}

// NewInstance creates a new instance of the given SPI by invoking its creator.
func (l *GenericAnalysisSPILoader[S]) NewInstance(name string, args map[string]string) S {
	creator, err := l.Lookup(name)
	if err != nil {
		panic(err)
	}
	return creator(args)
}

// Lookup finds the creator for the given SPI name.
func (l *GenericAnalysisSPILoader[S]) Lookup(name string) (func(map[string]string) S, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	creator, ok := l.services[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf(
			"a SPI class with name '%s' does not exist. The current registry supports the following names: %v",
			name,
			l.AvailableServices(),
		)
	}
	return creator, nil
}

// AvailableServices returns the list of all registered SPI names.
func (l *GenericAnalysisSPILoader[S]) AvailableServices() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.originalNames
}

// Register adds a new SPI creator to the loader.
func (l *GenericAnalysisSPILoader[S]) Register(name string, creator func(map[string]string) S) error {
	if !isValidName(name) {
		return fmt.Errorf(
			"the name %s is invalid: Allowed characters are (English) alphabet, digits, and underscore. It should be started with an alphabet",
			name,
		)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	lowerName := strings.ToLower(name)
	if _, exists := l.services[lowerName]; !exists {
		l.services[lowerName] = creator
		l.originalNames = append(l.originalNames, name)
	}
	return nil
}

func isValidName(name string) bool {
	return serviceNamePattern.MatchString(name)
}

// AnalysisSPILoader is a loader for analysis services (Tokenizers, CharFilters, TokenFilters).
type AnalysisSPILoader struct {
	mu sync.RWMutex
}

// NewAnalysisSPILoader creates a new AnalysisSPILoader.
func NewAnalysisSPILoader() *AnalysisSPILoader {
	return &AnalysisSPILoader{}
}

// AvailableServices returns a list of all available service names across all registries.
func (l *AnalysisSPILoader) AvailableServices() []string {
	tokenizerNames := AvailableTokenizers()
	charFilterNames := AvailableCharFilters()
	tokenFilterNames := AvailableTokenFilters()

	all := make(map[string]struct{})
	for _, n := range tokenizerNames {
		all[n] = struct{}{}
	}
	for _, n := range charFilterNames {
		all[n] = struct{}{}
	}
	for _, n := range tokenFilterNames {
		all[n] = struct{}{}
	}

	res := make([]string, 0, len(all))
	for n := range all {
		res = append(res, n)
	}
	return res
}

// NewInstance creates a new instance of the specified service.
func (l *AnalysisSPILoader) NewInstance(name string, params map[string]string) (any, error) {
	if tf, err := TokenizerForName(name, params); err == nil {
		return tf, nil
	}
	if cf, err := CharFilterForName(name, params); err == nil {
		return cf, nil
	}
	if tff, err := TokenFilterForName(name, params); err == nil {
		return tff, nil
	}

	return nil, fmt.Errorf("no analysis service found with name: %s", name)
}

