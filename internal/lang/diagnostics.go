package lang

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// FileDiagnostics groups diagnostics by the file they belong to. A single
// compile of one document can surface errors in @include'd files, so each entry
// is published to its own URI.
type FileDiagnostics struct {
	URI         string
	Diagnostics []protocol.Diagnostic
}

// severity returns a fresh pointer to the Error severity (the LSP Diagnostic
// field is a pointer).
func severity() *protocol.DiagnosticSeverity {
	s := protocol.DiagnosticSeverityError

	return &s
}

// matches the "<path>:<line>" tail that SourceLoc.writeTo emits, used to recover
// a location from error types whose fields are unexported (ParseError, lexer
// errors, FileNotFoundError, ...).
var locRe = regexp.MustCompile(`([^\s:]+\.mro):(\d+)`)

// Diagnose compiles the document and returns diagnostics grouped by file. The
// document's own URI is always present in the result (with an empty slice when
// clean) so the caller can clear stale squiggles.
func (d *Document) Diagnose() []FileDiagnostics {
	byPath := map[string][]protocol.Diagnostic{}
	// Ensure the current document always gets an entry so it is cleared.
	byPath[d.Path] = nil

	_, err := d.Compile()
	collect(err, d, byPath)

	out := make([]FileDiagnostics, 0, len(byPath))
	for path, diags := range byPath {
		uri := d.URI
		if path != d.Path {
			uri = PathToURI(path)
		}
		out = append(out, FileDiagnostics{URI: uri, Diagnostics: diags})
	}

	return out
}

// collect walks an error (flattening syntax.ErrorList) and appends a diagnostic
// for each leaf, indexed by the file it occurred in.
func collect(err error, d *Document, byPath map[string][]protocol.Diagnostic) {
	if err == nil {
		return
	}

	var list syntax.ErrorList
	var astErr *syntax.AstError
	switch {
	case errors.As(err, &list):
		for _, sub := range list {
			collect(sub, d, byPath)
		}
	case errors.As(err, &astErr):
		path, diag := diagFromNode(astErr.Node, astErr.Msg, d)
		byPath[path] = append(byPath[path], diag)
	default:
		path, diag := diagFromMessage(err.Error(), d)
		byPath[path] = append(byPath[path], diag)
	}
}

// diagFromNode builds a precise diagnostic from an AST node's SourceLoc.
func diagFromNode(node *syntax.AstNode, msg string, d *Document) (string, protocol.Diagnostic) {
	path := d.Path
	text := d.Text
	if node != nil && node.Loc.File != nil && node.Loc.File.FullPath != "" {
		path = node.Loc.File.FullPath
		if path != d.Path {
			text = "" // we don't have the included file's text; range falls back to line.
		}
	}
	line, col := 0, 0
	if node != nil {
		line = node.Loc.Line
		col = node.Loc.Col
	}

	return path, protocol.Diagnostic{
		Range:    rangeAt(text, line, col),
		Severity: severity(),
		Source:   new("martian"),
		Message:  strings.TrimSpace(msg),
	}
}

// diagFromMessage builds a best-effort diagnostic by parsing "<file>.mro:<line>"
// out of an error string.
func diagFromMessage(msg string, d *Document) (string, protocol.Diagnostic) {
	path, line := d.Path, 0
	text := d.Text
	if m := locRe.FindStringSubmatch(msg); m != nil {
		if n, err := strconv.Atoi(m[2]); err == nil {
			line = n
		}
		if m[1] != d.Path {
			path = m[1]
			text = ""
		}
	}

	return path, protocol.Diagnostic{
		Range:    rangeAt(text, line, 0),
		Severity: severity(),
		Source:   new("martian"),
		Message:  firstLine(strings.TrimSpace(stripMROPrefix(msg))),
	}
}

// rangeAt converts a 1-indexed Martian (line, col) into a 0-indexed LSP Range,
// synthesizing the end position. With a column it highlights the identifier
// token at that point; without one (col<=0) it highlights the whole line.
func rangeAt(text string, line, col int) protocol.Range {
	l := max(line-1, 0)
	startCh := max(col-1, 0)
	endCh := startCh + 1

	if text != "" {
		lines := strings.Split(text, "\n")
		if l < len(lines) {
			src := lines[l]
			if col <= 0 {
				// Whole-line highlight.
				return protocol.Range{
					Start: protocol.Position{Line: uint32(l), Character: 0},
					End:   protocol.Position{Line: uint32(l), Character: uint32(len(src))},
				}
			}
			endCh = identEnd(src, startCh)
		}
	}

	return protocol.Range{
		Start: protocol.Position{Line: uint32(l), Character: uint32(startCh)},
		End:   protocol.Position{Line: uint32(l), Character: uint32(endCh)},
	}
}

// identEnd returns the exclusive end column of the identifier-ish token starting
// at start within line, or start+1 if there is no such token.
func identEnd(line string, start int) int {
	if start >= len(line) {
		return start + 1
	}
	i := start
	for i < len(line) && isIdentByte(line[i]) {
		i++
	}
	if i == start {
		return start + 1
	}

	return i
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '.' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func stripMROPrefix(s string) string {
	return strings.TrimPrefix(s, "MRO ")
}

func firstLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return strings.TrimSpace(before)
	}

	return s
}
