package scanner

import (
	"testing"
)

func TestExtractGoImports(t *testing.T) {
	e := NewImportExtractor()
	content := `package main

import "fmt"

import (
	"os"
	"github.com/foo/bar/baz"
)
`
	got := e.ExtractImports(content, "go")
	want := map[string]bool{"fmt": true, "os": true, "github.com/foo/bar/baz": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected import %q", p)
		}
	}
}

func TestExtractJSImports(t *testing.T) {
	e := NewImportExtractor()
	content := `
import React from 'react';
import { foo } from './foo';
export { bar } from '../bar';
const x = require('lodash');
const y = import('./lazy');
`
	got := e.ExtractImports(content, "ts")
	want := map[string]bool{"react": true, "./foo": true, "../bar": true, "lodash": true, "./lazy": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected import %q", p)
		}
	}
}

func TestExtractPythonImports(t *testing.T) {
	e := NewImportExtractor()
	content := `
import os
import sys, re
from foo.bar import baz
from .sibling import helper
from ..parent import util
`
	got := e.ExtractImports(content, "py")
	want := map[string]bool{
		"os": true, "sys": true, "re": true,
		"foo.bar": true,
		".sibling": true, "..parent": true,
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected import %q", p)
		}
	}
	for w := range want {
		found := false
		for _, p := range got {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing import %q", w)
		}
	}
}

func TestExtractCImports(t *testing.T) {
	e := NewImportExtractor()
	content := `
#include "foo/bar.h"
#include <stdio.h>
`
	got := e.ExtractImports(content, "cpp")
	want := map[string]bool{"foo/bar.h": true, "stdio.h": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected import %q", p)
		}
	}
}

func TestExtractRustImports(t *testing.T) {
	e := NewImportExtractor()
	content := `
use crate::foo::Bar;
use std::collections::HashMap;
mod utils;
`
	got := e.ExtractImports(content, "rust")
	want := map[string]bool{"crate::foo::Bar": true, "std::collections::HashMap": true, "utils": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected import %q", p)
		}
	}
}
