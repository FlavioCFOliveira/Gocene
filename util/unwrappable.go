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

package util

// Unwrappable is implemented by any wrapper that holds a single delegate of
// type T. Mirrors org.apache.lucene.util.Unwrappable<T>.
//
// The generic parameter makes the interface express "this wraps another T",
// which is what Lucene's filter chains rely on (e.g. an IndexReader filter
// unwrapping to the underlying reader). Implementations should be cheap: the
// Unwrap method is called speculatively by [UnwrapAll].
type Unwrappable[T any] interface {
	// Unwrap returns the wrapped instance.
	Unwrap() T
}

// UnwrapAll walks the Unwrappable chain starting at o and returns the inner
// non-wrapping instance. If o does not implement [Unwrappable[T]] it is
// returned unchanged. Mirrors Unwrappable.unwrapAll.
//
// Note: because Go generics cannot infer interface satisfaction across
// successive Unwrap calls when T is itself an interface, this function uses
// a runtime type assertion against Unwrappable[T] on each iteration. The
// loop terminates as soon as the chain stops implementing the interface.
func UnwrapAll[T any](o T) T {
	for {
		// We need to interpret o (a value of type T) as a possible
		// Unwrappable[T]. A direct type assertion on T is only legal when T
		// is an interface type; for concrete types the assertion will always
		// fail, which is exactly the contract we want.
		w, ok := any(o).(Unwrappable[T])
		if !ok {
			return o
		}
		o = w.Unwrap()
	}
}
