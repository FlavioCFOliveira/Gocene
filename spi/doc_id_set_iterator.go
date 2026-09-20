// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import "github.com/FlavioCFOliveira/Gocene/util"

// DocIdSetIterator is org.apache.lucene.search.DocIdSetIterator (Apache Lucene
// 10.5.0).
//
// There is exactly one declaration of this type in Gocene, in the util package,
// because util types that Lucene also expresses in terms of it — BitSet.or and
// BitSet.of — must refer to it, and util cannot import spi or search without an
// import cycle. This alias lets spi consumers spell it spi.DocIdSetIterator;
// search aliases it under the Java name in the same way.
type DocIdSetIterator = util.DocIdSetIterator
