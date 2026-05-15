// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package bkd

import "github.com/FlavioCFOliveira/Gocene/util"

// PointValue represents a dimensional point value written in the BKD
// tree. Port of org.apache.lucene.util.bkd.PointValue (Lucene 10.4.0).
//
// Implementations expose the packed point bytes, the associated doc
// id, and a combined byte view used by the BKD merge / sort paths.
type PointValue interface {
	// PackedValue returns the packed values for the dimensions.
	PackedValue() *util.BytesRef

	// DocID returns the docID associated with this point.
	DocID() int

	// PackedValueDocIDBytes returns the byte representation of the
	// packed value together with the docID.
	PackedValueDocIDBytes() *util.BytesRef
}
