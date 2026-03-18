// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents the version of Lucene (or Gocene) in the format major.minor.bugfix.
// This is the Go port of Lucene's org.apache.lucene.util.Version.
type Version struct {
	// Major version number
	Major int

	// Minor version number
	Minor int

	// Bugfix version number
	Bugfix int

	// Prerelease is an optional prerelease identifier (e.g., "alpha", "beta")
	Prerelease string
}

// Current version constants for Gocene (matching Lucene 10.x)
const (
	// LuceneVersionMajor is the major version number
	LuceneVersionMajor = 10

	// LuceneVersionMinor is the minor version number
	LuceneVersionMinor = 4

	// LuceneVersionBugfix is the bugfix version number
	LuceneVersionBugfix = 0
)

// LuceneVersion is the current Lucene version that Gocene is compatible with.
var LuceneVersion = NewVersion(LuceneVersionMajor, LuceneVersionMinor, LuceneVersionBugfix)

// NewVersion creates a new Version with the given major, minor, and bugfix numbers.
func NewVersion(major, minor, bugfix int) *Version {
	return &Version{
		Major:  major,
		Minor:  minor,
		Bugfix: bugfix,
	}
}

// NewVersionWithPrerelease creates a new Version with a prerelease identifier.
func NewVersionWithPrerelease(major, minor, bugfix int, prerelease string) *Version {
	return &Version{
		Major:      major,
		Minor:      minor,
		Bugfix:     bugfix,
		Prerelease: prerelease,
	}
}

// ParseVersion parses a version string in the format "major.minor.bugfix" or "major.minor.bugfix-prerelease".
func ParseVersion(s string) (*Version, error) {
	if s == "" {
		return nil, fmt.Errorf("empty version string")
	}

	// Split prerelease part
	parts := strings.SplitN(s, "-", 2)
	versionPart := parts[0]
	var prerelease string
	if len(parts) > 1 {
		prerelease = parts[1]
	}

	// Parse version numbers
	nums := strings.Split(versionPart, ".")
	if len(nums) < 2 || len(nums) > 3 {
		return nil, fmt.Errorf("invalid version format: %s", s)
	}

	major, err := strconv.Atoi(nums[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", nums[0])
	}

	minor, err := strconv.Atoi(nums[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", nums[1])
	}

	bugfix := 0
	if len(nums) > 2 {
		bugfix, err = strconv.Atoi(nums[2])
		if err != nil {
			return nil, fmt.Errorf("invalid bugfix version: %s", nums[2])
		}
	}

	return &Version{
		Major:      major,
		Minor:      minor,
		Bugfix:     bugfix,
		Prerelease: prerelease,
	}, nil
}

// String returns the version string in the format "major.minor.bugfix".
// If prerelease is set, it returns "major.minor.bugfix-prerelease".
func (v *Version) String() string {
	if v.Prerelease != "" {
		return fmt.Sprintf("%d.%d.%d-%s", v.Major, v.Minor, v.Bugfix, v.Prerelease)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Bugfix)
}

// CompareTo compares this version to another.
// Returns:
//   - -1 if this version is less than other
//   - 0 if this version equals other
//   - 1 if this version is greater than other
func (v *Version) CompareTo(other *Version) int {
	if other == nil {
		return 1
	}

	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Bugfix != other.Bugfix {
		if v.Bugfix < other.Bugfix {
			return -1
		}
		return 1
	}

	// Handle prerelease comparison
	// A version without prerelease is considered greater than one with prerelease
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		return strings.Compare(v.Prerelease, other.Prerelease)
	}

	return 0
}

// Equals returns true if this version equals other.
func (v *Version) Equals(other *Version) bool {
	return v.CompareTo(other) == 0
}

// OnOrAfter returns true if this version is on or after the given version.
func (v *Version) OnOrAfter(other *Version) bool {
	return v.CompareTo(other) >= 0
}

// Before returns true if this version is before the given version.
func (v *Version) Before(other *Version) bool {
	return v.CompareTo(other) < 0
}

// IsMajorVersion returns true if this version has the same major version as other.
func (v *Version) IsMajorVersion(other *Version) bool {
	if other == nil {
		return false
	}
	return v.Major == other.Major
}

// EncodedVersion returns the version encoded as an integer.
// The format is: major * 1000000 + minor * 1000 + bugfix
func (v *Version) EncodedVersion() int {
	return v.Major*1000000 + v.Minor*1000 + v.Bugfix
}

// IsAtLeast returns true if this version is at least the given major.minor.bugfix.
func (v *Version) IsAtLeast(major, minor, bugfix int) bool {
	if v.Major != major {
		return v.Major > major
	}
	if v.Minor != minor {
		return v.Minor > minor
	}
	return v.Bugfix >= bugfix
}
