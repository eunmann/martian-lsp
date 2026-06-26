package lang

import (
	"strings"

	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// entry is one indexed, clickable span in a document: a definition, a call, or a
// reference. Each carries a symbol key so all occurrences of the same logical
// symbol can be found (for references and rename). Hover text and definition
// target are resolved eagerly at build time.
//
// Symbol key forms:
//
//	callable:<id>          a stage or pipeline
//	in:<callable>:<param>  an input parameter (decl, call-binding LHS, or self.X ref)
//	out:<callable>:<param> an output parameter (decl, return LHS, or CALL.X ref)
type entry struct {
	rng   protocol.Range
	sym   string
	hover string
	def   *protocol.Location
}

// Index maps document positions to resolved language entries.
type Index struct {
	entries []entry
}

// Callable kind strings, as returned by syntax.Callable.Type().
const (
	kindStage    = "stage"
	kindPipeline = "pipeline"
)

// multiLineWidth is the sentinel "width" for a multi-line range, so single-line
// (narrower) entries always win in at().
const multiLineWidth = 1 << 30

// at returns the narrowest entry whose range contains pos.
func (ix *Index) at(pos protocol.Position) (entry, bool) {
	best := entry{}
	found := false
	bestWidth := uint32(1<<31 - 1)
	for _, e := range ix.entries {
		if !contains(e.rng, pos) {
			continue
		}
		w := width(e.rng)
		if !found || w < bestWidth {
			best, bestWidth, found = e, w, true
		}
	}

	return best, found
}

func contains(r protocol.Range, p protocol.Position) bool {
	if p.Line < r.Start.Line || p.Line > r.End.Line {
		return false
	}
	if p.Line == r.Start.Line && p.Character < r.Start.Character {
		return false
	}
	if p.Line == r.End.Line && p.Character > r.End.Character {
		return false
	}

	return true
}

func width(r protocol.Range) uint32 {
	if r.Start.Line != r.End.Line {
		return multiLineWidth
	}

	return r.End.Character - r.Start.Character
}

// builder accumulates index entries for one document compile.
type builder struct {
	doc *Document
	ast *syntax.Ast
	out []entry
}

// Index compiles the document and builds its position index.
func (d *Document) Index() *Index {
	ast, _ := d.Compile()
	if ast == nil {
		return &Index{}
	}
	b := &builder{doc: d, ast: ast}
	b.build()

	return &Index{entries: b.out}
}

func (b *builder) build() {
	for _, st := range b.ast.Stages {
		if b.doc.inThisFile(st.Node.Loc) {
			b.addCallableDef(st)
		}
	}
	for _, pl := range b.ast.Pipelines {
		if b.doc.inThisFile(pl.Node.Loc) {
			b.addCallableDef(pl)
			b.addPipelineRefs(pl)
		}
	}
}

func (b *builder) add(e entry) { b.out = append(b.out, e) }

// addCallableDef indexes a stage/pipeline name and its parameter declarations.
func (b *builder) addCallableDef(c syntax.Callable) {
	id := c.GetId()
	loc := callableLoc(c)
	b.add(entry{
		rng:   b.doc.tokenRange(loc.Line, id),
		sym:   "callable:" + id,
		hover: sigHover(c),
		def:   new(b.location(loc, id)),
	})
	if ins := c.GetInParams(); ins != nil {
		for _, p := range ins.List {
			b.addParamDef("in", id, p.Node.Loc, p.Tname, p.Id, p.Help)
		}
	}
	if outs := c.GetOutParams(); outs != nil {
		for _, p := range outs.List {
			b.addParamDef("out", id, p.Node.Loc, p.Tname, p.Id, p.Help)
		}
	}
}

func (b *builder) addParamDef(mode, callableID string, loc syntax.SourceLoc, tname syntax.TypeId, id, help string) {
	if !b.doc.inThisFile(loc) {
		return
	}
	b.add(entry{
		rng:   b.doc.tokenRange(loc.Line, id),
		sym:   mode + ":" + callableID + ":" + id,
		hover: paramHover(mode, tname, id, help),
		def:   new(b.location(loc, id)),
	})
}

// addPipelineRefs indexes a pipeline's call sites, the binding LHS names, and the
// references appearing in call/return binding expressions.
func (b *builder) addPipelineRefs(pl *syntax.Pipeline) {
	callMap := map[string]string{} // call Id -> callable (DecId)
	for _, c := range pl.Calls {
		callMap[c.Id] = c.DecId
	}

	for _, c := range pl.Calls {
		callee := b.ast.Callables.Table[c.DecId]
		if callee != nil {
			b.add(entry{
				rng:   b.doc.tokenRange(c.Node.Loc.Line, c.DecId),
				sym:   "callable:" + c.DecId,
				hover: sigHover(callee),
				def:   new(b.location(callableLoc(callee), callee.GetId())),
			})
		}
		if c.Bindings != nil {
			for _, bind := range c.Bindings.List {
				b.addBindingLHS("in", c.DecId, callee, bind) // LHS = callee input param
				b.walkExp(pl, callMap, bind.Exp)
			}
		}
	}
	if pl.Ret != nil && pl.Ret.Bindings != nil {
		for _, bind := range pl.Ret.Bindings.List {
			b.addBindingLHS("out", pl.Id, pl, bind) // LHS = pipeline output param
			b.walkExp(pl, callMap, bind.Exp)
		}
	}
}

// addBindingLHS indexes the left-hand side of a binding (e.g. `name = ...`),
// which names a parameter of the owning callable.
func (b *builder) addBindingLHS(mode, ownerID string, owner syntax.Callable, bind *syntax.BindStm) {
	if bind == nil || !b.doc.inThisFile(bind.Node.Loc) {
		return
	}
	hover := ""
	var def *protocol.Location
	if p := lookupParam(owner, mode, bind.Id); p != nil {
		hover = paramHover(mode, p.GetTname(), p.GetId(), p.GetHelp())
		def = new(b.location(paramLoc(p), p.GetId()))
	}
	b.add(entry{
		rng:   b.doc.tokenRange(bind.Node.Loc.Line, bind.Id),
		sym:   mode + ":" + ownerID + ":" + bind.Id,
		hover: hover,
		def:   def,
	})
}

func (b *builder) walkExp(pl *syntax.Pipeline, callMap map[string]string, exp syntax.Exp) {
	if exp == nil {
		return
	}
	_ = syntax.WalkExp(exp, func(e syntax.Exp, _ string) error {
		if ref, ok := e.(*syntax.RefExp); ok && b.doc.inThisFile(ref.Node.Loc) {
			b.addRefParts(pl, callMap, ref)
		}

		return nil
	})
}

// addRefParts splits a reference like `HELLO.greeting` or `self.name` into its
// component tokens so each resolves and renames independently.
func (b *builder) addRefParts(pl *syntax.Pipeline, callMap map[string]string, r *syntax.RefExp) {
	line0 := max(r.Node.Loc.Line-1, 0)
	start := max(r.Node.Loc.Col-1, 0)

	switch r.Kind {
	case syntax.KindCall:
		decID := callMap[r.Id]
		if decID == "" {
			decID = r.Id
		}
		callee := b.ast.Callables.Table[decID]

		// "HELLO" -> the callable.
		callHover := ""
		var callDef *protocol.Location
		if callee != nil {
			callHover = sigHover(callee)
			callDef = new(b.location(callableLoc(callee), callee.GetId()))
		}
		b.add(entry{
			rng:   rangeOnLine(line0, start, len(r.Id)),
			sym:   "callable:" + decID,
			hover: callHover,
			def:   callDef,
		})

		// ".greeting" -> the output parameter.
		if r.OutputId != "" {
			out := firstSegment(r.OutputId)
			oHover, oDef := b.outputRef(callee, out)
			b.add(entry{
				rng:   rangeOnLine(line0, start+len(r.Id)+1, len(out)), // skip "HELLO."
				sym:   "out:" + decID + ":" + out,
				hover: oHover,
				def:   oDef,
			})
		}

	case syntax.KindSelf:
		// "self.name" -> the pipeline input param "name".
		nameStart := start + len(syntax.KindSelf) + 1 // skip "self."
		hover := ""
		var def *protocol.Location
		if pl.InParams != nil {
			if p, ok := pl.InParams.Table[r.Id]; ok {
				hover = paramHover("in", p.Tname, p.Id, p.Help)
				def = new(b.location(p.Node.Loc, p.Id))
			}
		}
		b.add(entry{
			rng:   rangeOnLine(line0, nameStart, len(r.Id)),
			sym:   "in:" + pl.Id + ":" + r.Id,
			hover: hover,
			def:   def,
		})

	default:
		// Other expression kinds (literals, arrays, maps) are not references.
	}
}

// outputRef resolves an output parameter of callee to (hover, definition).
func (b *builder) outputRef(callee syntax.Callable, name string) (string, *protocol.Location) {
	if callee == nil {
		return "", nil
	}
	outs := callee.GetOutParams()
	if outs == nil {
		return "", nil
	}
	p, ok := outs.Table[name]
	if !ok {
		return "", nil
	}

	return paramHover("out", p.Tname, p.Id, p.Help), new(b.location(p.Node.Loc, p.Id))
}

// location builds an LSP Location for a definition at loc, ranged over name.
func (b *builder) location(loc syntax.SourceLoc, name string) protocol.Location {
	uri := b.doc.URI
	if loc.File != nil && loc.File.FullPath != "" && loc.File.FullPath != b.doc.Path {
		uri = PathToURI(loc.File.FullPath)
	}

	return protocol.Location{
		URI:   uri,
		Range: b.doc.tokenRangeFor(loc, name),
	}
}

// ---- hover rendering ----

func sigHover(c syntax.Callable) string {
	var sb strings.Builder
	sb.WriteString("```mro\n")
	sb.WriteString(c.Type())
	sb.WriteByte(' ')
	sb.WriteString(c.GetId())
	sb.WriteByte('(')
	first := true
	writeParam := func(mode string, tn syntax.TypeId, id string) {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		t := tn
		sb.WriteString(mode + " " + t.String() + " " + id)
	}
	if ins := c.GetInParams(); ins != nil {
		for _, p := range ins.List {
			writeParam("in", p.Tname, p.Id)
		}
	}
	if outs := c.GetOutParams(); outs != nil {
		for _, p := range outs.List {
			writeParam("out", p.Tname, p.Id)
		}
	}
	sb.WriteString(")\n```")

	return sb.String()
}

func paramHover(mode string, tname syntax.TypeId, id, help string) string {
	tn := tname
	h := "```mro\n" + mode + " " + tn.String() + " " + id + "\n```"
	if help != "" {
		h += "\n\n" + help
	}

	return h
}

// ---- small helpers ----

func callableLoc(c syntax.Callable) syntax.SourceLoc {
	switch v := c.(type) {
	case *syntax.Stage:
		return v.Node.Loc
	case *syntax.Pipeline:
		return v.Node.Loc
	}

	return syntax.SourceLoc{}
}

func lookupParam(c syntax.Callable, mode, id string) syntax.StructMemberLike {
	if c == nil {
		return nil
	}
	if mode == "in" {
		if ins := c.GetInParams(); ins != nil {
			if p, ok := ins.Table[id]; ok {
				return p
			}
		}

		return nil
	}
	if outs := c.GetOutParams(); outs != nil {
		if p, ok := outs.Table[id]; ok {
			return p
		}
	}

	return nil
}

func paramLoc(p syntax.StructMemberLike) syntax.SourceLoc {
	switch v := p.(type) {
	case *syntax.InParam:
		return v.Node.Loc
	case *syntax.OutParam:
		return v.Node.Loc
	}

	return syntax.SourceLoc{}
}

func rangeOnLine(line0, char0, length int) protocol.Range {
	if char0 < 0 {
		char0 = 0
	}

	return protocol.Range{
		Start: protocol.Position{Line: uint32(line0), Character: uint32(char0)},
		End:   protocol.Position{Line: uint32(line0), Character: uint32(char0 + length)},
	}
}

func firstSegment(path string) string {
	if before, _, ok := strings.Cut(path, "."); ok {
		return before
	}

	return path
}
