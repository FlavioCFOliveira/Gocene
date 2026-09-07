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

// AnalysisSPILoader is a helper class for loading named SPIs from the registry.
//
// This is the Go port of org.apache.lucene.analysis.AnalysisSPILoader.
type AnalysisSPILoader[S any] struct {
	mu            sync.RWMutex
	services      map[string]func(map[string]string) S
	originalNames []string
}

// NewAnalysisSPILoader creates a new AnalysisSPILoader.
func NewAnalysisSPILoader[S any]() *AnalysisSPILoader[S] {
	return &AnalysisSPILoader[S]{
		services: make(map[string]func(map[string]string) S),
	}
}

// Reload reloads the internal SPI list.
//
// In Go, discovery is typically via init() registration, so this is a no-op for fidelity
// to the Lucene API.
func (l *AnalysisSPILoader[S]) Reload() {
	l.mu.Lock()
	defer l.mu.Unlock()
}

// NewInstance creates a new instance of the given SPI by invoking its creator.
//
// This is the Go port of AnalysisSPILoader.newInstance.
func (l *AnalysisSPILoader[S]) NewInstance(name string, args map[string]string) S {
	creator, err := l.Lookup(name)
	if err != nil {
		// Lucene throws IllegalArgumentException here.
		panic(err)
	}
	return creator(args)
}

// Lookup finds the creator for the given SPI name.
//
// This is the Go port of AnalysisSPILoader.lookupClass.
func (l *AnalysisSPILoader[S]) Lookup(name string) (func(map[string]string) S, error) {
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
//
// This is the Go port of AnalysisSPILoader.availableServices.
func (l *AnalysisSPILoader[S]) AvailableServices() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.originalNames
}

// Register adds a new SPI creator to the loader.
//
// This replaces the Java ServiceLoader mechanism in Go.
func (l *AnalysisSPILoader[S]) Register(name string, creator func(map[string]string) S) error {
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
