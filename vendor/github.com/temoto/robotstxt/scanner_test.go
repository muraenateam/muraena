package robotstxt

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanner(t *testing.T) {
	t.Parallel()

	type tcase struct {
		input    string
		expect   []string
		errCount int
	}
	cases := []tcase{
		{"foo", []string{"foo"}, 0},
		{"\u2010", []string{"‐"}, 0},
		{"# comment \r\nSomething: Somewhere\r\n", []string{tokEOL, "Something", "Somewhere", tokEOL}, 0},
		{"# comment \r\n# more comments\n\nDisallow:\r", []string{tokEOL, tokEOL, "Disallow", tokEOL}, 0},
		{"\xef\xbb\xbfUser-agent: *\n", []string{"User-agent", "*", tokEOL}, 0},
		{"\xd9\xd9", []string{"\uFFFD\uFFFD"}, 2},
	}
	for i, c := range cases {
		tag := fmt.Sprintf("test-%d", i)
		t.Run(tag, func(t *testing.T) {
			sc := newByteScanner(tag, true)
			sc.feed([]byte(c.input), true)
			tokens := sc.scanAll()
			assert.Equal(t, c.errCount, sc.ErrorCount)
			assert.Equal(t, c.expect, tokens)
		})
	}
}
