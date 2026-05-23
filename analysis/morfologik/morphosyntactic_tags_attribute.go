// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/java/org/apache/lucene/analysis/morfologik/MorphosyntacticTagsAttribute.java

// Package morfologik provides Lucene analysis components backed by the
// Morfologik morphological analysis library.
//
// It is the Go port of org.apache.lucene.analysis.morfologik.
package morfologik

import (
	"reflect"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MorphosyntacticTagsAttribute carries morphosyntactic annotations for
// surface forms produced by [MorfologikFilter].
//
// Morfologik provides morphosyntactic annotations for surface forms. For the
// exact format and description of these, see the Morfologik project's
// documentation.
//
// This is the Go port of
// org.apache.lucene.analysis.morfologik.MorphosyntacticTagsAttribute
// (Apache Lucene 10.4.0).
type MorphosyntacticTagsAttribute interface {
	util.Attribute

	// SetTags sets the POS tag list. The default value (no-value) is nil.
	// tags is a list of POS tags corresponding to the current lemma.
	SetTags(tags []strings.Builder)

	// GetTags returns the POS tags of the term. A single word may have
	// multiple POS tags depending on interpretation; context disambiguation
	// is typically needed to determine which particular tag applies.
	GetTags() []strings.Builder

	// Clear resets the attribute to its default (nil) value.
	Clear()
}

// MorphosyntacticTagsAttributeType is the reflect.Type for
// [MorphosyntacticTagsAttribute].
var MorphosyntacticTagsAttributeType = reflect.TypeOf((*MorphosyntacticTagsAttribute)(nil)).Elem()
