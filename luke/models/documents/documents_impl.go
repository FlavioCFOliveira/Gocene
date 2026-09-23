package documents

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/luke/models"
	"github.com/FlavioCFOliveira/Gocene/luke/models/util"
	coreutil "github.com/FlavioCFOliveira/Gocene/util"
)

// DocumentsImpl is the default implementation of Documents.
type DocumentsImpl struct {
	reader    index.IndexReader
	tvAdapter *TermVectorsAdapter
	dvAdapter *DocValuesAdapter
	curField  string
	tenum     index.TermsEnum
	penum     index.PostingsEnum
	liveDocs  coreutil.Bits
}

func NewDocumentsImpl(reader index.IndexReader) (*DocumentsImpl, error) {
	liveDocs, err := util.GetLiveDocs(reader)
	if err != nil {
		return nil, err
	}
	return &DocumentsImpl{
		reader:    reader,
		tvAdapter: NewTermVectorsAdapter(reader),
		dvAdapter: NewDocValuesAdapter(reader),
		liveDocs:  liveDocs,
	}, nil
}

func (d *DocumentsImpl) GetMaxDoc() int {
	return d.reader.MaxDoc()
}

func (d *DocumentsImpl) IsLive(docid int) bool {
	if d.liveDocs == nil {
		return true
	}
	return d.liveDocs.Get(docid)
}

func (d *DocumentsImpl) GetDocumentFields(docid int) ([]*DocumentField, error) {
	if !d.IsLive(docid) {
		return []*DocumentField{}, nil
	}

	var res []*DocumentField
	storedFields, err := d.reader.StoredFields()
	if err != nil {
		return nil, models.NewLukeException(fmt.Sprintf("Fields information not available for doc %d.", docid), err)
	}
	// Java: reader.storedFields().document(docid). Gocene's StoredFields
	// mirrors Lucene's visitor-based primitive, so the Document is rebuilt
	// through DocumentStoredFieldVisitor exactly as Lucene's own
	// document(int) overload does.
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(docid, visitor); err != nil {
		return nil, models.NewLukeException(fmt.Sprintf("Fields information not available for doc %d.", docid), err)
	}
	doc := visitor.GetDocument()

	fieldInfos, err := util.GetFieldInfos(d.reader)
	if err != nil {
		return nil, models.NewLukeException(fmt.Sprintf("Fields information not available for doc %d.", docid), err)
	}
	for _, finfo := range fieldInfos.Infos() {
		fields := doc.GetFieldsByName(finfo.Name())
		if len(fields) == 0 {
			df, err := NewDocumentField(finfo, nil, d.reader, docid)
			if err != nil {
				return nil, err
			}
			res = append(res, df)
		} else {
			for _, field := range fields {
				df, err := NewDocumentField(finfo, field, d.reader, docid)
				if err != nil {
					return nil, err
				}
				res = append(res, df)
			}
		}
	}

	return res, nil
}

func (d *DocumentsImpl) GetCurrentField() string {
	return d.curField
}

func (d *DocumentsImpl) FirstTerm(field string) (*index.Term, error) {
	terms, err := util.GetTerms(d.reader, field)
	if err != nil {
		d.resetTermsIterator()
		return nil, models.NewLukeException(fmt.Sprintf("Terms not available for field: %s.", field), err)
	}

	if terms == nil {
		d.resetCurrentField()
		d.resetTermsIterator()
		return nil, nil
	}

	d.curField = field
	tenum, err := terms.Iterator()
	if err != nil {
		d.resetTermsIterator()
		return nil, err
	}
	d.tenum = tenum

	next, err := d.tenum.Next()
	if err != nil {
		return nil, err
	}
	if next == nil {
		d.resetTermsIterator()
		return nil, nil
	}

	d.resetPostingsIterator()
	return index.NewTermFromBytesRef(d.curField, d.tenum.Term().Bytes), nil
}

func (d *DocumentsImpl) NextTerm() (*index.Term, error) {
	if d.tenum == nil {
		return nil, nil
	}

	next, err := d.tenum.Next()
	if err != nil {
		return nil, err
	}
	if next == nil {
		d.resetTermsIterator()
		return nil, nil
	}

	d.resetPostingsIterator()
	return index.NewTermFromBytesRef(d.curField, d.tenum.Term().Bytes), nil
}

func (d *DocumentsImpl) SeekTerm(termText string) (*index.Term, error) {
	if d.curField == "" {
		return nil, nil
	}

	terms, err := util.GetTerms(d.reader, d.curField)
	if err != nil {
		d.resetTermsIterator()
		return nil, models.NewLukeException(fmt.Sprintf("Terms not available for field: %s.", d.curField), err)
	}

	tenum, err := terms.Iterator()
	if err != nil {
		d.resetTermsIterator()
		return nil, err
	}
	d.tenum = tenum

	// Java: if (tenum.seekCeil(new BytesRef(termText)) == SeekStatus.END).
	// Gocene's SeekCeil returns the term it landed on, or nil past the end.
	seeked, err := d.tenum.SeekCeil(index.NewTerm(d.curField, termText))
	if err != nil {
		return nil, err
	}
	if seeked == nil {
		d.resetTermsIterator()
		return nil, nil
	}

	d.resetPostingsIterator()
	return index.NewTermFromBytesRef(d.curField, d.tenum.Term().Bytes), nil
}

func (d *DocumentsImpl) FirstTermDoc() (*int, error) {
	if d.tenum == nil {
		return nil, nil
	}

	penum, err := d.tenum.Postings(index.PostingsFlagAll)
	if err != nil {
		d.resetPostingsIterator()
		return nil, models.NewLukeException(fmt.Sprintf("Term docs not available for field: %s.", d.curField), err)
	}
	d.penum = penum

	doc, err := d.penum.NextDoc()
	if err != nil {
		return nil, err
	}
	if doc == coreutil.NO_MORE_DOCS {
		d.resetPostingsIterator()
		return nil, nil
	}

	docID := d.penum.DocID()
	return &docID, nil
}

func (d *DocumentsImpl) NextTermDoc() (*int, error) {
	if d.penum == nil {
		return nil, nil
	}

	doc, err := d.penum.NextDoc()
	if err != nil {
		return nil, err
	}
	if doc == coreutil.NO_MORE_DOCS {
		d.resetPostingsIterator()
		return nil, nil
	}

	docID := d.penum.DocID()
	return &docID, nil
}

func (d *DocumentsImpl) GetTermPositions() ([]*TermPosting, error) {
	if d.penum == nil {
		return []*TermPosting{}, nil
	}

	var res []*TermPosting
	freq, err := d.penum.Freq()
	if err != nil {
		return nil, err
	}
	for i := 0; i < freq; i++ {
		pos, err := d.penum.NextPosition()
		if err != nil {
			return nil, err
		}
		if pos < 0 {
			continue
		}
		posting, err := NewTermPosting(pos, d.penum)
		if err != nil {
			return nil, err
		}
		res = append(res, posting)
	}

	return res, nil
}

func (d *DocumentsImpl) GetDocFreq() (*int, error) {
	if d.tenum == nil {
		return nil, nil
	}

	freq, err := d.tenum.DocFreq()
	if err != nil {
		return nil, err
	}
	return &freq, nil
}

func (d *DocumentsImpl) GetTermVectors(docid int, field string) ([]*TermVectorEntry, error) {
	entries, err := d.tvAdapter.GetTermVector(docid, field)
	if err != nil {
		return nil, models.NewLukeException(fmt.Sprintf("Term vector not available for doc: #%d and field: %s", docid, field), err)
	}
	return entries, nil
}

func (d *DocumentsImpl) GetDocValues(docid int, field string) (*DocValues, error) {
	dv, err := d.dvAdapter.GetDocValues(docid, field)
	if err != nil {
		return nil, models.NewLukeException(fmt.Sprintf("Doc values not available for doc: #%d and field: %s", docid, field), err)
	}
	return dv, nil
}

func (d *DocumentsImpl) resetCurrentField() {
	d.curField = ""
}

func (d *DocumentsImpl) resetTermsIterator() {
	d.tenum = nil
}

func (d *DocumentsImpl) resetPostingsIterator() {
	d.penum = nil
}
