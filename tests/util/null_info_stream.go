// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package util implements org.apache.lucene.tests.util.
package util

import "github.com/FlavioCFOliveira/Gocene/util"

// NullInfoStream prints nothing. Just to make sure tests pass w/ and without
// enabled InfoStream without actually making noise.
//
// This is the Go port of org.apache.lucene.tests.util.NullInfoStream.
type NullInfoStream struct{}

// Message discards the message.
func (NullInfoStream) Message(component, message string) {}

// IsEnabled reports true for every component: to actually enable logging,
// Lucene just ignores the message in Message.
func (NullInfoStream) IsEnabled(component string) bool { return true }

// Close releases no resources.
func (NullInfoStream) Close() error { return nil }

var _ util.InfoStream = NullInfoStream{}
