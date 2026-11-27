package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"runtime"
	"sort"
	"sync"

	"golang.org/x/tools/imports"
)

// Formatting generated code with imports.Process parses the whole file, fixes
// its imports, and prints it twice, on one core. For a large document that is
// about half of the time Generate takes. formatGoSource gives the same bytes
// by splitting the file at top-level declarations and formatting the pieces
// on all cores. Only the import block needs a view of the whole file, and it
// is a few dozen lines.
const (
	// parallelFormatMin is the size of source below which the single call is
	// used: there is nothing to gain from splitting a small file. Both are
	// variables so that tests can exercise the split on small documents.
	defaultParallelFormatMin = 512 << 10
	defaultFormatChunkSize   = 256 << 10
)

var (
	parallelFormatMin = defaultParallelFormatMin
	// formatChunkSize is about how much source one piece holds.
	formatChunkSize = defaultFormatChunkSize
)

// blankLine finds the first empty line, which may hold white space.
var blankLine = regexp.MustCompile(`\n[ \t\r]*\n`)

// formatGoSource formats and fixes the imports of src, as
// imports.Process(filename, src, nil) does, and returns the same bytes. It
// returns an error from that call for a source it can't split, so a source
// that doesn't parse reports the error it always has.
func formatGoSource(filename string, src []byte) ([]byte, error) {
	if len(src) < parallelFormatMin {
		return imports.Process(filename, src, nil)
	}
	if out, ok := formatInPieces(filename, src); ok {
		return out, nil
	}
	return imports.Process(filename, src, nil)
}

func formatInPieces(filename string, src []byte) ([]byte, bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil || len(file.Decls) == 0 {
		return nil, false
	}
	importDecl, ok := file.Decls[0].(*ast.GenDecl)
	if !ok || importDecl.Tok != token.IMPORT {
		return nil, false
	}
	// Every import is in the first declaration, or the fix below would miss
	// the others.
	for _, d := range file.Decls[1:] {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			return nil, false
		}
	}
	offset := func(p token.Pos) int { return fset.Position(p).Offset }

	header, ok := fixedHeader(filename, src[:offset(importDecl.End())], file)
	if !ok {
		return nil, false
	}

	pieces := splitDeclarations(src, offset(importDecl.End()), file.Decls[1:], offset)
	formatted := make([][]byte, len(pieces))
	failed := false
	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan int)
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range work {
				// format.Source keeps the indentation of a fragment's first
				// line, and the templates indent their output. It puts the
				// package clause on that line too, which would make a
				// comment there a comment on the clause, not the doc comment
				// it is above the declaration, so the fragment starts on a
				// line of its own.
				out, err := format.Source(append([]byte("\n"), bytes.TrimSpace(pieces[i])...))
				if err != nil {
					mu.Lock()
					failed = true
					mu.Unlock()
					continue
				}
				formatted[i] = bytes.TrimSpace(out)
			}
		})
	}
	for i := range pieces {
		work <- i
	}
	close(work)
	wg.Wait()
	if failed {
		return nil, false
	}

	var out bytes.Buffer
	out.Write(header)
	for _, piece := range formatted {
		if len(piece) == 0 {
			continue
		}
		out.WriteString("\n")
		out.Write(piece)
		out.WriteString("\n")
	}
	return out.Bytes(), true
}

// fixedHeader returns the formatted text up to the end of the import block,
// with the imports the file uses and no others, as imports.Process would
// leave it. The block is fixed against a stand-in for the rest of the file
// that makes the same package references, which is all the fix reads.
func fixedHeader(filename string, header []byte, file *ast.File) ([]byte, bool) {
	refs := make(map[string]map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// An identifier declared in the file is not a package. This is how
		// goimports tells them apart, so the imports come out the same.
		//nolint:staticcheck // ast.Object is deprecated, but it is what resolves the identifier here.
		if x, ok := sel.X.(*ast.Ident); ok && x.Obj == nil {
			if refs[x.Name] == nil {
				refs[x.Name] = make(map[string]bool)
			}
			refs[x.Name][sel.Sel.Name] = true
		}
		return true
	})

	var stub bytes.Buffer
	stub.Write(header)
	stub.WriteString("\n\n")
	pkgs := make([]string, 0, len(refs))
	for pkg := range refs {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		names := make([]string, 0, len(refs[pkg]))
		for name := range refs[pkg] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			_, _ = fmt.Fprintf(&stub, "var _ = %s.%s\n", pkg, name)
		}
	}
	fixed, err := imports.Process(filename, stub.Bytes(), nil)
	if err != nil {
		return nil, false
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, fixed, parser.ParseComments)
	if err != nil || len(f.Decls) == 0 {
		return nil, false
	}
	first, ok := f.Decls[0].(*ast.GenDecl)
	if !ok || first.Tok != token.IMPORT {
		return nil, false
	}
	end := fset.Position(first.End()).Offset
	return append(bytes.TrimRight(fixed[:end], "\n"), '\n'), true
}

// splitDeclarations cuts the source from offset from, just after the imports,
// to the end, which holds decls, into pieces of about
// formatChunkSize. A cut is made only at an empty line between two
// declarations, where gofmt leaves exactly one empty line however the pieces
// are formatted, so joining the formatted pieces with one empty line gives
// what formatting the whole would.
func splitDeclarations(src []byte, from int, decls []ast.Decl, offset func(token.Pos) int) [][]byte {
	pieces := make([][]byte, 0, len(src)/formatChunkSize+1)
	pieceStart := from
	for i := 1; i < len(decls); i++ {
		next := offset(decls[i].Pos())
		if doc := docOf(decls[i]); doc != nil {
			next = offset(doc.Pos())
		}
		if next-pieceStart < formatChunkSize {
			continue
		}
		gapStart := offset(decls[i-1].End())
		loc := blankLine.FindIndex(src[gapStart:next])
		if loc == nil {
			continue
		}
		cut := gapStart + loc[1]
		pieces = append(pieces, src[pieceStart:cut])
		pieceStart = cut
	}
	return append(pieces, src[pieceStart:])
}

func docOf(d ast.Decl) *ast.CommentGroup {
	switch d := d.(type) {
	case *ast.GenDecl:
		return d.Doc
	case *ast.FuncDecl:
		return d.Doc
	}
	return nil
}
