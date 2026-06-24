package lang

import (
	"strings"

	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Symbols returns the document-symbol (outline) tree for this document:
// stages and pipelines as top-level symbols, their params and calls as
// children. Symbols defined in @include'd files are excluded.
func (d *Document) Symbols() []protocol.DocumentSymbol {
	ast, _ := d.Compile()
	if ast == nil {
		return nil
	}

	var syms []protocol.DocumentSymbol

	for _, st := range ast.Stages {
		if !d.inThisFile(st.Node.Loc) {
			continue
		}
		detail := kindStage
		if st.Split {
			detail = kindStage + " (split)"
		}
		syms = append(syms, d.callableSymbol(st.Id, detail, protocol.SymbolKindClass,
			st.Node.Loc.Line, st.GetInParams(), st.GetOutParams(), nil))
	}

	for _, pl := range ast.Pipelines {
		if !d.inThisFile(pl.Node.Loc) {
			continue
		}
		syms = append(syms, d.callableSymbol(pl.Id, "pipeline", protocol.SymbolKindFunction,
			pl.Node.Loc.Line, pl.GetInParams(), pl.GetOutParams(), pl.Calls))
	}

	return syms
}

// callableSymbol builds a stage/pipeline symbol with param (and, for pipelines,
// call) children.
func (d *Document) callableSymbol(
	id, detail string, kind protocol.SymbolKind, line int,
	ins *syntax.InParams, outs *syntax.OutParams, calls []*syntax.CallStm,
) protocol.DocumentSymbol {
	var children []protocol.DocumentSymbol

	if ins != nil {
		for _, p := range ins.List {
			children = append(children, d.paramSymbol("in", p))
		}
	}
	if outs != nil {
		for _, p := range outs.List {
			children = append(children, d.paramSymbol("out", p))
		}
	}
	for _, c := range calls {
		children = append(children, d.callSymbol(c))
	}

	full, sel := d.nameRange(line, id)
	det := detail

	return protocol.DocumentSymbol{
		Name:           id,
		Detail:         &det,
		Kind:           kind,
		Range:          full,
		SelectionRange: sel,
		Children:       children,
	}
}

func (d *Document) paramSymbol(mode string, p syntax.StructMemberLike) protocol.DocumentSymbol {
	full, sel := d.nameRange(p.Line(), p.GetId())
	tn := p.GetTname()
	detail := mode + " " + tn.String()

	return protocol.DocumentSymbol{
		Name:           p.GetId(),
		Detail:         &detail,
		Kind:           protocol.SymbolKindField,
		Range:          full,
		SelectionRange: sel,
	}
}

func (d *Document) callSymbol(c *syntax.CallStm) protocol.DocumentSymbol {
	full, sel := d.nameRange(c.Node.Loc.Line, c.Id)
	detail := "call " + c.DecId

	return protocol.DocumentSymbol{
		Name:           c.Id,
		Detail:         &detail,
		Kind:           protocol.SymbolKindVariable,
		Range:          full,
		SelectionRange: sel,
	}
}

// inThisFile reports whether a location belongs to this document (vs. an
// included file).
func (d *Document) inThisFile(loc syntax.SourceLoc) bool {
	if loc.File == nil {
		return true
	}

	return loc.File.FullPath == "" || loc.File.FullPath == d.Path
}

// nameRange returns (whole-line range, name-token range) for a symbol named
// name declared on the given 1-indexed line. The selection range is found by
// locating name as a standalone token on the line, so it is robust to whether
// the AST's column points at a keyword or the identifier.
func (d *Document) nameRange(line int, name string) (protocol.Range, protocol.Range) {
	l := max(line-1, 0)
	lines := strings.Split(d.Text, "\n")
	src := ""
	if l < len(lines) {
		src = lines[l]
	}

	start := max(indexWord(src, name), 0)
	end := start + len(name)

	sel := protocol.Range{
		Start: protocol.Position{Line: uint32(l), Character: uint32(start)},
		End:   protocol.Position{Line: uint32(l), Character: uint32(end)},
	}
	full := protocol.Range{
		Start: protocol.Position{Line: uint32(l), Character: 0},
		End:   protocol.Position{Line: uint32(l), Character: uint32(len(src))},
	}

	return full, sel
}

// indexWord returns the byte index of word in s where it appears as a standalone
// identifier (not bordered by identifier characters), or -1.
func indexWord(s, word string) int {
	if word == "" {
		return -1
	}
	from := 0
	for {
		i := strings.Index(s[from:], word)
		if i < 0 {
			return -1
		}
		i += from
		leftOK := i == 0 || !isIdentByte(s[i-1])
		rightOK := i+len(word) >= len(s) || !isIdentByte(s[i+len(word)])
		if leftOK && rightOK {
			return i
		}
		from = i + 1
	}
}
