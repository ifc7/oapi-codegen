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
	"strings"
	"sync"
	"sync/atomic"

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
	total := len(fixed)
	for _, r := range results {
		total += len(r.text) + 2
	}
	var out bytes.Buffer
	out.Grow(total)
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
	// The printer writes in small pieces, which would grow the buffer by
	// doubling it; the output is a little longer than the input.
	out.Grow(len(src) + len(src)/4)
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
	if out, ok := fixHeaderImports(filename, header, refs); ok {
		return out, true
	}
	slowHeaderFixes.Add(1)
	header = withoutUnusedThirdPartyImports(withStandardImports(header, refs), refs)
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

// standardImports are the standard library packages the generated code can use
// that the import block does not list, by the name the code refers to them by.
// goimports adds a missing import itself, and to do that it reads an index of
// the module cache, which allocates hundreds of megabytes and takes tens of
// milliseconds. The names here are unambiguous (not "template" or "rand", which
// name two packages), and a package the code does not use is dropped again, so
// the import block comes out as it would have.
var standardImports = map[string]string{
	"base64":  "encoding/base64",
	"bufio":   "bufio",
	"cmp":     "cmp",
	"hex":     "encoding/hex",
	"iter":    "iter",
	"maps":    "maps",
	"math":    "math",
	"regexp":  "regexp",
	"slices":  "slices",
	"sort":    "sort",
	"sync":    "sync",
	"unicode": "unicode",
	"utf8":    "unicode/utf8",
}

// withStandardImports adds to the import block of header the standard library
// packages in standardImports that refs use and the block does not import.
func withStandardImports(header []byte, refs map[string]map[string]bool) []byte {
	closing := bytes.LastIndex(header, []byte("\n)"))
	if closing < 0 {
		return header
	}
	// The packages go in the first group of the block, the standard library
	// ones, which ends at the first empty line, or at the end of the block.
	at := closing + 1
	if open := bytes.Index(header, []byte("import (\n")); open >= 0 && open < closing {
		if blank := bytes.Index(header[open:closing], []byte("\n\n")); blank >= 0 {
			at = open + blank + 1
		}
	}
	var missing []string
	for name, path := range standardImports {
		if _, used := refs[name]; !used {
			continue
		}
		// Imported already, by this path or under this name.
		if bytes.Contains(header, []byte(`"`+path+`"`)) || bytes.Contains(header, []byte("\t"+name+" \"")) {
			continue
		}
		missing = append(missing, path)
	}
	if len(missing) == 0 {
		return header
	}
	sort.Strings(missing)
	out := make([]byte, 0, len(header)+len(missing)*24)
	out = append(out, header[:at]...)
	for _, path := range missing {
		out = append(out, "\t\""+path+"\"\n"...)
	}
	return append(out, header[at:]...)
}

// templateThirdPartyImports are the imports of the import block that are not
// from the standard library, by the name the code refers to them by: the ones
// the template lists, and the one the default type mapping adds.
var templateThirdPartyImports = []struct{ name, path string }{
	{"nullable", "github.com/oapi-codegen/nullable"},
	{"yaml", "go.yaml.in/yaml/v3"},
	{"uuid", "github.com/google/uuid"},
}

// withoutUnusedThirdPartyImports removes from the import block of header the
// third-party imports of the template that refs do not use. goimports removes
// them too, but to be sure that an unused import is unused, it reads an index
// of the module cache to find out what the package is really called, which
// allocates hundreds of megabytes. The names of these packages are known.
func withoutUnusedThirdPartyImports(header []byte, refs map[string]map[string]bool) []byte {
	for _, imp := range templateThirdPartyImports {
		if _, used := refs[imp.name]; used {
			continue
		}
		line := []byte("\t\"" + imp.path + "\"\n")
		if i := bytes.Index(header, line); i >= 0 {
			header = append(append([]byte{}, header[:i]...), header[i+len(line):]...)
		}
	}
	return header
}

// slowHeaderFixes counts the headers that fixHeaderImports could not handle
// and so were fixed by imports.Process. Tests require none for the fixtures.
var slowHeaderFixes atomic.Int64

// fixHeaderImports removes the imports of the header's import block that refs
// do not use, adds the standard library packages in standardImports that it
// does and the block lacks, and formats the result, which is what
// imports.Process does to the whole file, without its cost: every call of
// imports.Process reads an index of the module cache first, hundreds of
// megabytes of allocation, whatever the file. The formatting is still
// goimports': with FormatOnly it sorts and groups the imports, and does not
// fix them.
//
// It only answers when it is sure which imports are used: an import without a
// name whose package name it can't know, such as one outside the standard
// library that the template does not list, or a use of a package that is not
// imported and not in standardImports, is left to imports.Process, which looks
// them up.
func fixHeaderImports(filename string, header []byte, refs map[string]map[string]bool) ([]byte, bool) {
	const open = "import (\n"
	start := bytes.Index(header, []byte(open))
	closing := bytes.LastIndex(header, []byte("\n)"))
	if start < 0 || closing < start {
		return nil, false
	}
	block := string(header[start+len(open) : closing])

	type spec struct{ line, name, path string }
	var groups [][]spec
	groups = append(groups, nil)
	provided := make(map[string]bool)
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(groups[len(groups)-1]) > 0 {
				groups = append(groups, nil)
			}
			continue
		}
		alias, quoted, hasAlias := strings.Cut(trimmed, " ")
		if !hasAlias {
			alias, quoted = "", trimmed
		}
		if len(quoted) < 2 || quoted[0] != '"' || quoted[len(quoted)-1] != '"' || strings.ContainsAny(quoted[1:len(quoted)-1], "\" \t") {
			return nil, false // a comment, or a form this does not handle
		}
		path := quoted[1 : len(quoted)-1]
		name := alias
		if name == "" {
			var ok bool
			if name, ok = packageName(path); !ok {
				return nil, false
			}
		}
		keep := name == "_" || name == "." || refs[name] != nil
		if !keep {
			continue
		}
		provided[name] = true
		groups[len(groups)-1] = append(groups[len(groups)-1], spec{line: line, name: name, path: path})
	}

	var added []spec
	for pkg := range refs {
		if provided[pkg] {
			continue
		}
		path, ok := standardImports[pkg]
		if !ok {
			return nil, false
		}
		added = append(added, spec{line: "\t\"" + path + "\"", name: pkg, path: path})
	}
	sort.Slice(added, func(i, j int) bool { return added[i].path < added[j].path })
	if len(added) > 0 {
		// Into the first group, which is the standard library's.
		groups[0] = append(groups[0], added...)
	}

	var out bytes.Buffer
	out.Write(header[:start+len(open)])
	wrote := false
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		if wrote {
			out.WriteString("\n")
		}
		for _, sp := range group {
			out.WriteString(sp.line)
			out.WriteString("\n")
		}
		wrote = true
	}
	out.Write(header[closing+1:])

	formatted, err := imports.Process(filename, out.Bytes(), &imports.Options{Comments: true, TabIndent: true, TabWidth: 8, FormatOnly: true})
	if err != nil {
		return nil, false
	}
	return append(bytes.TrimRight(formatted, "\n"), '\n'), true
}

// packageName returns the name the package at path is imported under without
// an alias, for the packages whose name is known: those of the standard
// library, which are named by the last element of the path (not counting a
// version), and the third-party ones in templateThirdPartyImports.
func packageName(path string) (string, bool) {
	for _, imp := range templateThirdPartyImports {
		if imp.path == path {
			return imp.name, true
		}
	}
	elements := strings.Split(path, "/")
	if strings.Contains(elements[0], ".") {
		return "", false
	}
	name := elements[len(elements)-1]
	if len(elements) > 1 && len(name) > 1 && name[0] == 'v' && strings.Trim(name[1:], "0123456789") == "" {
		name = elements[len(elements)-2]
	}
	return name, true
}
