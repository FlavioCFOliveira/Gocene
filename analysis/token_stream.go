// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analysis

import (

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DefaultTokenAttributeFactory is the default AttributeFactory instance that
// should be used for TokenStreams.
//
// It mirrors Lucene's DEFAULT_TOKEN_ATTRIBUTE_FACTORY.
var DefaultTokenAttributeFactory = util.NewStaticImplementationAttributeFactory(
	util.DefaultAttributeFactoryInstance,
	func() util.AttributeImpl {
		return NewPackedTokenAttributeImpl()
	},
)

// TokenStream enumerates the sequence of tokens, either from Fields of a
// Document or from query text.

const POS_SEP = -1
const HOLE = -2

// This is a port of org.apache.lucene.analysis.TokenStream.
//
// A TokenStream extends AttributeSource, which provides access to all of the
// token Attributes for the TokenStream. Note that only one instance per
// AttributeImpl is created and reused for every token.
type TokenStream = api.TokenStream

// BaseTokenStream provides the basic implementation of TokenStream.
// It is intended to be embedded in concrete TokenStream implementations.
type BaseTokenStream struct {
	*util.AttributeSource
}

// NewBaseTokenStream returns a BaseTokenStream using the default attribute factory.
func NewBaseTokenStream() *BaseTokenStream {
	return &BaseTokenStream{
		AttributeSource: util.NewAttributeSourceWithFactory(DefaultTokenAttributeFactory),
	}
}

// NewBaseTokenStreamFrom returns a BaseTokenStream that uses the same attributes
// as the supplied one.
func NewBaseTokenStreamFrom(input *util.AttributeSource) *BaseTokenStream {
	return &BaseTokenStream{
		AttributeSource: util.NewAttributeSourceFrom(input),
	}
}

// NewBaseTokenStreamWithFactory returns a BaseTokenStream using the supplied
// AttributeFactory for creating new Attribute instances.
func NewBaseTokenStreamWithFactory(factory util.AttributeFactory) *BaseTokenStream {
	return &BaseTokenStream{
		AttributeSource: util.NewAttributeSourceWithFactory(factory),
	}
}

func (b *BaseTokenStream) GetAttributeSource() *util.AttributeSource {
	return b.AttributeSource
}

// End is called by the consumer after the last token has been consumed.
// It ensures that any end-of-stream operations can be performed.
func (b *BaseTokenStream) End() error {
	b.EndAttributes()
	return nil
}

// Reset resets this stream to a clean state.
func (b *BaseTokenStream) Reset() error {
	return nil
}

// Close releases resources associated with this stream.
func (b *BaseTokenStream) Close() error {
	return nil
}

func (b *BaseTokenStream) IncrementToken() (bool, error) {
	return false, nil
}
