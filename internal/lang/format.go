package lang

import (
	"strings"

	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Format returns a single full-document TextEdit that canonically formats the
// document, or nil if the source cannot be parsed (so we never clobber a file
// the user is mid-edit) or already matches the formatter's output.
//
// Martian's parser returns the canonically formatted source as its first value,
// produced from the parsed AST; semantic (type) errors do not prevent it, but a
// hard syntax error leaves the AST nil, in which case we skip.
func (d *Document) Format() []protocol.TextEdit {
	formatted, _, ast, _ := syntax.ParseSourceBytes([]byte(d.Text), d.Path, d.MROPaths(), false)
	if ast == nil || formatted == "" || formatted == d.Text {
		return nil
	}

	return []protocol.TextEdit{{
		Range:   wholeDocRange(d.Text),
		NewText: formatted,
	}}
}

// wholeDocRange returns a range covering the entire text.
func wholeDocRange(text string) protocol.Range {
	lines := strings.Split(text, "\n")
	last := len(lines) - 1

	return protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: uint32(last), Character: uint32(len(lines[last]))},
	}
}
