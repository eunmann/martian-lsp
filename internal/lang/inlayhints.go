package lang

import (
	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// InlayHint is the LSP 3.17 inlay-hint wire type. glsp v0.2.2 only models LSP
// 3.16, so we declare it here and serialize it via the custom handler.
type InlayHint struct {
	Position     protocol.Position `json:"position"`
	Label        string            `json:"label"`
	Kind         int               `json:"kind,omitempty"` // 1=Type, 2=Parameter
	PaddingLeft  bool              `json:"paddingLeft,omitempty"`
	PaddingRight bool              `json:"paddingRight,omitempty"`
}

const inlayKindType = 1

// InlayHints returns type hints for call-argument bindings and pipeline return
// bindings within rng, e.g. `name: string = self.x`.
func (d *Document) InlayHints(snapshot *syntax.Ast, rng protocol.Range) []InlayHint {
	ast, _ := d.Compile()
	if ast == nil {
		ast = snapshot
	}
	if ast == nil {
		return nil
	}

	var hints []InlayHint
	for _, pl := range ast.Pipelines {
		if !d.inThisFile(pl.Node.Loc) {
			continue
		}
		for _, c := range pl.Calls {
			callee := lookupCallable(ast, c.DecId)
			if callee == nil || c.Bindings == nil {
				continue
			}
			for _, b := range c.Bindings.List {
				if t, ok := paramType(callee, "in", b.Id); ok {
					hints = d.appendTypeHint(hints, b.Node.Loc, b.Id, t, rng)
				}
			}
		}
		if pl.Ret != nil && pl.Ret.Bindings != nil {
			for _, b := range pl.Ret.Bindings.List {
				if t, ok := paramType(pl, "out", b.Id); ok {
					hints = d.appendTypeHint(hints, b.Node.Loc, b.Id, t, rng)
				}
			}
		}
	}

	return hints
}

func (d *Document) appendTypeHint(hints []InlayHint, loc syntax.SourceLoc, name string, t syntax.TypeId, rng protocol.Range) []InlayHint {
	if !d.inThisFile(loc) {
		return hints
	}
	r := d.tokenRange(loc.Line, name)
	if r.Start.Line < rng.Start.Line || r.Start.Line > rng.End.Line {
		return hints
	}
	tn := t

	return append(hints, InlayHint{
		Position: r.End, // right after the bound parameter name
		Label:    ": " + tn.String(),
		Kind:     inlayKindType,
	})
}

// paramType returns the type of a callable's in/out parameter by id.
func paramType(c syntax.Callable, mode, id string) (syntax.TypeId, bool) {
	if c == nil {
		return syntax.TypeId{}, false
	}
	if mode == "in" {
		if ps := c.GetInParams(); ps != nil {
			for _, p := range ps.List {
				if p.Id == id {
					return p.Tname, true
				}
			}
		}

		return syntax.TypeId{}, false
	}
	if ps := c.GetOutParams(); ps != nil {
		for _, p := range ps.List {
			if p.Id == id {
				return p.Tname, true
			}
		}
	}

	return syntax.TypeId{}, false
}
