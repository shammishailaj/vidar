// This is free and unencumbered software released into the public
// domain.  For more information, see <http://unlicense.org> or the
// accompanying UNLICENSE file.

package syntax

import (
	"go/ast"
	"go/parser"
	"go/token"
	"unicode/utf8"

	"github.com/nelsam/gxui"
)

// Syntax is a type that reads Go source code to provide information
// on it.
type Syntax struct {
	Theme Theme

	fileSet     *token.FileSet
	layers      map[Color]*gxui.CodeSyntaxLayer
	runeOffsets []int
}

// New constructs a new *Syntax value with theme as its Theme field.
func New(theme Theme) *Syntax {
	return &Syntax{Theme: theme}
}

// Parse parses the passed in Go source code, replacing s's stored
// context with that of the parsed source.  It returns any error
// encountered while parsing source, but will still store as much
// information as possible.
func (s *Syntax) Parse(source string) error {
	s.runeOffsets = make([]int, len(source))
	byteOffset := 0
	for runeIdx, r := range []rune(source) {
		byteIdx := runeIdx + byteOffset
		bytes := utf8.RuneLen(r)
		for i := byteIdx; i < byteIdx+bytes; i++ {
			s.runeOffsets[i] = -byteOffset
		}
		byteOffset += bytes - 1
	}

	s.fileSet = token.NewFileSet()
	s.layers = make(map[Color]*gxui.CodeSyntaxLayer)
	f, err := parser.ParseFile(s.fileSet, "", source, parser.ParseComments)

	// Parse everything we can before returning the error.
	if f.Package.IsValid() {
		s.add(s.Theme.Colors.Keyword, f.Package, len("package"))
	}
	for _, importSpec := range f.Imports {
		s.addNode(s.Theme.Colors.String, importSpec)
	}
	for _, comment := range f.Comments {
		s.addNode(s.Theme.Colors.Comment, comment)
	}
	for _, decl := range f.Decls {
		s.addDecl(decl)
	}
	for _, unresolved := range f.Unresolved {
		s.addUnresolved(unresolved)
	}
	return err
}

// Layers returns a gxui.CodeSyntaxLayer for each color used from
// s.Theme when s.Parse was called.  The corresponding
// gxui.CodeSyntaxLayer will have its foreground and background
// colors set, and all positions that should be highlighted that
// color will be stored.
func (s *Syntax) Layers() map[Color]*gxui.CodeSyntaxLayer {
	return s.layers
}

func (s *Syntax) add(color Color, pos token.Pos, byteLength int) {
	if byteLength == 0 {
		return
	}
	layer, ok := s.layers[color]
	if !ok {
		layer = &gxui.CodeSyntaxLayer{}
		layer.SetColor(color.Foreground)
		layer.SetBackgroundColor(color.Background)
		s.layers[color] = layer
	}
	bytePos := s.fileSet.Position(pos).Offset
	if bytePos >= len(s.runeOffsets) {
		return
	}
	idx := s.runePos(bytePos)
	end := s.runePos(bytePos + byteLength)
	layer.Add(idx, end-idx)
}

func (s *Syntax) runePos(bytePos int) int {
	if bytePos >= len(s.runeOffsets) {
		return -1
	}
	return bytePos + s.runeOffsets[bytePos]
}

func (s *Syntax) addNode(color Color, node ast.Node) {
	s.add(color, node.Pos(), int(node.End()-node.Pos()))
}
