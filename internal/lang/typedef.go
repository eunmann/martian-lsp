package lang

import (
	"strings"

	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// TypeDefinition jumps from a parameter (or a reference to one) at pos to the
// declaration of its type — i.e. the `filetype` or `struct` declaration. Builtin
// types have no declaration, so those return ok=false.
func (d *Document) TypeDefinition(pos protocol.Position) (protocol.Location, bool) {
	e, ok := d.Index().at(pos)
	if !ok {
		return protocol.Location{}, false
	}
	mode, callable, param, ok := splitParamSym(e.sym)
	if !ok {
		return protocol.Location{}, false
	}

	ast, _ := d.Compile()
	if ast == nil {
		return protocol.Location{}, false
	}
	base := paramBaseType(ast, mode, callable, param)
	if base == "" {
		return protocol.Location{}, false
	}

	return d.typeDeclLocation(ast, base)
}

// paramSymParts is the number of colon-separated fields in a parameter symbol
// ("mode:callable:param").
const paramSymParts = 3

// splitParamSym parses an index symbol of the form "in:<callable>:<param>" or
// "out:<callable>:<param>".
func splitParamSym(sym string) (string, string, string, bool) {
	parts := strings.SplitN(sym, ":", paramSymParts)
	if len(parts) != paramSymParts || (parts[0] != "in" && parts[0] != "out") {
		return "", "", "", false
	}

	return parts[0], parts[1], parts[2], true
}

// paramBaseType returns the base type name of a callable's in/out parameter.
func paramBaseType(ast *syntax.Ast, mode, callable, param string) string {
	c := lookupCallable(ast, callable)
	if c == nil {
		return ""
	}
	var ps *syntax.OutParams
	var ins *syntax.InParams
	if mode == "in" {
		ins = c.GetInParams()
	} else {
		ps = c.GetOutParams()
	}
	if ins != nil {
		for _, p := range ins.List {
			if p.Id == param {
				return p.Tname.Tname
			}
		}
	}
	if ps != nil {
		for _, p := range ps.List {
			if p.Id == param {
				return p.Tname.Tname
			}
		}
	}

	return ""
}

// typeDeclLocation finds the declaration of a user filetype or struct type.
func (d *Document) typeDeclLocation(ast *syntax.Ast, name string) (protocol.Location, bool) {
	for _, u := range ast.UserTypes {
		if u.Id == name {
			return d.locationOf(u.Node.Loc, name), true
		}
	}
	for _, s := range ast.StructTypes {
		if s.Id == name {
			return d.locationOf(s.Node.Loc, name), true
		}
	}

	return protocol.Location{}, false
}
