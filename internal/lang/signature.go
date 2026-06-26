package lang

import (
	"strings"

	"github.com/eunmann/martian-lsp/internal/lang/complete"
	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// SignatureHelp shows the input-parameter signature of the call enclosing pos,
// with the active argument highlighted. Symbols come from the snapshot so it
// works mid-edit.
func (d *Document) SignatureHelp(snapshot *syntax.Ast, pos protocol.Position) *protocol.SignatureHelp {
	callee, active, ok := complete.CallSignature(d.Text, int(pos.Line), int(pos.Character))
	if !ok || callee == "" {
		return nil
	}
	c := lookupCallable(snapshot, callee)
	if c == nil || c.GetInParams() == nil {
		return nil
	}

	ins := c.GetInParams().List
	params := make([]protocol.ParameterInformation, 0, len(ins))
	parts := make([]string, 0, len(ins))
	for _, p := range ins {
		tn := p.Tname
		parts = append(parts, "in "+tn.String()+" "+p.Id)
		params = append(params, protocol.ParameterInformation{Label: p.Id})
	}

	label := callee + "(" + strings.Join(parts, ", ") + ")"
	activeIdx := uint32(0)
	if active >= 0 && active < len(ins) {
		activeIdx = uint32(active)
	}
	zero := uint32(0)

	return &protocol.SignatureHelp{
		Signatures: []protocol.SignatureInformation{{
			Label:      label,
			Parameters: params,
		}},
		ActiveSignature: &zero,
		ActiveParameter: &activeIdx,
	}
}

// lookupCallable resolves a callable by id from an AST, preferring the table but
// falling back to a list scan (the table may be unpopulated on a failed compile).
func lookupCallable(ast *syntax.Ast, id string) syntax.Callable {
	if ast == nil || ast.Callables == nil {
		return nil
	}
	if ast.Callables.Table != nil {
		if c, ok := ast.Callables.Table[id]; ok {
			return c
		}
	}
	for _, c := range ast.Callables.List {
		if c.GetId() == id {
			return c
		}
	}

	return nil
}
