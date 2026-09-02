// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	fieldFull                = "full"
	fieldParentOrdinalNDV    = "parent"
	indexEpochKey            = "index.epoch"
	defaultCacheSize         = 4000
	cacheMissesUntilFill     = 11
)

// TaxonomyWriter is used to write a taxonomy to an index.
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.DirectoryTaxonomyWriter.
type TaxonomyWriter struct {
	mu             sync.Mutex
	dir            store.Directory
	indexWriter    *index.IndexWriter
	cache          TaxonomyWriterCache
	cacheMisses    atomic.Int32
	nextID         atomic.Int32
	indexEpoch     int64
	cacheIsComplete bool
	shouldFillCache bool
	isClosed       bool

	readerManager *index.ReaderManager
	initializedRM bool
	shouldRefreshRM bool

	taxoArrays *TaxonomyIndexArrays
}

// NewTaxonomyWriter creates a new TaxonomyWriter.
func NewTaxonomyWriter(dir store.Directory, openMode index.OpenMode, cache TaxonomyWriterCache) (*TaxonomyWriter, error) {
	config := index.IndexWriterConfig{
		OpenMode:   openMode,
		MergePolicy: index.NewLogByteSizeMergePolicy(),
	}
	iw, err := index.OpenIndexWriter(dir, config)
	if err != nil {
		return nil, err
	}

	tw := &TaxonomyWriter{
		dir:            dir,
		indexWriter:    iw,
		shouldFillCache: true,
	}

	// Read index epoch
	if !index.IndexExists(dir) {
		tw.indexEpoch = 1
	} else {
		infos := index.ReadLatestCommit(dir)
		userData := infos.UserData()
		epochStr, ok := userData[indexEpochKey]
		if !ok {
			tw.indexEpoch = 1
		} else {
			fmt.Sscanf(epochStr, "%x", &tw.indexEpoch)
		}
	}

	if openMode == index.OpenModeCreate {
		tw.indexEpoch++
	}

	tw.nextID.Store(int32(iw.MaxDoc()))

	if cache == nil {
		cache = NewLruTaxonomyWriterCache(defaultCacheSize)
	}
	tw.cache = cache

	if tw.nextID.Load() == 0 {
		tw.cacheIsComplete = true
		_, err := tw.AddCategory(NewFacetLabel())
		if err != nil {
			return nil, err
		}
	} else {
		tw.cacheIsComplete = false
	}

	return tw, nil
}

func (tw *TaxonomyWriter) initReaderManager() error {
	if !tw.initializedRM {
		tw.mu.Lock()
		defer tw.mu.Unlock()
		if !tw.initializedRM {
			tw.readerManager = index.NewReaderManager(tw.indexWriter, false, false)
			tw.shouldRefreshRM = false
			tw.initializedRM = true
		}
	}
	return nil
}

func (tw *TaxonomyWriter) findCategory(cp *FacetLabel) (int, error) {
	res := tw.cache.Get(cp)
	if res >= 0 || tw.cacheIsComplete {
		return res, nil
	}

	tw.cacheMisses.Add(1)
	tw.perhapsFillCache()
	res = tw.cache.Get(cp)
	if res >= 0 || tw.cacheIsComplete {
		return res, nil
	}

	if err := tw.initReaderManager(); err != nil {
		return -1, err
	}

	reader := tw.readerManager.Acquire()
	defer tw.readerManager.Release(reader)

	catTerm := PathToString(cp.components, cp.length)
	for _, leaf := range reader.Leaves() {
		terms := leaf.Terms(fieldFull)
		if terms.SeekExact(catTerm) {
			docs := terms.Postings(0)
			return docs.NextDoc() + leaf.DocBase, nil
		}
	}

	return -1, nil
}

func (tw *TaxonomyWriter) AddCategory(cp *FacetLabel) (int, error) {
	tw.ensureOpen()

	res := tw.cache.Get(cp)
	if res < 0 {
		tw.mu.Lock()
		defer tw.mu.Unlock()
		res, err := tw.findCategory(cp)
		if err != nil {
			return -1, err
		}
		if res < 0 {
			res = tw.internalAddCategory(cp)
		}
	}
	return res, nil
}

func (tw *TaxonomyWriter) internalAddCategory(cp *FacetLabel) int {
	var parent int
	if cp.length > 1 {
		parentPath := cp.Subpath(cp.length - 1)
		res, err := tw.findCategory(parentPath)
		if err != nil || res < 0 {
			parent = tw.internalAddCategory(parentPath)
		} else {
			parent = res
		}
	} else if cp.length == 1 {
		parent = RootOrdinal
	} else {
		parent = InvalidOrdinal
	}
	return tw.addCategoryDocument(cp, parent)
}

func (tw *TaxonomyWriter) addCategoryDocument(cp *FacetLabel, parent int) int {
	doc := document.NewDocument()
	doc.Add(document.NewNumericDocValuesField(fieldParentOrdinalNDV, int64(parent)))

	fieldPath := PathToString(cp.components, cp.length)
	doc.Add(document.NewBinaryDocValuesField(fieldFull, []byte(fieldPath)))
	doc.Add(document.NewStringField(fieldFull, fieldPath, index.DocValuesOnly))

	tw.indexWriter.AddDocument(doc)
	id := int(tw.nextID.Add(1)) - 1

	tw.shouldRefreshRM = true

	if tw.taxoArrays == nil {
		reader := tw.readerManager.Acquire()
		tw.taxoArrays, _ = newTaxonomyIndexArrays(reader)
		tw.readerManager.Release(reader)
	}
	tw.taxoArrays = tw.taxoArrays.Add(id, parent)

	tw.addToCache(cp, id)
	return id
}

func (tw *TaxonomyWriter) addToCache(cp *FacetLabel, id int) {
	if tw.cache.Put(cp, id) {
		tw.refreshReaderManager()
		tw.cacheIsComplete = false
	}
}

func (tw *TaxonomyWriter) refreshReaderManager() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.shouldRefreshRM && tw.initializedRM {
		tw.readerManager.MaybeRefresh()
		tw.shouldRefreshRM = false
	}
}

func (tw *TaxonomyWriter) perhapsFillCache() {
	if tw.cacheMisses.Load() < cacheMissesUntilFill {
		return
	}

	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.shouldFillCache {
		return
	}
	tw.shouldFillCache = false

	if err := tw.initReaderManager(); err != nil {
		return
	}

	reader := tw.readerManager.Acquire()
	defer tw.readerManager.Release(reader)

	aborted := false
	for _, leaf := range reader.Leaves() {
		terms := leaf.Terms(fieldFull)
		it := terms.Iterator()
		for it.Next() {
			if !tw.cache.IsFull() {
				term := it.Term()
				cp := NewFacetLabel(StringToPath(string(term)))
				docs := it.Postings(0)
				tw.cache.Put(cp, docs.NextDoc()+leaf.DocBase)
			} else {
				aborted = true
				break
			}
		}
		if aborted {
			break
		}
	}

	tw.cacheIsComplete = !aborted
	if tw.cacheIsComplete {
		tw.readerManager.Close()
		tw.readerManager = nil
		tw.initializedRM = false
	}
}

func (tw *TaxonomyWriter) GetParent(ordinal int) (int, error) {
	tw.ensureOpen()
	if ordinal < 0 || ordinal >= int(tw.nextID.Load()) {
		return -1, fmt.Errorf("ordinal out of bounds")
	}

	if tw.taxoArrays == nil {
		reader := tw.readerManager.Acquire()
		tw.taxoArrays, _ = newTaxonomyIndexArrays(reader)
		tw.readerManager.Release(reader)
	}
	return tw.taxoArrays.Parents().Get(ordinal), nil
}

func (tw *TaxonomyWriter) GetSize() int {
	tw.ensureOpen()
	return int(tw.nextID.Load())
}

func (tw *TaxonomyWriter) Commit() error {
	tw.ensureOpen()
	data := make(map[string]string)
	for k, v := range tw.indexWriter.GetLiveCommitData() {
		data[k] = v
	}

	epochStr, ok := data[indexEpochKey]
	var currentEpoch int64
	if ok {
		fmt.Sscanf(epochStr, "%x", &currentEpoch)
	}

	if epochStr == "" || currentEpoch != tw.indexEpoch {
		data[indexEpochKey] = fmt.Sprintf("%x", tw.indexEpoch)
		tw.indexWriter.SetLiveCommitData(data)
	}
	return tw.indexWriter.Commit()
}

func (tw *TaxonomyWriter) Close() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.isClosed {
		tw.Commit()
		tw.indexWriter.Close()
		tw.isClosed = true
		if tw.initializedRM {
			tw.readerManager.Close()
		}
		tw.cache.Close()
	}
	return nil
}

func (tw *TaxonomyWriter) ensureOpen() {
	if tw.isClosed {
		panic("taxonomy writer is closed")
	}
}
