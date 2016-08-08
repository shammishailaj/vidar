// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package syntax_test

import (
	"testing"

	"github.com/a8m/expect"
	"github.com/nelsam/vidar/syntax"
)

func TestLayers_Unicode(t *testing.T) {
	expect := expect.New(t)

	ast := `
package foo

func µ() string {
	var þ = "Ωð"
	return þ
}
`
	s := syntax.New(syntax.DefaultTheme)
	err := s.Parse(ast)
	expect(err).To.Be.Nil()

	layers := s.Layers()
	keywords := layers[syntax.DefaultTheme.Colors.Keyword]
	expect(keywords.Spans()).To.Have.Len(4)

	// var
	start, _ := keywords.Spans()[2].Range()
	expectedStart := 33 // strings.Index counts by byte
	expect(start).To.Equal(expectedStart)

	// return
	start, _ = keywords.Spans()[3].Range()
	expectedStart = 47 // same as above
	expect(start).To.Equal(expectedStart)

	strings := layers[syntax.DefaultTheme.Colors.String]
	expect(strings.Spans()).To.Have.Len(1)
	start, end := strings.Spans()[0].Range()
	expectedStart = 41 // you guessed it
	expect(start).To.Equal(expectedStart)
	expect(end).To.Equal(expectedStart + 4)
}
