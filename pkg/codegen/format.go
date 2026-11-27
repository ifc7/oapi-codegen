package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
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
	// The import block ends at the first closing parenthesis on a line of its
	// own. The header is only a few dozen lines, so this is the one part that
	// is checked by a parse: the stand-in in fixedHeader is parsed, and a
	// header that is not what is expected there sends the source to the
	// single call.
	start := bytes.Index(src, []byte("\nimport (\n"))
	if start < 0 {
		return nil, false
	}
	end := bytes.Index(src[start:], []byte("\n)\n"))
	if end < 0 {
		return nil, false
	}
	end += start + len("\n)")
	header, body := src[:end], src[end:]

	cuts := cutPoints(body, formatChunkSize)
	results := make([]pieceResult, len(cuts)+1)
	work := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			for i := range work {
				from, to := 0, len(body)
				if i > 0 {
					from = cuts[i-1]
				}
				if i < len(cuts) {
					to = cuts[i]
				}
				results[i] = formatPiece(body[from:to])
			}
		})
	}
	for i := range results {
		work <- i
	}
	close(work)
	wg.Wait()

	refs := make(map[string]map[string]bool)
	declared := make(map[string]bool)
	for _, r := range results {
		if !r.ok {
			return nil, false
		}
		for name := range r.declared {
			declared[name] = true
		}
	}
	for _, r := range results {
		for pkg, names := range r.refs {
			if declared[pkg] {
				continue
			}
			if refs[pkg] == nil {
				refs[pkg] = make(map[string]bool)
			}
			for name := range names {
				refs[pkg][name] = true
			}
		}
	}

	fixed, ok := fixedHeader(filename, header, refs)
	if !ok {
		return nil, false
	}
	var out bytes.Buffer
	out.Write(fixed)
	for _, r := range results {
		if len(r.text) == 0 {
			continue
		}
		out.WriteString("\n")
		out.Write(r.text)
		out.WriteString("\n")
	}
	return out.Bytes(), true
}

// pieceResult is a formatted piece of the body, with what the rest of the
// file needs to know about it.
type pieceResult struct {
	ok   bool
	text []byte
	// refs are the package-qualified names it uses, by package, and declared
	// the names it declares at package level.
	refs     map[string]map[string]bool
	declared map[string]bool
}

// printerMode is how go/format prints, including its number normalization.
const printerMode = printer.UseSpaces | printer.TabIndent | 1<<30

// formatPiece parses and prints one run of declarations. The one parse gives
// the formatted text, the package references the imports are fixed against,
// and the names declared, which are not packages. A piece that doesn't parse
// as declarations, such as one cut inside a function, is not ok.
func formatPiece(piece []byte) pieceResult {
	const clause = "package p\n"
	src := make([]byte, 0, len(clause)+len(piece))
	src = append(append(src, clause...), bytes.TrimSpace(piece)...)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "piece.go", src, parser.ParseComments)
	if err != nil {
		return pieceResult{}
	}

	result := pieceResult{ok: true, refs: make(map[string]map[string]bool), declared: make(map[string]bool)}
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				result.declared[d.Name.Name] = true
			}
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				return pieceResult{}
			}
			for _, spec := range d.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					result.declared[spec.Name.Name] = true
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						result.declared[name.Name] = true
					}
				}
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// An identifier declared in the piece is not a package. This is how
		// goimports tells them apart, so the imports come out the same. The
		// names other pieces declare are removed when the pieces are joined.
		//nolint:staticcheck // ast.Object is deprecated, but it is what resolves the identifier here.
		if x, ok := sel.X.(*ast.Ident); ok && x.Obj == nil {
			if result.refs[x.Name] == nil {
				result.refs[x.Name] = make(map[string]bool)
			}
			result.refs[x.Name][sel.Sel.Name] = true
		}
		return true
	})

	var out bytes.Buffer
	cfg := printer.Config{Mode: printerMode, Tabwidth: 8}
	if err := cfg.Fprint(&out, fset, file); err != nil {
		return pieceResult{}
	}
	result.text = bytes.TrimSpace(bytes.TrimPrefix(out.Bytes(), []byte("package p")))
	return result
}

// cutPoints returns where to cut body into pieces of about size bytes. A cut
// is made at an empty line that the next declaration follows, as a function or
// a type with its doc comment, if any, which is where one declaration ends and the next begins: gofmt
// leaves exactly one empty line there however the pieces are formatted, so
// joining the formatted pieces with one empty line gives what formatting the
// whole would. The scan is a guess, not a parse: a cut inside a declaration
// leaves a piece that does not parse, and the caller falls back to formatting
// the whole.
func cutPoints(body []byte, size int) []int {
	var cuts []int
	pos := 0
	for pos+size < len(body) {
		from := pos + size
		cut := -1
		for cut < 0 {
			loc := blankLine.FindIndex(body[from:])
			if loc == nil {
				break
			}
			candidate := from + loc[1]
			if startsDeclaration(body[candidate:]) {
				cut = candidate
			}
			from = candidate
		}
		if cut < 0 {
			break
		}
		cuts = append(cuts, cut)
		pos = cut
	}
	return cuts
}

// startsDeclaration reports whether the code that follows begins a function or
// type declaration, after any comment lines that document it.
func startsDeclaration(rest []byte) bool {
	// A doc comment is a few lines, so a short look ahead is enough; a longer
	// one is just not a place to cut.
	if len(rest) > 2048 {
		rest = rest[:2048]
	}
	for {
		rest = bytes.TrimLeft(rest, " \t\r\n")
		if !bytes.HasPrefix(rest, []byte("//")) {
			break
		}
		next := bytes.IndexByte(rest, '\n')
		if next < 0 {
			return false
		}
		rest = rest[next:]
	}
	return bytes.HasPrefix(rest, []byte("func ")) || bytes.HasPrefix(rest, []byte("type "))
}

// fixedHeader returns the formatted text up to the end of the import block,
// with the imports the file uses and no others, as imports.Process would
// leave it. The block is fixed against a stand-in for the rest of the file
// that makes the same package references, which is all the fix reads.
func fixedHeader(filename string, header []byte, refs map[string]map[string]bool) ([]byte, bool) {
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
