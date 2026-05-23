// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

// singleFieldTestDb mirrors the Java helper class
// org.apache.lucene.queryparser.surround.query.SingleFieldTestDb.
//
// It is intended to create an in-memory Lucene index holding a single text
// field for use by the boolean/distance round-trip tests.  Index construction
// is deferred until Gocene's IndexWriter/DirectoryReader stack is complete
// enough to support a full search round-trip.
//
// Port of: queryparser/src/test/.../surround/query/SingleFieldTestDb.java
type singleFieldTestDb struct {
	docs      []string
	fieldName string
	// dir is intentionally omitted until the index stack is available.
}

// newSingleFieldTestDb creates a placeholder test database.  No actual index
// is built; callers that need search execution must skip via t.Skip until the
// index round-trip is implemented.
func newSingleFieldTestDb(docs []string, fieldName string) *singleFieldTestDb {
	return &singleFieldTestDb{docs: docs, fieldName: fieldName}
}

func (db *singleFieldTestDb) getDocs() []string    { return db.docs }
func (db *singleFieldTestDb) getFieldname() string { return db.fieldName }
