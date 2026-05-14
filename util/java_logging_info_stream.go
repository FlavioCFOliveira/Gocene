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

package util

import (
	"context"
	"log/slog"
	"sync"
)

// JavaLoggingInfoStream is the Go analogue of
// org.apache.lucene.util.JavaLoggingInfoStream. The Java implementation
// bridges InfoStream messages onto java.util.logging (JUL); the Go port
// bridges them onto log/slog instead, which is the closest idiomatic
// equivalent in the Go standard library.
//
// Divergence from Lucene 10.4.0:
//
//   - Java uses java.util.logging.Level and java.util.logging.Logger;
//     the Go port uses slog.Level and slog.Logger.
//   - Java caches one Logger per component name in a ConcurrentHashMap.
//     The Go port caches one *slog.Logger per component in a
//     sync.Map, achieving the same per-component-lazy-init behaviour
//     without the lock overhead of sync.RWMutex.
//   - The Java overload {@code new JavaLoggingInfoStream(Function<String, String>, Level)}
//     accepts a component-to-logger-name function. The Go port mirrors
//     this with the ComponentToName field of type func(component string)
//     string. When nil, the default prefix "org.apache.lucene." is
//     applied (matching the Java zero-argument overload).
//   - The Java implementation calls {@code logger.logp(level, null, null, message)}
//     so JUL never inspects the stack. The Go port emits via
//     slog.Logger.Log with a LogAttrs payload carrying the component
//     name as a structured attribute, which is the slog idiom for the
//     same "no stack frame inspection" behaviour.
type JavaLoggingInfoStream struct {
	// ComponentToName converts a component name to the slog logger name
	// used for that component. When nil, "org.apache.lucene." + component
	// is used, matching Lucene's default prefix.
	ComponentToName func(component string) string

	// Level is the slog.Level at which every Message call is emitted.
	Level slog.Level

	// Base is the slog.Logger that emits the bridged messages. When nil,
	// slog.Default() is used.
	Base *slog.Logger

	cache sync.Map // string -> *slog.Logger
}

// NewJavaLoggingInfoStream returns a JavaLoggingInfoStream configured
// with the default prefix "org.apache.lucene." and the given level.
// This is the closest Go analogue to the
// {@code new JavaLoggingInfoStream(Level)} Java constructor.
func NewJavaLoggingInfoStream(level slog.Level) *JavaLoggingInfoStream {
	return &JavaLoggingInfoStream{Level: level}
}

// NewJavaLoggingInfoStreamWithPrefix mirrors the Java overload
// {@code new JavaLoggingInfoStream(String namePrefix, Level level)}. The
// prefix may be empty or must end with a dot, matching the Lucene
// contract. The contract is asserted via panic to surface programming
// errors at construction time.
func NewJavaLoggingInfoStreamWithPrefix(namePrefix string, level slog.Level) *JavaLoggingInfoStream {
	if namePrefix != "" && namePrefix[len(namePrefix)-1] != '.' {
		panic("namePrefix must be empty or end with a dot")
	}
	return &JavaLoggingInfoStream{
		ComponentToName: func(c string) string { return namePrefix + c },
		Level:           level,
	}
}

// NewJavaLoggingInfoStreamWithMapper mirrors the Java overload
// {@code new JavaLoggingInfoStream(Function<String, String>, Level)}.
// The mapper is invoked exactly once per unseen component name.
func NewJavaLoggingInfoStreamWithMapper(mapper func(component string) string, level slog.Level) *JavaLoggingInfoStream {
	if mapper == nil {
		panic("mapper must not be nil")
	}
	return &JavaLoggingInfoStream{ComponentToName: mapper, Level: level}
}

// Message bridges a Lucene InfoStream message onto slog. It is a no-op
// when the resolved logger reports the configured level as disabled.
func (s *JavaLoggingInfoStream) Message(component, message string) {
	logger := s.logger(component)
	ctx := context.Background()
	if !logger.Enabled(ctx, s.Level) {
		return
	}
	logger.LogAttrs(ctx, s.Level, message, slog.String("component", component))
}

// IsEnabled reports whether the resolved logger for the component is
// enabled at the configured slog level.
func (s *JavaLoggingInfoStream) IsEnabled(component string) bool {
	return s.logger(component).Enabled(context.Background(), s.Level)
}

// Close empties the per-component logger cache. The slog handlers
// referenced by the loggers are unaffected.
func (s *JavaLoggingInfoStream) Close() error {
	s.cache.Range(func(k, _ any) bool {
		s.cache.Delete(k)
		return true
	})
	return nil
}

// logger resolves and caches the slog.Logger for the given component.
func (s *JavaLoggingInfoStream) logger(component string) *slog.Logger {
	if v, ok := s.cache.Load(component); ok {
		return v.(*slog.Logger)
	}
	base := s.Base
	if base == nil {
		base = slog.Default()
	}
	name := s.componentToName(component)
	// slog has no "named logger" concept; we attach the name as a
	// permanent attribute so downstream handlers see what Lucene's JUL
	// would have routed via its Logger hierarchy.
	derived := base.With(slog.String("logger", name))
	actual, _ := s.cache.LoadOrStore(component, derived)
	return actual.(*slog.Logger)
}

// componentToName runs the configured ComponentToName mapper, defaulting
// to the Lucene "org.apache.lucene." prefix when no mapper is set.
func (s *JavaLoggingInfoStream) componentToName(component string) string {
	if s.ComponentToName == nil {
		return "org.apache.lucene." + component
	}
	return s.ComponentToName(component)
}

// Compile-time assertion that JavaLoggingInfoStream satisfies InfoStream.
var _ InfoStream = (*JavaLoggingInfoStream)(nil)
