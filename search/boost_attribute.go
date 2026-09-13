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

package search

// Ported from Apache Lucene 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/search/BoostAttribute.java
//	lucene/core/src/java/org/apache/lucene/search/BoostAttributeImpl.java

import (
	"errors"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BoostAttributeType is the reflect.Type of the [BoostAttribute] interface,
// used as the lookup key for a [util.AttributeSource]. It is the Go stand-in
// for {@code BoostAttribute.class}.
var BoostAttributeType = reflect.TypeOf((*BoostAttribute)(nil)).Elem()

// DefaultBoost mirrors {@code BoostAttribute#DEFAULT_BOOST} (1.0f).
const DefaultBoost float32 = 1.0

// errBoostAttribute reports an AttributeSource that handed back an impl which
// does not satisfy [BoostAttribute]. It cannot happen with the registration in
// this file, and exists because Go has no equivalent of Java's
// {@code attClass.cast(attImpl)}.
var errBoostAttribute = errors.New("AttributeSource returned an impl that is not a BoostAttribute")

// BoostAttribute is added to a TermsEnum returned by
// MultiTermQuery.GetTermsEnumWithAttributes to update the boost on each
// returned term. This enables control of the boost factor for each matching
// term in ScoringBooleanRewrite or TopTermsRewrite mode. FuzzyQuery uses this
// to take the edit distance into account.
//
// Please note: this attribute is intended to be added only by the TermsEnum to
// itself in its constructor and consumed by the MultiTermQuery.RewriteMethod.
//
// Mirrors org.apache.lucene.search.BoostAttribute (Lucene 10.5.0). Java's
// interface extends the bare Attribute marker; the Go interface embeds
// [util.AttributeImpl] so a value obtained from
// {@code AttributeSource.AddAttribute} can be used directly, which is the
// convention the rest of Gocene's attribute surface already follows.
type BoostAttribute interface {
	util.AttributeImpl

	// SetBoost sets the boost in this attribute.
	SetBoost(boost float32)

	// GetBoost retrieves the boost, default is 1.0f.
	GetBoost() float32
}

// BoostAttributeImpl is the implementation class for [BoostAttribute].
//
// Mirrors org.apache.lucene.search.BoostAttributeImpl (Lucene 10.5.0).
type BoostAttributeImpl struct {
	util.BaseAttributeImpl
	boost float32
}

// Compile-time assertions lock in the contracts this impl participates in.
var (
	_ BoostAttribute                  = (*BoostAttributeImpl)(nil)
	_ util.AttributeImpl              = (*BoostAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*BoostAttributeImpl)(nil)
)

// NewBoostAttributeImpl creates a BoostAttributeImpl with the Lucene field
// initialiser {@code private float boost = 1.0f}.
func NewBoostAttributeImpl() *BoostAttributeImpl {
	return &BoostAttributeImpl{boost: DefaultBoost}
}

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (a *BoostAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{BoostAttributeType}
}

// SetBoost sets the boost in this attribute.
func (a *BoostAttributeImpl) SetBoost(boost float32) { a.boost = boost }

// GetBoost retrieves the boost, default is 1.0f.
func (a *BoostAttributeImpl) GetBoost() float32 { return a.boost }

// Clear reproduces {@code public void clear() { boost = 1.0f; }}.
func (a *BoostAttributeImpl) Clear() { a.boost = DefaultBoost }

// CopyTo reproduces
// {@code public void copyTo(AttributeImpl target) { ((BoostAttribute) target).setBoost(boost); }}.
func (a *BoostAttributeImpl) CopyTo(target util.AttributeImpl) {
	if t, ok := target.(BoostAttribute); ok {
		t.SetBoost(a.boost)
	}
}

// CloneAttribute returns a deep clone of this impl, mirroring
// AttributeImpl#clone().
func (a *BoostAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &BoostAttributeImpl{boost: a.boost}
}

// ReflectWith reproduces
// {@code reflector.reflect(BoostAttribute.class, "boost", boost);}.
func (a *BoostAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(BoostAttributeType, "boost", a.boost)
}

func init() {
	util.RegisterAttributeImpl(BoostAttributeType, func() util.AttributeImpl {
		return NewBoostAttributeImpl()
	})
	util.RegisterAttributeClassName(BoostAttributeType, "org.apache.lucene.search.BoostAttribute")

	util.RegisterAttributeImpl(MaxNonCompetitiveBoostAttributeType, func() util.AttributeImpl {
		return NewMaxNonCompetitiveBoostAttributeImpl()
	})
	util.RegisterAttributeClassName(
		MaxNonCompetitiveBoostAttributeType,
		"org.apache.lucene.search.MaxNonCompetitiveBoostAttribute",
	)
}
