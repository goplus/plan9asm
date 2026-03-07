package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPackageSFilesAbsFiltersNonPlan9Asm(t *testing.T) {
	pkg := goListPackage{
		Dir: "/tmp/pkg",
		SFiles: []string{
			"foo.s",
			"bar.S",
			"baz.Sx",
			filepath.Join("/abs", "keep.s"),
		},
	}
	got := packageSFilesAbs(pkg)
	want := []string{
		filepath.Join("/tmp/pkg", "foo.s"),
		filepath.Join("/abs", "keep.s"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packageSFilesAbs() = %#v, want %#v", got, want)
	}
}
