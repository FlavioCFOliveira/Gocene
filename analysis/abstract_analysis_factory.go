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
//   lucene/core/src/java/org/apache/lucene/analysis/AbstractAnalysisFactory.java

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LuceneMatchVersionParam is the key used to pass the Lucene version in args.
//
// Mirrors AbstractAnalysisFactory.LUCENE_MATCH_VERSION_PARAM (Lucene 10.4.0).
const LuceneMatchVersionParam = "luceneMatchVersion"

// classNameParam and spiNameParam are consumed by the constructor and not
// retained in the processed args map. Mirrors the CLASS_NAME / SPI_NAME
// private constants in Java.
const (
	classNameParam = "class"
	spiNameParam   = "name"
)

// AbstractAnalysisFactory is the base type for analysis factory types
// (TokenizerFactory, TokenFilterFactory, CharFilterFactory). It stores the
// original args and the resolved Lucene match version, and provides a
// collection of helpers for consuming typed parameters from an args map.
//
// Mirrors org.apache.lucene.analysis.AbstractAnalysisFactory (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java's no-arg constructor throws UnsupportedOperationException; Go has
//     no such service-loader requirement. Embed AbstractAnalysisFactory and
//     call NewAbstractAnalysisFactory in the composite constructor instead.
//   - Java's getWordSet / getSnowballWordSet / getLines take a ResourceLoader
//     and are exposed here as methods that accept util.ResourceLoader; all
//     file-reading helpers are preserved.
//   - Version.parseLeniently is approximated by util.Parse (tolerant parser).
type AbstractAnalysisFactory struct {
	originalArgs               map[string]string
	luceneMatchVersion         *util.Version
	isExplicitLuceneMatchVersion bool
}

// NewAbstractAnalysisFactory initialises an AbstractAnalysisFactory from args.
// It removes the "luceneMatchVersion", "class", and "name" keys from args so
// that the caller's subsequent consumption loop only sees domain parameters.
//
// Mirrors AbstractAnalysisFactory(Map<String,String>) (Lucene 10.4.0).
func NewAbstractAnalysisFactory(args map[string]string) (*AbstractAnalysisFactory, error) {
	originalArgs := make(map[string]string, len(args))
	for k, v := range args {
		originalArgs[k] = v
	}

	f := &AbstractAnalysisFactory{originalArgs: originalArgs}

	if version, ok := args[LuceneMatchVersionParam]; ok {
		v, err := util.Parse(version)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", LuceneMatchVersionParam, err)
		}
		f.luceneMatchVersion = v
		f.isExplicitLuceneMatchVersion = true
		delete(args, LuceneMatchVersionParam)
	} else {
		f.luceneMatchVersion = util.Latest
	}

	delete(args, classNameParam)
	delete(args, spiNameParam)
	return f, nil
}

// GetOriginalArgs returns the unprocessed args as supplied to the constructor.
// The map is a defensive copy — callers must not modify it.
//
// Mirrors getOriginalArgs() (Lucene 10.4.0).
func (f *AbstractAnalysisFactory) GetOriginalArgs() map[string]string {
	cp := make(map[string]string, len(f.originalArgs))
	for k, v := range f.originalArgs {
		cp[k] = v
	}
	return cp
}

// GetLuceneMatchVersion returns the resolved Lucene version.
//
// Mirrors getLuceneMatchVersion() (Lucene 10.4.0).
func (f *AbstractAnalysisFactory) GetLuceneMatchVersion() *util.Version {
	return f.luceneMatchVersion
}

// IsExplicitLuceneMatchVersion reports whether luceneMatchVersion was
// explicitly provided in the args rather than defaulting to Latest.
//
// Mirrors isExplicitLuceneMatchVersion() (Lucene 10.4.0).
func (f *AbstractAnalysisFactory) IsExplicitLuceneMatchVersion() bool {
	return f.isExplicitLuceneMatchVersion
}

// SetExplicitLuceneMatchVersion overrides the explicit-version flag.
//
// Mirrors setExplicitLuceneMatchVersion(boolean) (Lucene 10.4.0).
func (f *AbstractAnalysisFactory) SetExplicitLuceneMatchVersion(v bool) {
	f.isExplicitLuceneMatchVersion = v
}

// GetClassArg returns the class name recorded in originalArgs, or the
// empty string if none was provided.
//
// Mirrors getClassArg() (Lucene 10.4.0).
func (f *AbstractAnalysisFactory) GetClassArg() string {
	if f.originalArgs != nil {
		if className, ok := f.originalArgs[classNameParam]; ok {
			return className
		}
	}
	return ""
}

// ─── require / get helpers ───────────────────────────────────────────────────

// Require removes and returns args[name]. Returns an error if the key is absent.
//
// Mirrors require(Map, String) (Lucene 10.4.0).
func Require(args map[string]string, name string) (string, error) {
	s, ok := args[name]
	if !ok {
		return "", fmt.Errorf("configuration error: missing parameter %q", name)
	}
	delete(args, name)
	return s, nil
}

// RequireWithAllowed removes and returns args[name], enforcing membership in
// allowedValues (case-sensitive). Returns an error if the key is absent or the
// value is not in allowedValues.
//
// Mirrors require(Map, String, Collection, boolean) (Lucene 10.4.0).
func RequireWithAllowed(args map[string]string, name string, allowedValues []string, caseSensitive bool) (string, error) {
	s, err := Require(args, name)
	if err != nil {
		return "", err
	}
	for _, allowed := range allowedValues {
		if caseSensitive && s == allowed {
			return s, nil
		}
		if !caseSensitive && strings.EqualFold(s, allowed) {
			return s, nil
		}
	}
	return "", fmt.Errorf("configuration error: %q value must be one of %v", name, allowedValues)
}

// Get removes and returns args[name], or the empty string with ok=false if
// the key is absent.
//
// Mirrors get(Map, String) (Lucene 10.4.0).
func Get(args map[string]string, name string) (string, bool) {
	s, ok := args[name]
	if ok {
		delete(args, name)
	}
	return s, ok
}

// GetWithDefault removes and returns args[name], or defaultVal if absent.
//
// Mirrors get(Map, String, String) (Lucene 10.4.0).
func GetWithDefault(args map[string]string, name, defaultVal string) string {
	s, ok := args[name]
	if !ok {
		return defaultVal
	}
	delete(args, name)
	return s
}

// GetWithAllowed removes and returns args[name] if it is in allowedValues
// (case-sensitive), or defaultVal if absent. Returns an error if the value
// is present but not in allowedValues.
//
// Mirrors get(Map, String, Collection, String, boolean) (Lucene 10.4.0).
func GetWithAllowed(args map[string]string, name string, allowedValues []string, defaultVal string, caseSensitive bool) (string, error) {
	s, ok := args[name]
	if !ok {
		return defaultVal, nil
	}
	delete(args, name)
	for _, allowed := range allowedValues {
		if caseSensitive && s == allowed {
			return s, nil
		}
		if !caseSensitive && strings.EqualFold(s, allowed) {
			return s, nil
		}
	}
	return "", fmt.Errorf("configuration error: %q value must be one of %v", name, allowedValues)
}

// RequireInt removes and parses args[name] as a decimal integer.
//
// Mirrors requireInt(Map, String) (Lucene 10.4.0).
func RequireInt(args map[string]string, name string) (int, error) {
	s, err := Require(args, name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parameter %q: %w", name, err)
	}
	return n, nil
}

// GetInt removes and parses args[name] as an integer, or returns defaultVal
// if the key is absent.
//
// Mirrors getInt(Map, String, int) (Lucene 10.4.0).
func GetInt(args map[string]string, name string, defaultVal int) (int, error) {
	s, ok := args[name]
	if !ok {
		return defaultVal, nil
	}
	delete(args, name)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parameter %q: %w", name, err)
	}
	return n, nil
}

// RequireBoolean removes and parses args[name] as a boolean.
//
// Mirrors requireBoolean(Map, String) (Lucene 10.4.0).
func RequireBoolean(args map[string]string, name string) (bool, error) {
	s, err := Require(args, name)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(s)
}

// GetBoolean removes and parses args[name] as a boolean, or returns
// defaultVal if the key is absent.
//
// Mirrors getBoolean(Map, String, boolean) (Lucene 10.4.0).
func GetBoolean(args map[string]string, name string, defaultVal bool) (bool, error) {
	s, ok := args[name]
	if !ok {
		return defaultVal, nil
	}
	delete(args, name)
	return strconv.ParseBool(s)
}

// RequireFloat removes and parses args[name] as a float64.
//
// Mirrors requireFloat(Map, String) (Lucene 10.4.0).
func RequireFloat(args map[string]string, name string) (float64, error) {
	s, err := Require(args, name)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parameter %q: %w", name, err)
	}
	return f, nil
}

// GetFloat removes and parses args[name] as a float64, or returns defaultVal
// if the key is absent.
//
// Mirrors getFloat(Map, String, float) (Lucene 10.4.0).
func GetFloat(args map[string]string, name string, defaultVal float64) (float64, error) {
	s, ok := args[name]
	if !ok {
		return defaultVal, nil
	}
	delete(args, name)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parameter %q: %w", name, err)
	}
	return f, nil
}

// RequireChar removes args[name] and returns its first rune.
// Returns an error if the value is not exactly one character.
//
// Mirrors requireChar(Map, String) (Lucene 10.4.0).
func RequireChar(args map[string]string, name string) (rune, error) {
	s, err := Require(args, name)
	if err != nil {
		return 0, err
	}
	runes := []rune(s)
	if len(runes) != 1 {
		return 0, fmt.Errorf("%s should be a char. %q is invalid", name, s)
	}
	return runes[0], nil
}

// GetChar removes args[name] and returns its first rune, or defaultValue if
// the key is absent. Returns an error if the value is not exactly one character.
//
// Mirrors getChar(Map, String, char) (Lucene 10.4.0).
func GetChar(args map[string]string, name string, defaultValue rune) (rune, error) {
	s, ok := args[name]
	if !ok {
		return defaultValue, nil
	}
	delete(args, name)
	runes := []rune(s)
	if len(runes) != 1 {
		return 0, fmt.Errorf("%s should be a char. %q is invalid", name, s)
	}
	return runes[0], nil
}

// itemPattern matches non-whitespace, non-comma substrings.
var itemPattern = regexp.MustCompile(`[^,\s]+`)

// GetSet removes args[name] and returns the whitespace- and/or
// comma-separated set of values, or nil if the key is absent.
//
// Mirrors getSet(Map, String) (Lucene 10.4.0).
func GetSet(args map[string]string, name string) map[string]struct{} {
	s, ok := args[name]
	if !ok {
		return nil
	}
	delete(args, name)
	matches := itemPattern.FindAllString(s, -1)
	if len(matches) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		set[m] = struct{}{}
	}
	return set
}

// GetPattern compiles and returns the regex named by args[name].
// Returns an error if the key is absent or the pattern is invalid.
//
// Mirrors getPattern(Map, String) (Lucene 10.4.0).
func GetPattern(args map[string]string, name string) (*regexp.Regexp, error) {
	s, err := Require(args, name)
	if err != nil {
		return nil, err
	}
	r, err := regexp.Compile(s)
	if err != nil {
		return nil, fmt.Errorf("configuration error: %q cannot be parsed: %w", name, err)
	}
	return r, nil
}

// SplitFileNames splits a comma-separated list of file names that may contain
// backslash-escaped commas.
//
// Mirrors splitFileNames(String) (Lucene 10.4.0).
func SplitFileNames(fileNames string) []string {
	return SplitAt(',', fileNames)
}

// SplitAt splits list on every unescaped occurrence of separator, removing
// the backslash escape before the separator in each item. Returns an empty
// slice for an empty or nil-equivalent list.
//
// Mirrors splitAt(char, String) (Lucene 10.4.0).
func SplitAt(separator rune, list string) []string {
	if list == "" {
		return nil
	}
	sep := string(separator)
	escaped := `\` + sep
	// Split on unescaped separators: use negative lookbehind equivalent by
	// splitting on the separator and then re-joining items that ended with
	// a backslash (the backslash-escape rule).
	raw := strings.Split(list, sep)
	var result []string
	current := ""
	for _, part := range raw {
		if strings.HasSuffix(current, `\`) {
			// The separator was escaped; join and continue accumulating.
			current = strings.TrimSuffix(current, `\`) + sep + part
		} else {
			if current != "" || len(result) > 0 {
				result = append(result, strings.ReplaceAll(current, escaped, sep))
			}
			current = part
		}
	}
	result = append(result, strings.ReplaceAll(current, escaped, sep))
	return result
}
