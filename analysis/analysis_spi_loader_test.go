// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/TestAnalysisSPILoader.java
//
// Deviation: the Java test exercises Java ServiceLoader auto-discovery and
// reflection-based SPI lookups (TokenizerFactory.forName, lookupClass,
// availableTokenizers). In Go, the equivalent is explicit registration via
// AnalysisSPILoader.Register. The tests verify the same logical behaviour:
// case-insensitive lookup, first-wins registration, bogus-name errors, and
// available-services enumeration.

package analysis

import (
	"testing"
)

// newTestLoader builds a fresh loader with two registered factories: "standard"
// and "Fake" (mixed case to exercise case-insensitive lookup).
func newTestLoader(t *testing.T) *AnalysisSPILoader {
	t.Helper()
	l := NewAnalysisSPILoader()
	if err := l.Register("standard", func(args map[string]string) (any, error) {
		return "StandardFactory", nil
	}); err != nil {
		t.Fatalf("Register standard: %v", err)
	}
	if err := l.Register("Fake", func(args map[string]string) (any, error) {
		return "FakeFactory", nil
	}); err != nil {
		t.Fatalf("Register Fake: %v", err)
	}
	return l
}

// TestAnalysisSPILoader_LookupCaseInsensitive verifies that NewInstance and
// LookupName are case-insensitive, mirroring testLookupTokenizer /
// testLookupTokenFilter / testLookupCharFilter (Lucene 10.4.0).
func TestAnalysisSPILoader_LookupCaseInsensitive(t *testing.T) {
	l := newTestLoader(t)
	for _, name := range []string{"standard", "Standard", "STANDARD"} {
		v, err := l.NewInstance(name, nil)
		if err != nil {
			t.Errorf("NewInstance(%q): %v", name, err)
			continue
		}
		if v.(string) != "StandardFactory" {
			t.Errorf("NewInstance(%q): expected StandardFactory, got %v", name, v)
		}
	}
	for _, name := range []string{"fake", "Fake", "FAKE"} {
		v, err := l.NewInstance(name, nil)
		if err != nil {
			t.Errorf("NewInstance(%q): %v", name, err)
			continue
		}
		if v.(string) != "FakeFactory" {
			t.Errorf("NewInstance(%q): expected FakeFactory, got %v", name, v)
		}
	}
}

// TestAnalysisSPILoader_BogusLookup verifies that unknown names return errors,
// mirroring testBogusLookupTokenizer / etc. (Lucene 10.4.0).
func TestAnalysisSPILoader_BogusLookup(t *testing.T) {
	l := newTestLoader(t)
	for _, name := range []string{"sdfsdfsdfdsfsdfsdf", "!(**#$U*#$*", ""} {
		if _, err := l.NewInstance(name, nil); err == nil {
			t.Errorf("NewInstance(%q): expected error for bogus name", name)
		}
		if _, err := l.LookupName(name); err == nil {
			t.Errorf("LookupName(%q): expected error for bogus name", name)
		}
	}
}

// TestAnalysisSPILoader_LookupName mirrors testLookupTokenizerClass /
// testLookupTokenFilterClass / testLookupCharFilterClass (Lucene 10.4.0).
func TestAnalysisSPILoader_LookupName(t *testing.T) {
	l := newTestLoader(t)
	for _, name := range []string{"standard", "Standard", "STANDARD"} {
		orig, err := l.LookupName(name)
		if err != nil {
			t.Errorf("LookupName(%q): %v", name, err)
			continue
		}
		if orig != "standard" {
			t.Errorf("LookupName(%q): expected original name 'standard', got %q", name, orig)
		}
	}
}

// TestAnalysisSPILoader_AvailableServices verifies that all registered names
// appear in AvailableServices, mirroring testAvailableTokenizers /
// testAvailableTokenFilters / testAvailableCharFilters (Lucene 10.4.0).
func TestAnalysisSPILoader_AvailableServices(t *testing.T) {
	l := newTestLoader(t)
	services := l.AvailableServices()
	has := func(name string) bool {
		for _, s := range services {
			if s == name {
				return true
			}
		}
		return false
	}
	if !has("standard") {
		t.Errorf("expected 'standard' in AvailableServices, got %v", services)
	}
	if !has("Fake") {
		t.Errorf("expected 'Fake' in AvailableServices, got %v", services)
	}
}

// TestAnalysisSPILoader_FirstWins verifies that duplicate registrations are
// silently ignored and the first-registered factory is used.
func TestAnalysisSPILoader_FirstWins(t *testing.T) {
	l := NewAnalysisSPILoader()
	_ = l.Register("alpha", func(args map[string]string) (any, error) { return "first", nil })
	_ = l.Register("alpha", func(args map[string]string) (any, error) { return "second", nil })
	v, err := l.NewInstance("alpha", nil)
	if err != nil || v.(string) != "first" {
		t.Fatalf("expected first-registered factory, got %v err=%v", v, err)
	}
}

// TestAnalysisSPILoader_InvalidName verifies that an invalid SPI name is
// rejected at registration time.
func TestAnalysisSPILoader_InvalidName(t *testing.T) {
	l := NewAnalysisSPILoader()
	for _, bad := range []string{"!bad", "0startdigit", ""} {
		if err := l.Register(bad, func(args map[string]string) (any, error) { return nil, nil }); err == nil {
			t.Errorf("Register(%q): expected error for invalid name", bad)
		}
	}
}

// TestAnalysisSPILoader_ArgsForwarded verifies that the args map is forwarded
// to the factory constructor.
func TestAnalysisSPILoader_ArgsForwarded(t *testing.T) {
	l := NewAnalysisSPILoader()
	_ = l.Register("echo", func(args map[string]string) (any, error) {
		return args["key"], nil
	})
	v, err := l.NewInstance("echo", map[string]string{"key": "value"})
	if err != nil || v.(string) != "value" {
		t.Fatalf("expected args to be forwarded, got %v err=%v", v, err)
	}
}

// TestLookupSPIName verifies the Go replacement for the Java
// AnalysisSPILoader.lookupSPIName reflection helper.
func TestLookupSPIName(t *testing.T) {
	if got := LookupSPIName("standard"); got != "standard" {
		t.Fatalf("LookupSPIName: expected 'standard', got %q", got)
	}
}
