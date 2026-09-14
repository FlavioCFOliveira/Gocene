// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermVectorsWriterHelper provides the default implementation for merging and
// adding doc vectors, mirroring the logic in Apache Lucene's TermVectorsWriter.
type TermVectorsWriterHelper struct{}

// AddProx is an expert API that allows the codec to consume positions and offsets
// directly from the indexer. Mirrors org.apache.lucene.codecs.TermVectorsWriter.addProx.
func (h *TermVectorsWriterHelper) AddProx(writer spi.TermVectorsWriter, numProx int, positions, offsets store.IndexInput) error {
	position := 0
	lastOffset := 0
	var payload *util.BytesRefBuilder

	for i := 0; i < numProx; i++ {
		var startOffset, endOffset int
		var thisPayload []byte

		if positions == nil {
			position = -1
			thisPayload = nil
		} else {
			code, err := positions.ReadVInt()
			if err != nil {
				return err
			}
			position += int(code >> 1)
			if (code & 1) != 0 {
				payloadLength, err := positions.ReadVInt()
				if err != nil {
					return err
				}

				if payload == nil {
					payload = util.NewBytesRefBuilder()
				}
				payload.GrowNoCopy(int(payloadLength))

				buf := make([]byte, payloadLength)
				if err := positions.ReadBytes(buf, 0, len(buf)); err != nil {
					return err
				}
				payload.SetLength(int(payloadLength))
				thisPayload = payload.Get().ValidBytes()
			} else {
				thisPayload = nil
			}
		}

		if offsets == nil {
			startOffset = -1
			endOffset = -1
		} else {
			v1, err := offsets.ReadVInt()
			if err != nil {
				return err
			}
			startOffset = lastOffset + int(v1)
			v2, err := offsets.ReadVInt()
			if err != nil {
				return err
			}
			endOffset = startOffset + int(v2)
			lastOffset = endOffset
		}

		if err := writer.AddPosition(position, startOffset, endOffset, thisPayload); err != nil {
			return err
		}
	}
	return nil
}

// Merge merges in the term vectors from the readers in mergeState.
// Mirrors org.apache.lucene.codecs.TermVectorsWriter.merge.
func (h *TermVectorsWriterHelper) Merge(writer spi.TermVectorsWriter, mergeState *index.MergeState) (int, error) {
	subs := make([]index.DocIDMergerSub, 0, len(mergeState.Readers))
	for i := 0; i < len(mergeState.Readers); i++ {
		reader := mergeState.TermVectorsReaders[i]
		if reader != nil {
			if err := reader.CheckIntegrity(); err != nil {
				return 0, err
			}
		}
		subs = append(subs, &termVectorsMergeSub{
			docMap: mergeState.DocMaps[i],
			reader: reader,
			maxDoc: mergeState.MaxDocs[i],
			docID:  -1,
		})
	}

	docIDMerger, err := index.NewDocIDMerger(subs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		return 0, err
	}

	docCount := 0
	for {
		sub, err := docIDMerger.Next()
		if err != nil {
			return 0, err
		}
		if sub == nil {
			break
		}

		tvSub := sub.(*termVectorsMergeSub)
		var vectors index.Fields
		if tvSub.reader == nil {
			vectors = nil
		} else {
			var err error
			vectors, err = tvSub.reader.Get(tvSub.docID)
			if err != nil {
				return 0, err
			}
		}

		if err := h.AddAllDocVectors(writer, vectors, mergeState); err != nil {
			return 0, err
		}
		docCount++
	}

	if err := writer.Finish(docCount); err != nil {
		return 0, err
	}

	return docCount, nil
}

func (h *TermVectorsWriterHelper) AddAllDocVectors(writer spi.TermVectorsWriter, vectors index.Fields, mergeState *index.MergeState) error {
	if vectors == nil {
		if err := writer.StartDocument(0); err != nil {
			return err
		}
		if err := writer.FinishDocument(); err != nil {
			return err
		}
		return nil
	}

	numFields := vectors.Size()
	if numFields == -1 {
		numFields = 0
		it, err := vectors.Iterator()
		if err != nil {
			return err
		}
		for {
			name, err := it.Next()
			if err != nil {
				return err
			}
			if name == "" {
				break
			}
			numFields++
		}
	}

	if err := writer.StartDocument(numFields); err != nil {
		return err
	}

	var lastFieldName string
	fieldCount := 0

	it, err := vectors.Iterator()
	if err != nil {
		return err
	}

	for {
		fieldName, err := it.Next()
		if err != nil {
			return err
		}
		if fieldName == "" {
			break
		}
		fieldCount++

		fieldInfo := mergeState.MergeFieldInfos.FieldInfo(fieldName)

		if lastFieldName != "" && fieldName <= lastFieldName {
			return fmt.Errorf("fields must be sorted: lastFieldName=%s fieldName=%s", lastFieldName, fieldName)
		}
		lastFieldName = fieldName

		terms, err := vectors.Terms(fieldName)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}

		hasPositions := terms.HasPositions()
		hasOffsets := terms.HasOffsets()
		hasPayloads := terms.HasPayloads()

		numTerms := int(terms.Size())
		if numTerms == -1 {
			numTerms = 0
			termsEnum, err := terms.Iterator()
			if err != nil {
				return err
			}
			for {
				term, err := termsEnum.Next()
				if err != nil {
					return err
				}
				if term == nil {
					break
				}
				numTerms++
			}
		}

		if err := writer.StartField(fieldInfo, numTerms, hasPositions, hasOffsets, hasPayloads); err != nil {
			return err
		}

		termsEnum, err := terms.Iterator()
		if err != nil {
			return err
		}

		termCount := 0
		for {
			term, err := termsEnum.Next()
			if err != nil {
				return err
			}
			if term == nil {
				break
			}
			termCount++

			// Java: final int freq = (int) termsEnum.totalTermFreq();
			ttf, err := termsEnum.TotalTermFreq()
			if err != nil {
				return err
			}
			freq := int(ttf)

			// Java: startTerm(termsEnum.term(), freq) — term() is a BytesRef.
			if err := writer.StartTerm(term.BytesValue().ValidBytes(), freq); err != nil {
				return err
			}

			if hasPositions || hasOffsets {
				// Java: termsEnum.postings(docsAndPositionsEnum, PostingsEnum.OFFSETS | PostingsEnum.PAYLOADS).
				// spi.TermsEnum.Postings takes the flags alone; Java's reuse
				// argument has no counterpart on this contract.
				docsAndPositionsEnum, err := termsEnum.Postings(index.PostingsFlagOffsets | index.PostingsFlagPayloads)
				if err != nil {
					return err
				}

				docID, err := docsAndPositionsEnum.NextDoc()
				if err != nil {
					return err
				}
				if docID == index.NO_MORE_DOCS {
					return fmt.Errorf("expected docID in postings enum")
				}

				postingsFreq, err := docsAndPositionsEnum.Freq()
				if err != nil {
					return err
				}
				if postingsFreq != freq {
					return fmt.Errorf("postings freq %d does not match term freq %d", postingsFreq, freq)
				}

				for posUpto := 0; posUpto < freq; posUpto++ {
					pos, err := docsAndPositionsEnum.NextPosition()
					if err != nil {
						return err
					}
					startOffset, err := docsAndPositionsEnum.StartOffset()
					if err != nil {
						return err
					}
					endOffset, err := docsAndPositionsEnum.EndOffset()
					if err != nil {
						return err
					}
					payload, err := docsAndPositionsEnum.GetPayload()
					if err != nil {
						return err
					}

					if err := writer.AddPosition(pos, startOffset, endOffset, payload); err != nil {
						return err
					}
				}
			}
			if err := writer.FinishTerm(); err != nil {
				return err
			}
		}

		if termCount != numTerms {
			return fmt.Errorf("term count %d does not match expected numTerms %d", termCount, numTerms)
		}
		if err := writer.FinishField(); err != nil {
			return err
		}
	}

	if fieldCount != numFields {
		return fmt.Errorf("field count %d does not match expected numFields %d", fieldCount, numFields)
	}

	if err := writer.FinishDocument(); err != nil {
		return err
	}

	return nil
}

type termVectorsMergeSub struct {
	docMap index.DocMap
	reader spi.TermVectorsReader
	maxDoc int
	docID  int
}

func (s *termVectorsMergeSub) MappedDocID() int {
	return s.docMap.Get(s.docID)
}

func (s *termVectorsMergeSub) NextDoc() (int, error) {
	s.docID++
	if s.docID >= s.maxDoc {
		return index.NO_MORE_DOCS, nil
	}
	return s.docID, nil
}

func (s *termVectorsMergeSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		mapped := s.docMap.Get(doc)
		if mapped != -1 {
			s.docID = doc
			return mapped, nil
		}
	}
}
