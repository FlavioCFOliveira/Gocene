// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// Port of lucene/core/src/test/org/apache/lucene/analysis/TestAnalysisSPILoader.java
// (Apache Lucene 10.5.0).
//
// Java overload → Go name: TokenizerFactory.forName → TokenizerForName,
// TokenFilterFactory.forName → TokenFilterForName, CharFilterFactory.forName
// → CharFilterForName, available*() → Available*().
//
// Not ported: testLookupTokenizerClass, testBogusLookupTokenizerClass,
// testLookupTokenFilterClass, testBogusLookupTokenFilterClass,
// testLookupCharFilterClass, testBogusLookupCharFilterClass. They exercise
// TokenizerFactory/TokenFilterFactory/CharFilterFactory.lookupClass(String),
// which return java.lang.Class objects found through JVM service loading;
// Gocene has no rendering of that API yet.
//
// In Lucene, FakeTokenFilterFactory and FakeCharFilterFactory are registered
// through META-INF/services files on the test class path. Gocene's
// registration mechanism is RegisterTokenFilterFactory /
// RegisterCharFilterFactory; the init function below registers the fakes for
// the analysis test binary.

import (
	"slices"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func init() {
	RegisterTokenFilterFactory(FakeTokenFilterFactoryName, func(args map[string]string) TokenFilterFactory {
		f, err := newFakeTokenFilterFactory(args)
		if err != nil {
			panic(err)
		}
		return f
	})
	RegisterCharFilterFactory(FakeCharFilterFactoryName, func(args map[string]string) CharFilterFactory {
		f, err := newFakeCharFilterFactory(args)
		if err != nil {
			panic(err)
		}
		return f
	})
}

func versionArgOnly() map[string]string {
	return map[string]string{"luceneMatchVersion": util.Latest.String()}
}

func TestAnalysisSPILoader_LookupTokenizer(t *testing.T) {
	for _, name := range []string{"Standard", "STANDARD", "standard"} {
		f, err := TokenizerForName(name, versionArgOnly())
		if err != nil {
			t.Errorf("TokenizerFactory.forName(%q): %v", name, err)
			continue
		}
		if _, ok := f.(*StandardTokenizerFactory); !ok {
			t.Errorf("TokenizerFactory.forName(%q): got %T, want *StandardTokenizerFactory", name, f)
		}
	}
}

func TestAnalysisSPILoader_BogusLookupTokenizer(t *testing.T) {
	for _, name := range []string{"sdfsdfsdfdsfsdfsdf", "!(**#$U*#$*"} {
		if _, err := TokenizerForName(name, map[string]string{}); err == nil {
			t.Errorf("TokenizerFactory.forName(%q): expected IllegalArgumentException", name)
		}
	}
}

func TestAnalysisSPILoader_AvailableTokenizers(t *testing.T) {
	if !slices.Contains(AvailableTokenizers(), "standard") {
		t.Errorf("availableTokenizers() does not contain standard: %v", AvailableTokenizers())
	}
}

func TestAnalysisSPILoader_LookupTokenFilter(t *testing.T) {
	for _, name := range []string{"Fake", "FAKE", "fake"} {
		f, err := TokenFilterForName(name, versionArgOnly())
		if err != nil {
			t.Errorf("TokenFilterFactory.forName(%q): %v", name, err)
			continue
		}
		if _, ok := f.(*fakeTokenFilterFactory); !ok {
			t.Errorf("TokenFilterFactory.forName(%q): got %T, want *fakeTokenFilterFactory", name, f)
		}
	}
}

func TestAnalysisSPILoader_BogusLookupTokenFilter(t *testing.T) {
	for _, name := range []string{"sdfsdfsdfdsfsdfsdf", "!(**#$U*#$*"} {
		if _, err := TokenFilterForName(name, map[string]string{}); err == nil {
			t.Errorf("TokenFilterFactory.forName(%q): expected IllegalArgumentException", name)
		}
	}
}

func TestAnalysisSPILoader_AvailableTokenFilters(t *testing.T) {
	if !slices.Contains(AvailableTokenFilters(), "fake") {
		t.Errorf("availableTokenFilters() does not contain fake: %v", AvailableTokenFilters())
	}
}

func TestAnalysisSPILoader_LookupCharFilter(t *testing.T) {
	for _, name := range []string{"Fake", "FAKE", "fake"} {
		f, err := CharFilterForName(name, versionArgOnly())
		if err != nil {
			t.Errorf("CharFilterFactory.forName(%q): %v", name, err)
			continue
		}
		if _, ok := f.(*fakeCharFilterFactory); !ok {
			t.Errorf("CharFilterFactory.forName(%q): got %T, want *fakeCharFilterFactory", name, f)
		}
	}
}

func TestAnalysisSPILoader_BogusLookupCharFilter(t *testing.T) {
	for _, name := range []string{"sdfsdfsdfdsfsdfsdf", "!(**#$U*#$*"} {
		if _, err := CharFilterForName(name, map[string]string{}); err == nil {
			t.Errorf("CharFilterFactory.forName(%q): expected IllegalArgumentException", name)
		}
	}
}

func TestAnalysisSPILoader_AvailableCharFilters(t *testing.T) {
	if !slices.Contains(AvailableCharFilters(), "fake") {
		t.Errorf("availableCharFilters() does not contain fake: %v", AvailableCharFilters())
	}
}
