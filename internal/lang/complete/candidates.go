package complete

import "github.com/martian-lang/martian/martian/syntax"

// CandKind is a neutral candidate category, mapped to protocol kinds by the
// caller (keeps this package free of glsp).
type CandKind int

// Candidate categories (mapped to LSP CompletionItemKind by the caller).
const (
	CKeyword CandKind = iota
	CFunction
	CClass
	CField
	CType
	CSnippet
)

// Candidate is one completion suggestion. When Snippet is non-empty the item is
// inserted as an LSP snippet (with $-placeholders) instead of plain text.
type Candidate struct {
	Label   string
	Detail  string
	Kind    CandKind
	Snippet string
}

// Candidates produces suggestions for a context against the symbol snapshot.
// The snapshot may be nil (no successful parse yet); AST-backed contexts then
// yield nothing, while static keyword/type contexts still work.
func Candidates(ctx Context, ast *syntax.Ast) []Candidate {
	switch ctx.Kind {
	case TopLevel:
		return topLevelSnippets()
	case PipelineBody:
		return pipelineBodySnippets()
	case SrcLang:
		return keywords("py", "exec", "comp")
	case InOutType:
		return typeCandidates(ast)
	case CallName:
		return callableCandidates(ast)
	case BindingLHS:
		return paramCandidates(lookupCallable(ast, ctx.Name), true)
	case SelfRef:
		return paramCandidates(findPipeline(ast, ctx.Pipeline), true)
	case OutputRef:
		return paramCandidates(resolveCallRef(ast, ctx.Pipeline, ctx.Name), false)
	default:
		return nil
	}
}

// topLevelSnippets are scaffolds offered at file scope.
func topLevelSnippets() []Candidate {
	return []Candidate{
		snippet("stage", "stage ${1:NAME}(\n\tin  ${2:type} ${3:name},\n\tout ${4:type} ${5:name},\n\tsrc ${6:py} \"${7:stages/name}\",\n)\n"),
		snippet("pipeline", "pipeline ${1:NAME}(\n\tin  ${2:type} ${3:name},\n\tout ${4:type} ${5:name},\n)\n{\n\t$0\n}\n"),
		snippet("filetype", "filetype ${1:ext};"),
		snippet("struct", "struct ${1:NAME}(\n\t${2:type} ${3:name},\n)"),
		snippet("call", "call ${1:NAME}(\n\t$0\n)"),
		snippet("@include", "@include \"${1:path.mro}\""),
	}
}

// pipelineBodySnippets are scaffolds offered inside a pipeline body.
func pipelineBodySnippets() []Candidate {
	return []Candidate{
		snippet("call", "call ${1:NAME}(\n\t$0\n)"),
		snippet("return", "return (\n\t$0\n)"),
		snippet("retain", "retain (\n\t$0\n)"),
	}
}

func snippet(label, body string) Candidate {
	return Candidate{Label: label, Detail: "snippet", Kind: CSnippet, Snippet: body}
}

func keywords(words ...string) []Candidate {
	out := make([]Candidate, 0, len(words))
	for _, w := range words {
		out = append(out, Candidate{Label: w, Kind: CKeyword})
	}

	return out
}

func builtinTypes() []string {
	return []string{"int", "float", "string", "bool", "path", "file", "map"}
}

func typeCandidates(ast *syntax.Ast) []Candidate {
	bt := builtinTypes()
	out := make([]Candidate, 0, len(bt))
	for _, t := range bt {
		out = append(out, Candidate{Label: t, Detail: "builtin", Kind: CType})
	}
	if ast == nil {
		return out
	}
	for _, u := range ast.UserTypes {
		out = append(out, Candidate{Label: u.GetId(), Detail: "filetype", Kind: CType})
	}
	for _, s := range ast.StructTypes {
		out = append(out, Candidate{Label: s.GetId(), Detail: "struct", Kind: CClass})
	}

	return out
}

func callableCandidates(ast *syntax.Ast) []Candidate {
	if ast == nil || ast.Callables == nil {
		return nil
	}
	out := make([]Candidate, 0, len(ast.Callables.List))
	for _, c := range ast.Callables.List {
		kind := CFunction
		if c.Type() == "stage" {
			kind = CClass
		}
		out = append(out, Candidate{Label: c.GetId(), Detail: c.Type(), Kind: kind})
	}

	return out
}

// paramCandidates lists the input (in=true) or output (in=false) parameters of a
// callable/pipeline. The argument is taken as a Callable so it serves stages,
// pipelines, and the resolved call target uniformly.
func paramCandidates(c syntax.Callable, in bool) []Candidate {
	if c == nil {
		return nil
	}
	if in {
		ps := c.GetInParams()
		if ps == nil {
			return nil
		}
		out := make([]Candidate, 0, len(ps.List))
		for _, p := range ps.List {
			out = append(out, Candidate{Label: p.Id, Detail: "in " + typeName(p.Tname), Kind: CField})
		}

		return out
	}
	ps := c.GetOutParams()
	if ps == nil {
		return nil
	}
	out := make([]Candidate, 0, len(ps.List))
	for _, p := range ps.List {
		out = append(out, Candidate{Label: p.Id, Detail: "out " + typeName(p.Tname), Kind: CField})
	}

	return out
}

// lookupCallable resolves a callable by name (DecId), preferring the O(1) table
// but falling back to a List scan since the table may be unpopulated when an
// earlier compile pass failed.
func lookupCallable(ast *syntax.Ast, name string) syntax.Callable {
	if ast == nil || ast.Callables == nil {
		return nil
	}
	if ast.Callables.Table != nil {
		if c, ok := ast.Callables.Table[name]; ok {
			return c
		}
	}
	for _, c := range ast.Callables.List {
		if c.GetId() == name {
			return c
		}
	}

	return nil
}

func findPipeline(ast *syntax.Ast, name string) syntax.Callable {
	if ast == nil || name == "" {
		return nil
	}
	for _, p := range ast.Pipelines {
		if p.Id == name {
			return p
		}
	}

	return nil
}

// resolveCallRef maps a call reference (`alias.`) inside a pipeline to the called
// callable, so its outputs can be completed.
func resolveCallRef(ast *syntax.Ast, pipeline, alias string) syntax.Callable {
	pl, _ := findPipeline(ast, pipeline).(*syntax.Pipeline)
	if pl == nil {
		// Not inside a known pipeline: best effort — treat the base as a callable.
		return lookupCallable(ast, alias)
	}
	for _, c := range pl.Calls {
		if c.Id == alias {
			return lookupCallable(ast, c.DecId)
		}
	}

	return lookupCallable(ast, alias)
}

func typeName(t syntax.TypeId) string {
	return t.String()
}
