// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed
// with this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// liveDocsPostingsEnum wraps a PostingsEnum to skip documents that are not live.
type liveDocsPostingsEnum struct {
	delegate schema.PostingsEnum
	liveDocs util.Bits
}

func (e *liveDocsPostingsEnum) NextDoc() (int, error) {
	for {
		doc, err := e.delegate.NextDoc()
		if err != nil || doc == schema.NO_MORE_DOCS {
			return doc, err
		}
		if e.liveDocs == nil || e.liveDocs.Get(doc) {
			return doc, nil
		}
	}
}

func (e *liveDocsPostingsEnum) Advance(target int) (int, error) {
	for {
		doc, err := e.delegate.Advance(target)
		if err != nil || doc == schema.NO_MORE_DOCS {
			return doc, err
		}
		if e.liveDocs == nil || e.liveDocs.Get(doc) {
			return doc, nil
		}
		target = doc + 1
	}
}

func (e *liveDocsPostingsEnum) DocID() int {
	return e.delegate.DocID()
}

func (e *liveDocsPostingsEnum) Freq() (int, error) {
	return e.delegate.Freq()
}

func (e *liveDocsPostingsEnum) NextPosition() (int, error) {
	return e.delegate.NextPosition()
}

func (e *liveDocsPostingsEnum) StartOffset() (int, error) {
	return e.delegate.StartOffset()
}

func (e *liveDocsPostingsEnum) EndOffset() (int, error) {
	return e.delegate.EndOffset()
}

func (e *liveDocsPostingsEnum) GetPayload() ([]byte, error) {
	return e.delegate.GetPayload()
}

func (e *liveDocsPostingsEnum) Cost() int64 {
	return e.delegate.Cost()
}

// liveDocsTermsEnum wraps a TermsEnum to ensure its Postings are filtered by liveDocs.
type liveDocsTermsEnum struct {
	delegate schema.TermsEnum
	liveDocs util.Bits
}

func (e *liveDocsTermsEnum) Next() (*schema.Term, error) {
	return e.delegate.Next()
}

func (e *liveDocsTermsEnum) SeekCeil(term *schema.Term) (*schema.Term, error) {
	return e.delegate.SeekCeil(term)
}

func (e *liveDocsTermsEnum) SeekExact(term *schema.Term) (bool, error) {
	return e.delegate.SeekExact(term)
}

func (e *liveDocsTermsEnum) Term() *schema.Term {
	return e.delegate.Term()
}

func (e *liveDocsTermsEnum) DocFreq() (int, error) {
	return e.delegate.DocFreq()
}

func (e *liveDocsTermsEnum) TotalTermFreq() (int64, error) {
	return e.delegate.TotalTermFreq()
}

func (e *liveDocsTermsEnum) Postings(flags int) (schema.PostingsEnum, error) {
	pe, err := e.delegate.Postings(flags)
	if err != nil || pe == nil {
		return pe, err
	}
	return &liveDocsPostingsEnum{
		delegate: pe,
		liveDocs: e.liveDocs,
	}, nil
}

func (e *liveDocsTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (schema.PostingsEnum, error) {
	if liveDocs != e.liveDocs {
		pe, err := e.delegate.PostingsWithLiveDocs(liveDocs, flags)
		if err != nil || pe == nil {
			return pe, err
		}
		return &liveDocsPostingsEnum{
			delegate: pe,
			liveDocs: liveDocs,
		}, nil
	}
	return e.Postings(flags)
}

// liveDocsTerms wraps a Terms object to ensure all its iterators and postings are filtered.
type liveDocsTerms struct {
	delegate schema.Terms
	liveDocs util.Bits
}

func (t *liveDocsTerms) GetIterator() (schema.TermsEnum, error) {
	te, err := t.delegate.GetIterator()
	if err != nil || te == nil {
		return te, err
	}
	return &liveDocsTermsEnum{
		delegate: te,
		liveDocs: t.liveDocs,
	}, nil
}

func (t *liveDocsTerms) GetIteratorWithSeek(seekTerm *schema.Term) (schema.TermsEnum, error) {
	te, err := t.delegate.GetIteratorWithSeek(seekTerm)
	if err != nil || te == nil {
		return te, err
	}
	return &liveDocsTermsEnum{
		delegate: te,
		liveDocs: t.liveDocs,
	}, nil
}

func (t *liveDocsTerms) GetPostingsReader(termText string, flags int) (schema.PostingsEnum, error) {
	pe, err := t.delegate.GetPostingsReader(termText, flags)
	if err != nil || pe == nil {
		return pe, err
	}
	return &liveDocsPostingsEnum{
		delegate: pe,
		liveDocs: t.liveDocs,
	}, nil
}

func (t *liveDocsTerms) Size() int64 {
	return t.delegate.Size()
}

func (t *liveDocsTerms) GetDocCount() (int, error) {
	return t.delegate.GetDocCount()
}

func (t *liveDocsTerms) GetSumDocFreq() (int64, error) {
	return t.delegate.GetSumDocFreq()
}

func (t *liveDocsTerms) GetSumTotalTermFreq() (int64, error) {
	return t.delegate.GetSumTotalTermFreq()
}

func (t *liveDocsTerms) HasFreqs() bool {
	return t.delegate.HasFreqs()
}

func (t *liveDocsTerms) HasOffsets() bool {
	return t.delegate.HasOffsets()
}

func (t *liveDocsTerms) HasPositions() bool {
	return t.delegate.HasPositions()
}

func (t *liveDocsTerms) HasPayloads() bool {
	return t.delegate.HasPayloads()
}

func (t *liveDocsTerms) GetMin() (*schema.Term, error) {
	return t.delegate.GetMin()
}

func (t *liveDocsTerms) GetMax() (*schema.Term, error) {
	return t.delegate.GetMax()
}

// WrapTerms creates a Terms object that filters postings by the given liveDocs.
func WrapTerms(t schema.Terms, liveDocs util.Bits) schema.Terms {
	if liveDocs == nil {
		return t
	}
	return &liveDocsTerms{
		delegate: t,
		liveDocs: liveDocs,
	}
}
