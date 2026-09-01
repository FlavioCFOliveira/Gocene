// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// JoinUtil provides utility methods for join operations.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.JoinUtil.
type JoinUtil struct{}

func NewJoinUtil() *JoinUtil {
	return &JoinUtil{}
}

// Simple mock implementation of JoinUtil methods.
func (ju *JoinUtil) GetDocs(docs []int) []int {
	return docs
}
