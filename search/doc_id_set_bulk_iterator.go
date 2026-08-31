// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
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

package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocIdSetBulkIterator is the Go port of org.apache.lucene.search.DocIdSetBulkIterator.
//
// Bulk iterator over a DocIdSetIterator.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/search/DocIdSetBulkIterator.java
type DocIdSetBulkIterator interface {
	// Iterate over documents contained in this iterator and call LeafCollector.Collect on
	// them.
	Iterate(collector LeafCollector, acceptDocs util.Bits, min, max int) error
}
