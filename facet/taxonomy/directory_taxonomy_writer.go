// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/facet"
	"github.com/FlavioCFOliveira/Gocene/facet/taxonomy/writercache"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const indexEpochKey = "index.epoch"

type DirectoryTaxonomyWriter struct {
	dir            store.Directory
	indexWriter   *index.IndexWriter
	cache         writercache.TaxonomyWriterCache
	cacheMisses   atomic.Int32
	nextID        atomic.Int32
	indexEpoch    int64
	isClosed      atomic.Bool
	taxoArrays    atomic.Pointer[TaxonomyIndexArrays]

	mu            sync.Mutex
	readerManager *index.ReaderManager
	initializedRM bool
	shouldRefreshRM bool
	cacheIsComplete bool
	shouldFillCache bool
	cacheMissesUntilFill int
}

func NewDirectoryTaxonomyWriter(dir store.Directory, openMode index.OpenMode, cache writercache.TaxonomyWriterCache) (*DirectoryTaxonomyWriter, error) {
	config := index.NewIndexWriterConfig().SetOpenMode(openMode).SetMergePolicy(index.NewLogByteSizeMergePolicy())
	iw, err := index.OpenIndexWriter(dir, config)
	if err != nil {
		return nil, err
	}

	dtw := &DirectoryTaxonomyWriter{
		dir:            dir,
		indexWriter:    iw,
		cache:          cache,
		cacheMissesUntilFill: 11,
		shouldFillCache: true,
	}

	if cache == nil {
		dtw.cache = writercache.NewLruTaxonomyWriterCache(4000)
	}

	// Determine epoch
	if !index.IndexExists(dir) {
		dtw.indexEpoch = 1
	} else {
		infos := index.ReadLatestCommit(dir)
		epochStr := infos.UserData()[indexEpochKey]
		if epochStr == "" {
			dtw.indexEpoch = 1
		} else {
			// In Java it's Long.parseLong(epochStr, 16)
			var epoch int64
			fmt.Sscanf(epochStr, "%x", &epoch)
			dtw.indexEpoch = epoch
		}
	}

	if openMode == index.OpenModeCreate {
		dtw.indexEpoch++
	}

	dtw.nextID.Store(int32(iw.GetDocStats().MaxDoc()))

	if dtw.nextID.Load() == 0 {
		dtw.cacheIsComplete = true
		dtw.AddCategory(facet.NewFacetLabel())
	} else {
		dtw.cacheIsComplete = false
	}

	return dtw, nil
}

func (dtw *DirectoryTaxonomyWriter) AddCategory(categoryPath *facet.FacetLabel) (int, error) {
	if dtw.isClosed.Load() {
		return -1, fmt.Errorf("taxonomy writer closed")
	}

	res := dtw.cache.Get(categoryPath)
	if res >= 0 {
		return res, nil
	}

	dtw.mu.Lock()
	defer dtw.mu.Unlock()

	res = dtw.findCategory(categoryPath)
	if res < 0 {
		res = dtw.internalAddCategory(categoryPath)
	}

	return res, nil
}

func (dtw *DirectoryTaxonomyWriter) findCategory(categoryPath *facet.FacetLabel) int {
	res := dtw.cache.Get(categoryPath)
	if res >= 0 || dtw.cacheIsComplete {
		return res
	}

	dtw.cacheMisses.Add(1)
	dtw.perhapsFillCache()

	res = dtw.cache.Get(categoryPath)
	if res >= 0 || dtw.cacheIsComplete {
		return res
	}

	dtw.initReaderManager()
	doc := -1
	reader := dtw.readerManager.Acquire()
	defer dtw.readerManager.Release(reader)

	catTerm := facet.PathToString(categoryPath.Components, categoryPath.Length)
	for _, ctx := range reader.Leaves() {
		terms := ctx.Reader().Terms("$full_path$")
		it := terms.Iterator()
		if it.SeekExact(catTerm) {
			postings := it.Postings(0)
			doc = postings.NextDoc() + ctx.DocBase
			break
		}
	}

	if doc > 0 {
		dtw.addToCache(categoryPath, doc)
	}
	return doc
}

func (dtw *DirectoryTaxonomyWriter) internalAddCategory(cp *facet.FacetLabel) int {
	var parent int
	if cp.Length() > 1 {
		parentPath := cp.Subpath(cp.Length() - 1)
		parent = dtw.findCategory(parentPath)
		if parent < 0 {
			parent = dtw.internalAddCategory(parentPath)
		}
	} else if cp.Length() == 1 {
		parent = 0 // ROOT_ORDINAL
	} else {
		parent = -1 // INVALID_ORDINAL
	}
	return dtw.addCategoryDocument(cp, parent)
}

func (dtw *DirectoryTaxonomyWriter) addCategoryDocument(categoryPath *facet.FacetLabel, parent int) int {
	doc := index.NewDocument()
	doc.Add(index.NewNumericDocValuesField("$parent_ndv$", int64(parent)))

	fieldPath := facet.PathToString(categoryPath.Components, categoryPath.Length)
	doc.Add(index.NewBinaryDocValuesField("$full_path$", []byte(fieldPath)))
	doc.Add(index.NewStringField("$full_path$", fieldPath, index.FieldStoreNo))

	dtw.indexWriter.AddDocument(doc)
	id := int(dtw.nextID.Add(1)) - 1

	dtw.shouldRefreshRM = true

	arrays := dtw.getTaxoArrays()
	newArrays := arrays.Add(id, parent)
	dtw.taxoArrays.Store(newArrays)

	dtw.addToCache(categoryPath, id)
	return id
}

func (dtw *DirectoryTaxonomyWriter) addToCache(categoryPath *facet.FacetLabel, id int) {
	if dtw.cache.Put(categoryPath, id) {
		dtw.refreshReaderManager()
		dtw.cacheIsComplete = false
	}
}

func (dtw *DirectoryTaxonomyWriter) refreshReaderManager() {
	if dtw.shouldRefreshRM && dtw.initializedRM {
		dtw.readerManager.MaybeRefresh()
		dtw.shouldRefreshRM = false
	}
}

func (dtw *DirectoryTaxonomyWriter) perhapsFillCache() {
	if int(dtw.cacheMisses.Load()) < dtw.cacheMissesUntilFill {
		return
	}
	if !dtw.shouldFillCache {
		return
	}
	dtw.shouldFillCache = false

	dtw.initReaderManager()
	reader := dtw.readerManager.Acquire()
	defer dtw.readerManager.Release(reader)

	aborted := false
	for _, ctx := range reader.Leaves() {
		terms := ctx.Reader().Terms("$full_path$")
		it := terms.Iterator()
		for it.Next() != nil {
			if !dtw.cache.IsFull() {
				term := it.Term()
				cp := facet.NewFacetLabel(facet.StringToPath(term))
				postings := it.Postings(0)
				res := dtw.cache.Put(cp, postings.NextDoc()+ctx.DocBase)
				if res {
					// evicted
				}
			} else {
				aborted = true
				break
			}
		}
		if aborted {
			break
		}
	}

	dtw.cacheIsComplete = !aborted
	if dtw.cacheIsComplete {
		dtw.readerManager.Close()
		dtw.readerManager = nil
		dtw.initializedRM = false
	}
}

func (dtw *DirectoryTaxonomyWriter) initReaderManager() {
	if !dtw.initializedRM {
		dtw.readerManager = index.NewReaderManager(dtw.indexWriter, false, false)
		dtw.shouldRefreshRM = false
		dtw.initializedRM = true
	}
}

func (dtw *DirectoryTaxonomyWriter) PrepareCommit() (int64, error) {
	if dtw.isClosed.Load() {
		return -1, fmt.Errorf("taxonomy writer closed")
	}

	data := make(map[string]string)
	for k, v := range dtw.indexWriter.GetLiveCommitData() {
		data[k] = v
	}

	epochStr := data[indexEpochKey]
	var epoch int64
	fmt.Sscanf(epochStr, "%x", &epoch)
	if epochStr == "" || epoch != dtw.indexEpoch {
		data[indexEpochKey] = fmt.Sprintf("%x", dtw.indexEpoch)
		dtw.indexWriter.SetLiveCommitData(data)
	}

	return dtw.indexWriter.PrepareCommit()
}

func (dtw *DirectoryTaxonomyWriter) ReplaceTaxonomy(taxoDir store.Directory) error {
	dtw.mu.Lock()
	defer dtw.mu.Unlock()

	if err := dtw.indexWriter.DeleteAll(); err != nil {
		return err
	}
	if err := dtw.indexWriter.AddIndexes(taxoDir); err != nil {
		return err
	}

	dtw.shouldRefreshRM = true
	dtw.initReaderManager()
	dtw.refreshReaderManager()
	dtw.nextID.Store(int32(dtw.indexWriter.GetDocStats().MaxDoc()))
	dtw.taxoArrays = nil // will be re-computed

	dtw.cache.Clear()
	dtw.cacheIsComplete = false
	dtw.shouldFillCache = true
	dtw.cacheMisses.Store(0)

	dtw.indexEpoch++
	return nil
}

func (dtw *DirectoryTaxonomyWriter) DeleteAll() error {
	dtw.mu.Lock()
	defer dtw.mu.Unlock()

	if err := dtw.indexWriter.DeleteAll(); err != nil {
		return err
	}

	dtw.shouldRefreshRM = true
	dtw.initReaderManager()
	dtw.refreshReaderManager()
	dtw.nextID.Store(0)
	dtw.taxoArrays = nil

	dtw.cache.Clear()
	dtw.cacheIsComplete = false
	dtw.shouldFillCache = true
	dtw.cacheMisses.Store(0)

	dtw.indexEpoch++
	return nil
}

func (dtw *DirectoryTaxonomyWriter) AddTaxonomy(taxoDir store.Directory, mapObj OrdinalMap) error {
	if dtw.isClosed.Load() {
		return fmt.Errorf("taxonomy writer closed")
	}

	reader, err := index.OpenDirectoryReader(taxoDir)
	if err != nil {
		return err
	}
	defer reader.Close()

	size := reader.NumDocs()
	if err := mapObj.SetSize(size); err != nil {
		return err
	}

	base := 0
	for _, ctx := range reader.Leaves() {
		ar := ctx.Reader()
		terms := ar.Terms("$full_path$")
		it := terms.Iterator()
		for it.Next() != nil {
			cp := facet.NewFacetLabel(facet.StringToPath(it.Term()))
			ordinal, err := dtw.AddCategory(cp)
			if err != nil {
				return err
			}
			postings := it.Postings(0)
			if err := mapObj.AddMapping(postings.NextDoc()+base, ordinal); err != nil {
				return err
			}
		}
		base += ar.MaxDoc()
	}
	return mapObj.AddDone()
}
