// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package analysis

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/analysis/AnalysisSPILoader.java

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// serviceNamePattern validates that SPI names start with an ASCII letter and
// contain only ASCII letters, digits, and underscores.
//
// Mirrors AnalysisSPILoader.SERVICE_NAME_PATTERN (Lucene 10.4.0).
var serviceNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]+$`)

// FactoryFunc is the constructor signature for all analysis factories. It
// receives the args map (which the factory may consume during construction)
// and returns the created factory value or an error.
type FactoryFunc func(args map[string]string) (any, error)

// AnalysisSPILoader is a name-keyed registry of analysis factories. It is the
// Go replacement for Java's AnalysisSPILoader, which discovers implementations
// via ServiceLoader reflection. In Go, factories are registered explicitly
// (typically in package init functions) using Register.
//
// All lookups are case-insensitive on the ASCII range, matching Java's
// name.toLowerCase(Locale.ROOT) behaviour.
//
// Mirrors org.apache.lucene.analysis.AnalysisSPILoader (Lucene 10.4.0).
//
// Deviations from Java:
//   - ServiceLoader / classpath scanning is replaced by explicit registration.
//   - lookupSPIName (reflection on a static NAME field) is replaced by the
//     registered name supplied at registration time.
//   - lookupClass returns a string name rather than a reflect.Type, since Go
//     does not expose a usable class-token equivalent.
//   - newFactoryClassInstance (constructor reflection) is replaced by the
//     FactoryFunc callback.
type AnalysisSPILoader struct {
	mu            sync.RWMutex
	services      map[string]FactoryFunc // lowercase-name → constructor
	originalNames map[string]string      // lowercase-name → original-case name
}

// NewAnalysisSPILoader creates an empty AnalysisSPILoader.
func NewAnalysisSPILoader() *AnalysisSPILoader {
	return &AnalysisSPILoader{
		services:      make(map[string]FactoryFunc),
		originalNames: make(map[string]string),
	}
}

// Register adds a factory to the registry under the given name. The name must
// start with an ASCII letter and contain only ASCII letters, digits, and
// underscores. Duplicate registrations are silently ignored (first-wins,
// matching Java's "only add the first one for each name" behaviour).
// Returns an error if the name is syntactically invalid.
//
// Mirrors AnalysisSPILoader.reload / ServiceLoader discovery (Lucene 10.4.0).
func (l *AnalysisSPILoader) Register(name string, fn FactoryFunc) error {
	if !serviceNamePattern.MatchString(name) {
		return fmt.Errorf(
			"SPI name %q is invalid: must start with a letter and contain only letters, digits, or underscore",
			name,
		)
	}
	key := strings.ToLower(name)
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.services[key]; !exists {
		l.services[key] = fn
		l.originalNames[key] = name
	}
	return nil
}

// NewInstance constructs and returns the factory registered under name (case-
// insensitive). args is forwarded to the factory constructor. Returns an error
// if no factory is registered for the given name.
//
// Mirrors AnalysisSPILoader.newInstance (Lucene 10.4.0).
func (l *AnalysisSPILoader) NewInstance(name string, args map[string]string) (any, error) {
	fn, err := l.lookupFunc(name)
	if err != nil {
		return nil, err
	}
	return fn(args)
}

// LookupName returns the original-case registered name for the given name
// (case-insensitive). Returns an error if the name is not registered.
//
// Mirrors AnalysisSPILoader.lookupClass (Lucene 10.4.0) — returns the
// registered name string instead of a reflect.Type.
func (l *AnalysisSPILoader) LookupName(name string) (string, error) {
	key := strings.ToLower(name)
	l.mu.RLock()
	defer l.mu.RUnlock()
	orig, ok := l.originalNames[key]
	if !ok {
		return "", l.lookupError(name)
	}
	return orig, nil
}

// AvailableServices returns the set of original-case registered names.
//
// Mirrors AnalysisSPILoader.availableServices (Lucene 10.4.0).
func (l *AnalysisSPILoader) AvailableServices() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	names := make([]string, 0, len(l.originalNames))
	for _, orig := range l.originalNames {
		names = append(names, orig)
	}
	return names
}

// lookupFunc returns the registered FactoryFunc for the given name.
func (l *AnalysisSPILoader) lookupFunc(name string) (FactoryFunc, error) {
	key := strings.ToLower(name)
	l.mu.RLock()
	defer l.mu.RUnlock()
	fn, ok := l.services[key]
	if !ok {
		return nil, l.lookupError(name)
	}
	return fn, nil
}

func (l *AnalysisSPILoader) lookupError(name string) error {
	return fmt.Errorf(
		"a factory with SPI name %q does not exist; available: %v",
		name, l.AvailableServices(),
	)
}

// LookupSPIName returns the registered SPI name for the given original name.
// This is the Go analogue of the Java static method
// AnalysisSPILoader.lookupSPIName(Class), which used reflection to read the
// static NAME field. In Go, the name is simply the key supplied at registration.
//
// Mirrors AnalysisSPILoader.lookupSPIName (Lucene 10.4.0).
func LookupSPIName(originalName string) string {
	return originalName
}
