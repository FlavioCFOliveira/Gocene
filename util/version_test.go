// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestNewVersion(t *testing.T) {
	v := NewVersion(10, 4, 0)

	if v.Major != 10 {
		t.Errorf("Expected major 10, got %d", v.Major)
	}
	if v.Minor != 4 {
		t.Errorf("Expected minor 4, got %d", v.Minor)
	}
	if v.Bugfix != 0 {
		t.Errorf("Expected bugfix 0, got %d", v.Bugfix)
	}
	if v.Prerelease != "" {
		t.Error("Expected no prerelease")
	}
}

func TestNewVersionWithPrerelease(t *testing.T) {
	v := NewVersionWithPrerelease(10, 4, 0, "alpha")

	if v.Prerelease != "alpha" {
		t.Errorf("Expected prerelease 'alpha', got '%s'", v.Prerelease)
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		version  *Version
		expected string
	}{
		{NewVersion(10, 4, 0), "10.4.0"},
		{NewVersion(9, 0, 0), "9.0.0"},
		{NewVersionWithPrerelease(10, 4, 0, "alpha"), "10.4.0-alpha"},
		{NewVersionWithPrerelease(10, 4, 0, "beta-1"), "10.4.0-beta-1"},
	}

	for _, tt := range tests {
		result := tt.version.String()
		if result != tt.expected {
			t.Errorf("String() = %s, want %s", result, tt.expected)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected *Version
		wantErr  bool
	}{
		{"10.4.0", NewVersion(10, 4, 0), false},
		{"9.0.0", NewVersion(9, 0, 0), false},
		{"10.4", NewVersion(10, 4, 0), false},
		{"10.4.0-alpha", NewVersionWithPrerelease(10, 4, 0, "alpha"), false},
		{"", nil, true},
		{"invalid", nil, true},
		{"10.invalid.0", nil, true},
		{"10.4.0.1", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !result.Equals(tt.expected) {
				t.Errorf("ParseVersion() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestVersionCompareTo(t *testing.T) {
	tests := []struct {
		a        *Version
		b        *Version
		expected int
	}{
		{NewVersion(10, 4, 0), NewVersion(10, 4, 0), 0},
		{NewVersion(10, 4, 0), NewVersion(10, 3, 0), 1},
		{NewVersion(10, 3, 0), NewVersion(10, 4, 0), -1},
		{NewVersion(10, 4, 1), NewVersion(10, 4, 0), 1},
		{NewVersion(10, 4, 0), NewVersion(10, 4, 1), -1},
		{NewVersion(11, 0, 0), NewVersion(10, 9, 9), 1},
		{NewVersion(10, 4, 0), nil, 1},
		// Prerelease comparisons
		{NewVersionWithPrerelease(10, 4, 0, "alpha"), NewVersion(10, 4, 0), -1},
		{NewVersion(10, 4, 0), NewVersionWithPrerelease(10, 4, 0, "alpha"), 1},
		{NewVersionWithPrerelease(10, 4, 0, "alpha"), NewVersionWithPrerelease(10, 4, 0, "beta"), -1},
	}

	for _, tt := range tests {
		result := tt.a.CompareTo(tt.b)
		if result != tt.expected {
			t.Errorf("CompareTo(%s, %s) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestVersionEquals(t *testing.T) {
	a := NewVersion(10, 4, 0)
	b := NewVersion(10, 4, 0)
	c := NewVersion(10, 4, 1)

	if !a.Equals(b) {
		t.Error("Expected equal versions to be equal")
	}

	if a.Equals(c) {
		t.Error("Expected different versions to not be equal")
	}
}

func TestVersionOnOrAfter(t *testing.T) {
	v := NewVersion(10, 4, 0)

	if !v.OnOrAfter(NewVersion(10, 3, 0)) {
		t.Error("Expected 10.4.0 to be on or after 10.3.0")
	}

	if !v.OnOrAfter(NewVersion(10, 4, 0)) {
		t.Error("Expected 10.4.0 to be on or after 10.4.0")
	}

	if v.OnOrAfter(NewVersion(10, 5, 0)) {
		t.Error("Expected 10.4.0 to not be on or after 10.5.0")
	}
}

func TestVersionBefore(t *testing.T) {
	v := NewVersion(10, 4, 0)

	if !v.Before(NewVersion(10, 5, 0)) {
		t.Error("Expected 10.4.0 to be before 10.5.0")
	}

	if v.Before(NewVersion(10, 4, 0)) {
		t.Error("Expected 10.4.0 to not be before 10.4.0")
	}

	if v.Before(NewVersion(10, 3, 0)) {
		t.Error("Expected 10.4.0 to not be before 10.3.0")
	}
}

func TestVersionIsMajorVersion(t *testing.T) {
	v := NewVersion(10, 4, 0)

	if !v.IsMajorVersion(NewVersion(10, 0, 0)) {
		t.Error("Expected same major version")
	}

	if v.IsMajorVersion(NewVersion(9, 4, 0)) {
		t.Error("Expected different major version")
	}

	if v.IsMajorVersion(nil) {
		t.Error("Expected false for nil")
	}
}

func TestVersionEncodedVersion(t *testing.T) {
	v := NewVersion(10, 4, 0)
	expected := 10*1000000 + 4*1000 + 0

	if v.EncodedVersion() != expected {
		t.Errorf("EncodedVersion() = %d, want %d", v.EncodedVersion(), expected)
	}
}

func TestVersionIsAtLeast(t *testing.T) {
	v := NewVersion(10, 4, 0)

	if !v.IsAtLeast(10, 4, 0) {
		t.Error("Expected 10.4.0 to be at least 10.4.0")
	}

	if !v.IsAtLeast(10, 3, 0) {
		t.Error("Expected 10.4.0 to be at least 10.3.0")
	}

	if !v.IsAtLeast(9, 0, 0) {
		t.Error("Expected 10.4.0 to be at least 9.0.0")
	}

	if v.IsAtLeast(10, 5, 0) {
		t.Error("Expected 10.4.0 to not be at least 10.5.0")
	}

	if v.IsAtLeast(11, 0, 0) {
		t.Error("Expected 10.4.0 to not be at least 11.0.0")
	}
}

func TestLuceneVersion(t *testing.T) {
	if LuceneVersion == nil {
		t.Fatal("LuceneVersion should not be nil")
	}

	if LuceneVersion.Major != LuceneVersionMajor {
		t.Errorf("Expected major %d, got %d", LuceneVersionMajor, LuceneVersion.Major)
	}

	if LuceneVersion.Minor != LuceneVersionMinor {
		t.Errorf("Expected minor %d, got %d", LuceneVersionMinor, LuceneVersion.Minor)
	}

	if LuceneVersion.Bugfix != LuceneVersionBugfix {
		t.Errorf("Expected bugfix %d, got %d", LuceneVersionBugfix, LuceneVersion.Bugfix)
	}
}
