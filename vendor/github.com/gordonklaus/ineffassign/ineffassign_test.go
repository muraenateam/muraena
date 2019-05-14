package main

import (
	"strings"
	"testing"
)

func Test(t *testing.T) {
	fset, comments, ineff := checkPath("testdata/testdata.go")
	expected := map[int]string{}
	for _, c := range comments {
		expected[fset.Position(c.Pos()).Line] = strings.TrimSpace(c.Text())
	}

	for _, id := range ineff {
		line := fset.Position(id.Pos()).Line
		if name, ok := expected[line]; !ok || name != id.Name {
			t.Error("unexpected:", line, id.Name)
		}
		delete(expected, line)
	}
	for line, name := range expected {
		t.Error("expected:", line, name)
	}
}
