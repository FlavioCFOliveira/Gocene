// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package tokenattributes contains ICU-specific token attribute types.
//
// Go port of org.apache.lucene.analysis.icu.tokenattributes (Apache Lucene
// 10.4.0).
package tokenattributes

import "reflect"

// UScriptCommon is the numeric code for Unicode script COMMON, matching
// com.ibm.icu.lang.UScript.COMMON = 0.
const UScriptCommon = 0

// UScriptInherited is the numeric code for Unicode script INHERITED, matching
// com.ibm.icu.lang.UScript.INHERITED = 1.
const UScriptInherited = 1

// UScriptInvalidCode is the numeric code for an invalid script, matching
// com.ibm.icu.lang.UScript.INVALID_CODE = -1.
const UScriptInvalidCode = -1

// ScriptAttribute stores the UTR #24 script value for a token of text.
//
// Go port of org.apache.lucene.analysis.icu.tokenattributes.ScriptAttribute
// (Apache Lucene 10.4.0).
//
// Deviation: The Java interface references com.ibm.icu.lang.UScript
// constants. In Go these are plain integer constants; callers are
// expected to provide script codes from an ICU-compatible source.
type ScriptAttribute interface {
	// GetCode returns the numeric code for this script value.
	GetCode() int

	// SetCode sets the numeric code for this script value.
	SetCode(code int)

	// GetName returns the full name of this script (e.g. "Latin").
	//
	// Deviation: without an embedded ICU data table, the name is derived
	// from the scriptNames map in this package. Unknown codes return
	// "Unknown".
	GetName() string

	// GetShortName returns the abbreviated ISO 15924 script name (e.g.
	// "Latn").
	//
	// Deviation: same caveat as GetName.
	GetShortName() string
}

// ScriptAttributeType is the reflect.Type of the ScriptAttribute interface,
// used as the key in AttributeSource lookups.
var ScriptAttributeType = reflect.TypeOf((*ScriptAttribute)(nil)).Elem()
