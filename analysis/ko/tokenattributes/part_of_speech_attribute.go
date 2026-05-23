// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package tokenattributes provides Korean-specific token attributes.
package tokenattributes

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PartOfSpeechAttributeType is the reflect.Type of the PartOfSpeechAttribute
// interface.
var PartOfSpeechAttributeType = reflect.TypeOf((*PartOfSpeechAttribute)(nil)).Elem()

// KoreanToken is the interface for tokens that provide Korean POS and reading
// data. Both *ko.Token and its subtypes satisfy this interface.
type KoreanToken interface {
	GetPOSType() dict.POSType
	GetLeftPOS() dict.POSTag
	GetRightPOS() dict.POSTag
	GetMorphemes() []dict.Morpheme
	GetReading() string
}

// PartOfSpeechAttribute exposes part-of-speech data for Korean tokens.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.tokenattributes.PartOfSpeechAttribute from
// Apache Lucene 10.4.0.
type PartOfSpeechAttribute interface {
	// GetPOSType returns the POSType of the current token.
	GetPOSType() dict.POSType

	// GetLeftPOS returns the left POSTag of the current token.
	GetLeftPOS() dict.POSTag

	// GetRightPOS returns the right POSTag of the current token.
	GetRightPOS() dict.POSTag

	// GetMorphemes returns the morpheme decomposition of the current token.
	GetMorphemes() []dict.Morpheme

	// SetToken sets the token that drives this attribute.
	SetToken(token KoreanToken)
}

// PartOfSpeechAttributeImpl is the concrete implementation of
// PartOfSpeechAttribute.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.tokenattributes.PartOfSpeechAttributeImpl
// from Apache Lucene 10.4.0.
type PartOfSpeechAttributeImpl struct {
	token KoreanToken
}

var (
	_ util.AttributeImpl              = (*PartOfSpeechAttributeImpl)(nil)
	_ PartOfSpeechAttribute           = (*PartOfSpeechAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*PartOfSpeechAttributeImpl)(nil)
)

// NewPartOfSpeechAttributeImpl creates a new PartOfSpeechAttributeImpl.
func NewPartOfSpeechAttributeImpl() *PartOfSpeechAttributeImpl {
	return &PartOfSpeechAttributeImpl{}
}

// AttributeInterfaces satisfies util.AttributeInterfaceProvider.
func (a *PartOfSpeechAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PartOfSpeechAttributeType}
}

// GetPOSType returns the POSType of the current token.
func (a *PartOfSpeechAttributeImpl) GetPOSType() dict.POSType {
	if a.token == nil {
		return dict.POSTypeMorpheme
	}
	return a.token.GetPOSType()
}

// GetLeftPOS returns the left POSTag of the current token.
func (a *PartOfSpeechAttributeImpl) GetLeftPOS() dict.POSTag {
	if a.token == nil {
		return dict.POSTagUNKNOWN
	}
	return a.token.GetLeftPOS()
}

// GetRightPOS returns the right POSTag of the current token.
func (a *PartOfSpeechAttributeImpl) GetRightPOS() dict.POSTag {
	if a.token == nil {
		return dict.POSTagUNKNOWN
	}
	return a.token.GetRightPOS()
}

// GetMorphemes returns the morpheme decomposition of the current token.
func (a *PartOfSpeechAttributeImpl) GetMorphemes() []dict.Morpheme {
	if a.token == nil {
		return nil
	}
	return a.token.GetMorphemes()
}

// SetToken sets the token that drives this attribute.
func (a *PartOfSpeechAttributeImpl) SetToken(token KoreanToken) { a.token = token }

// Clear resets the attribute to zero state.
func (a *PartOfSpeechAttributeImpl) Clear() { a.token = nil }

// End resets to end-of-field state (same as Clear for this attribute).
func (a *PartOfSpeechAttributeImpl) End() { a.Clear() }

// CopyTo copies this attribute's state to target.
func (a *PartOfSpeechAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(PartOfSpeechAttribute)
	if !ok {
		panic("PartOfSpeechAttributeImpl.CopyTo: target must implement PartOfSpeechAttribute")
	}
	t.SetToken(a.token)
}

// CloneAttribute returns a deep clone of this impl.
func (a *PartOfSpeechAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &PartOfSpeechAttributeImpl{token: a.token}
}

// ReflectWith reports attribute values via the provided reflector.
func (a *PartOfSpeechAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	posName := a.GetPOSType().String()
	leftPOS := a.GetLeftPOS()
	rightPOS := a.GetRightPOS()
	leftStr := fmt.Sprintf("%s(%s)", leftPOS.String(), leftPOS.Description())
	rightStr := fmt.Sprintf("%s(%s)", rightPOS.String(), rightPOS.Description())
	reflector(PartOfSpeechAttributeType, "posType", posName)
	reflector(PartOfSpeechAttributeType, "leftPOS", leftStr)
	reflector(PartOfSpeechAttributeType, "rightPOS", rightStr)
	reflector(PartOfSpeechAttributeType, "morphemes", displayMorphemes(a.GetMorphemes()))
}

func displayMorphemes(morphemes []dict.Morpheme) string {
	if morphemes == nil {
		return ""
	}
	var sb strings.Builder
	for i, m := range morphemes {
		if i > 0 {
			sb.WriteByte('+')
		}
		sb.WriteString(m.SurfaceForm)
		sb.WriteByte('/')
		sb.WriteString(m.PosTag.String())
		sb.WriteByte('(')
		sb.WriteString(m.PosTag.Description())
		sb.WriteByte(')')
	}
	return sb.String()
}
