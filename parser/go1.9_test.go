// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains test cases for type alias which is supported from go1.9.
// +build go1.9

package parser

import "testing"

var validTypeAliases = []string{
	`package p; type T = int`,
	`package p; type (T = p.T; _ = struct{}; x = *T)`,
}

func TestValidTypeAlias(t *testing.T) {
	for _, src := range validTypeAliases {
		checkErrors(t, src, src)
	}
}

func TestParseExprTypeAlias(t *testing.T) {
	// ParseExpr must not crash
	for _, src := range validTypeAliases {
		ParseExpr(src)
	}
}
