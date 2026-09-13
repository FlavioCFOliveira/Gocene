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
//	lucene/core/src/java/org/apache/lucene/search/MaxNonCompetitiveBoostAttribute.java
//	lucene/core/src/java/org/apache/lucene/search/MaxNonCompetitiveBoostAttributeImpl.java

import (
	"errors"
	"math"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MaxNonCompetitiveBoostAttributeType is the reflect.Type of the
// [MaxNonCompetitiveBoostAttribute] interface, used as the lookup key for a
// [util.AttributeSource]. It is the Go stand-in for
// {@code MaxNonCompetitiveBoostAttribute.class}.
var MaxNonCompetitiveBoostAttributeType = reflect.TypeOf((*MaxNonCompetitiveBoostAttribute)(nil)).Elem()

// errMaxNonCompetitiveBoostAttribute reports an AttributeSource that handed
// back an impl which does not satisfy [MaxNonCompetitiveBoostAttribute]. It
// cannot happen with the registration in boost_attribute.go, and exists
// because Go has no equivalent of Java's {@code attClass.cast(attImpl)}.
var errMaxNonCompetitiveBoostAttribute = errors.New(
	"AttributeSource returned an impl that is not a MaxNonCompetitiveBoostAttribute")

// MaxNonCompetitiveBoostAttribute is added to a fresh AttributeSource before
// calling MultiTermQuery.GetTermsEnumWithAttributes. A TermsEnum can use it to
// inform about the current boost that is not competitive with the collected
// terms, so it may optimize and skip terms below that boost.
//
// Mirrors org.apache.lucene.search.MaxNonCompetitiveBoostAttribute (Lucene
// 10.5.0). Java's interface extends the bare Attribute marker; the Go
// interface embeds [util.AttributeImpl] so a value obtained from
// {@code AttributeSource.AddAttribute} can be used directly, which is the
// convention the rest of Gocene's attribute surface already follows.
//
// PORT NOTE: Java's competitive term is a BytesRef; Gocene carries the same
// bytes as a []byte. The two hold identical content, so the observable
// behaviour is unchanged.
type MaxNonCompetitiveBoostAttribute interface {
	util.AttributeImpl

	// SetMaxNonCompetitiveBoost is called by the TermsEnum's consumer to
	// record the highest boost that is not competitive.
	SetMaxNonCompetitiveBoost(maxNonCompetitiveBoost float32)

	// GetMaxNonCompetitiveBoost retrieves the current maximum non-competitive
	// boost, defaulting to negative infinity.
	GetMaxNonCompetitiveBoost() float32

	// SetCompetitiveTerm is called by the TermsEnum's consumer to record the
	// term that triggered the boost change.
	SetCompetitiveTerm(competitiveTerm []byte)

	// GetCompetitiveTerm retrieves the term that triggered the last boost
	// change, or nil.
	GetCompetitiveTerm() []byte
}

// MaxNonCompetitiveBoostAttributeImpl is the implementation class for
// [MaxNonCompetitiveBoostAttribute].
//
// Mirrors org.apache.lucene.search.MaxNonCompetitiveBoostAttributeImpl
// (Lucene 10.5.0).
type MaxNonCompetitiveBoostAttributeImpl struct {
	util.BaseAttributeImpl
	maxNonCompetitiveBoost float32
	competitiveTerm        []byte
}

// Compile-time assertions lock in the contracts this impl participates in.
var (
	_ MaxNonCompetitiveBoostAttribute = (*MaxNonCompetitiveBoostAttributeImpl)(nil)
	_ util.AttributeImpl              = (*MaxNonCompetitiveBoostAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*MaxNonCompetitiveBoostAttributeImpl)(nil)
)

// NewMaxNonCompetitiveBoostAttributeImpl creates a new instance with the
// Lucene field initialisers
// {@code maxNonCompetitiveBoost = Float.NEGATIVE_INFINITY} and
// {@code competitiveTerm = null}.
func NewMaxNonCompetitiveBoostAttributeImpl() *MaxNonCompetitiveBoostAttributeImpl {
	return &MaxNonCompetitiveBoostAttributeImpl{
		maxNonCompetitiveBoost: float32(math.Inf(-1)),
	}
}

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (a *MaxNonCompetitiveBoostAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{MaxNonCompetitiveBoostAttributeType}
}

// SetMaxNonCompetitiveBoost updates the boost threshold.
func (a *MaxNonCompetitiveBoostAttributeImpl) SetMaxNonCompetitiveBoost(v float32) {
	a.maxNonCompetitiveBoost = v
}

// GetMaxNonCompetitiveBoost returns the current threshold.
func (a *MaxNonCompetitiveBoostAttributeImpl) GetMaxNonCompetitiveBoost() float32 {
	return a.maxNonCompetitiveBoost
}

// SetCompetitiveTerm updates the recorded triggering term.
func (a *MaxNonCompetitiveBoostAttributeImpl) SetCompetitiveTerm(term []byte) {
	a.competitiveTerm = term
}

// GetCompetitiveTerm returns the triggering term.
func (a *MaxNonCompetitiveBoostAttributeImpl) GetCompetitiveTerm() []byte {
	return a.competitiveTerm
}

// Clear reproduces
//
//	maxNonCompetitiveBoost = Float.NEGATIVE_INFINITY;
//	competitiveTerm = null;
func (a *MaxNonCompetitiveBoostAttributeImpl) Clear() {
	a.maxNonCompetitiveBoost = float32(math.Inf(-1))
	a.competitiveTerm = nil
}

// CopyTo reproduces
//
//	final MaxNonCompetitiveBoostAttributeImpl t = (MaxNonCompetitiveBoostAttributeImpl) target;
//	t.setMaxNonCompetitiveBoost(maxNonCompetitiveBoost);
//	t.setCompetitiveTerm(competitiveTerm);
func (a *MaxNonCompetitiveBoostAttributeImpl) CopyTo(target util.AttributeImpl) {
	if t, ok := target.(MaxNonCompetitiveBoostAttribute); ok {
		t.SetMaxNonCompetitiveBoost(a.maxNonCompetitiveBoost)
		t.SetCompetitiveTerm(a.competitiveTerm)
	}
}

// CloneAttribute returns a deep clone of this impl, mirroring
// AttributeImpl#clone().
func (a *MaxNonCompetitiveBoostAttributeImpl) CloneAttribute() util.AttributeImpl {
	clone := &MaxNonCompetitiveBoostAttributeImpl{
		maxNonCompetitiveBoost: a.maxNonCompetitiveBoost,
	}
	if a.competitiveTerm != nil {
		clone.competitiveTerm = append([]byte(nil), a.competitiveTerm...)
	}
	return clone
}

// ReflectWith reproduces
//
//	reflector.reflect(MaxNonCompetitiveBoostAttribute.class, "maxNonCompetitiveBoost", maxNonCompetitiveBoost);
//	reflector.reflect(MaxNonCompetitiveBoostAttribute.class, "competitiveTerm", competitiveTerm);
func (a *MaxNonCompetitiveBoostAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(MaxNonCompetitiveBoostAttributeType, "maxNonCompetitiveBoost", a.maxNonCompetitiveBoost)
	reflector(MaxNonCompetitiveBoostAttributeType, "competitiveTerm", a.competitiveTerm)
}
