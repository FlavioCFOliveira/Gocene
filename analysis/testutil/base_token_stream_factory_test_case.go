// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenStreamFactoryTestCase provides helper methods for testing analysis factories.
//
// This is the Go port of org.apache.lucene.tests.analysis.BaseTokenStreamFactoryTestCase
// from Apache Lucene 10.5.0.
type TokenStreamFactoryTestCase struct {
	T TestingT
}

// NewTokenStreamFactoryTestCase creates a new helper.
func NewTokenStreamFactoryTestCase(t TestingT) *TokenStreamFactoryTestCase {
	return &TokenStreamFactoryTestCase{T: t}
}

// tokenizerFactory returns a fully initialized TokenizerFactory with the specified
// name and key-value arguments.
func (tc *TokenStreamFactoryTestCase) tokenizerFactory(name string, keysAndValues ...string) analysis.TokenizerFactory {
	args := tc.parseArgs(keysAndValues...)
	factory, err := analysis.ForName(name, args)
	if err != nil {
		tc.T.Fatalf("tokenizerFactory(%q) failed: %v", name, err)
	}
	return factory
}

// tokenizerFactoryWithVersion returns a fully initialized TokenizerFactory with the
// specified name, version, and key-value arguments.
func (tc *TokenStreamFactoryTestCase) tokenizerFactoryWithVersion(name string, version *util.Version, keysAndValues ...string) analysis.TokenizerFactory {
	args := tc.parseArgs(keysAndValues...)
	if version != nil {
		args["luceneMatchVersion"] = version.String()
	}
	factory, err := analysis.ForName(name, args)
	if err != nil {
		tc.T.Fatalf("tokenizerFactory(%q, version=%v) failed: %v", name, version, err)
	}
	return factory
}

// tokenFilterFactory returns a fully initialized TokenFilterFactory with the specified
// name and key-value arguments.
func (tc *TokenStreamFactoryTestCase) tokenFilterFactory(name string, keysAndValues ...string) analysis.TokenFilterFactory {
	args := tc.parseArgs(keysAndValues...)
	factory, err := analysis.ForName(name, args)
	if err != nil {
		tc.T.Fatalf("tokenFilterFactory(%q) failed: %v", name, err)
	}
	return factory
}

// tokenFilterFactoryWithVersion returns a fully initialized TokenFilterFactory with the
// specified name, version, and key-value arguments.
func (tc *TokenStreamFactoryTestCase) tokenFilterFactoryWithVersion(name string, version *util.Version, keysAndValues ...string) analysis.TokenFilterFactory {
	args := tc.parseArgs(keysAndValues...)
	if version != nil {
		args["luceneMatchVersion"] = version.String()
	}
	factory, err := analysis.ForName(name, args)
	if err != nil {
		tc.T.Fatalf("tokenFilterFactory(%q, version=%v) failed: %v", name, version, err)
	}
	return factory
}

// charFilterFactory returns a fully initialized CharFilterFactory with the specified
// name and key-value arguments.
func (tc *TokenStreamFactoryTestCase) charFilterFactory(name string, keysAndValues ...string) analysis.CharFilterFactory {
	args := tc.parseArgs(keysAndValues...)
	factory, err := analysis.ForName(name, args)
	if err != nil {
		tc.T.Fatalf("charFilterFactory(%q) failed: %v", name, err)
	}
	return factory
}

// charFilterFactoryWithVersion returns a fully initialized CharFilterFactory with the
// specified name, version, and key-value arguments.
func (tc *TokenStreamFactoryTestCase) charFilterFactoryWithVersion(name string, version *util.Version, keysAndValues ...string) analysis.CharFilterFactory {
	args := tc.parseArgs(keysAndValues...)
	if version != nil {
		args["luceneMatchVersion"] = version.String()
	}
	factory, err := analysis.ForName(name, args)
	if err != nil {
		tc.T.Fatalf("charFilterFactory(%q, version=%v) failed: %v", name, version, err)
	}
	return factory
}

func (tc *TokenStreamFactoryTestCase) parseArgs(keysAndValues ...string) map[string]string {
	if len(keysAndValues)%2 == 1 {
		panic(fmt.Sprintf("invalid keysAndValues map: length %d", len(keysAndValues)))
	}
	args := make(map[string]string)
	for i := 0; i < len(keysAndValues); i += 2 {
		k := keysAndValues[i]
		v := keysAndValues[i+1]
		if _, exists := args[k]; exists {
			tc.T.Fatalf("duplicate values for key: %s", k)
		}
		args[k] = v
	}
	return args
}
