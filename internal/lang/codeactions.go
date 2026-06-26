package lang

import (
	"path/filepath"
	"strings"

	"github.com/eunmann/martian-lsp/internal/lang/complete"
	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// CodeActions offers quick-fixes for the call at the cursor:
//   - undefined callee: "Add @include" (if defined elsewhere in the workspace),
//     plus "Create stage"/"Create pipeline" scaffolds;
//   - known callee with unbound inputs: "Add missing arguments".
func (d *Document) CodeActions(ws *Workspace, rng protocol.Range) []protocol.CodeAction {
	ast, _ := d.Compile()
	if ast == nil {
		return nil
	}
	callee, bound, ok := complete.CallContext(d.Text, int(rng.Start.Line), int(rng.Start.Character))
	if !ok || callee == "" {
		return nil
	}

	c := lookupCallable(ast, callee)
	if c == nil {
		return d.undefinedCalleeActions(ws, callee)
	}
	if missing := missingInputs(c, bound); len(missing) > 0 {
		return []protocol.CodeAction{d.addMissingArgsAction(missing, rng)}
	}

	return nil
}

func (d *Document) undefinedCalleeActions(ws *Workspace, callee string) []protocol.CodeAction {
	var actions []protocol.CodeAction
	if ws != nil {
		if path, found := ws.DefiningFile(callee); found {
			actions = append(actions, d.addIncludeAction(callee, path))
		}
	}

	return append(actions,
		d.createCallableAction(kindStage, callee),
		d.createCallableAction(kindPipeline, callee),
	)
}

// missingInputs returns the callee's input parameters not present in bound.
func missingInputs(c syntax.Callable, bound []string) []string {
	ins := c.GetInParams()
	if ins == nil {
		return nil
	}
	have := make(map[string]bool, len(bound))
	for _, b := range bound {
		have[b] = true
	}
	var miss []string
	for _, p := range ins.List {
		if !have[p.Id] {
			miss = append(miss, p.Id)
		}
	}

	return miss
}

// addMissingArgsAction inserts `param = null,` lines for each missing input at
// the cursor line (null is a valid placeholder the user replaces).
func (d *Document) addMissingArgsAction(missing []string, rng protocol.Range) protocol.CodeAction {
	indent := ""
	lines := strings.Split(d.Text, "\n")
	if int(rng.Start.Line) < len(lines) {
		indent = leadingWhitespace(lines[rng.Start.Line])
	}
	var b strings.Builder
	for _, p := range missing {
		b.WriteString(indent)
		b.WriteString(p)
		b.WriteString(" = null,\n")
	}
	at := protocol.Position{Line: rng.Start.Line, Character: 0}
	qf := protocol.CodeActionKindQuickFix

	return protocol.CodeAction{
		Title: "Add missing arguments",
		Kind:  &qf,
		Edit:  d.singleEdit(at, at, b.String()),
	}
}

// addIncludeAction inserts an @include for a callable defined in another file.
func (d *Document) addIncludeAction(callee, defPath string) protocol.CodeAction {
	rel := defPath
	if d.Path != "" {
		if r, err := filepath.Rel(filepath.Dir(d.Path), defPath); err == nil {
			rel = r
		}
	}
	qf := protocol.CodeActionKindQuickFix
	top := protocol.Position{Line: 0, Character: 0}

	return protocol.CodeAction{
		Title: "Add @include \"" + rel + "\" for " + callee,
		Kind:  &qf,
		Edit:  d.singleEdit(top, top, "@include \""+rel+"\"\n"),
	}
}

// createCallableAction builds a quick-fix that appends a stage/pipeline skeleton.
func (d *Document) createCallableAction(kind, name string) protocol.CodeAction {
	var skeleton string
	if kind == kindStage {
		skeleton = "\nstage " + name + "(\n    src comp \"" + name + "\",\n)\n"
	} else {
		skeleton = "\npipeline " + name + "(\n)\n{\n}\n"
	}
	end := d.endPosition()
	qf := protocol.CodeActionKindQuickFix

	return protocol.CodeAction{
		Title: "Create " + kind + " " + name,
		Kind:  &qf,
		Edit:  d.singleEdit(end, end, skeleton),
	}
}

func (d *Document) singleEdit(start, end protocol.Position, text string) *protocol.WorkspaceEdit {
	return &protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			d.URI: {{Range: protocol.Range{Start: start, End: end}, NewText: text}},
		},
	}
}

// endPosition is the position at the very end of the document.
func (d *Document) endPosition() protocol.Position {
	lines := strings.Split(d.Text, "\n")
	last := len(lines) - 1

	return protocol.Position{Line: uint32(last), Character: uint32(len(lines[last]))}
}

func leadingWhitespace(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " \t"))]
}
