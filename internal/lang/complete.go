package lang

import (
	"strings"

	"github.com/eunmann/martian-lsp/internal/lang/complete"
	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Complete returns completion items at pos. The snapshot supplies symbols (it may
// be the current AST or the last-good one if the buffer no longer parses); the
// context is classified purely from the live text.
func (d *Document) Complete(snapshot *syntax.Ast, pos protocol.Position) []protocol.CompletionItem {
	ctx := complete.Classify(d.Text, int(pos.Line), int(pos.Character))
	if ctx.Kind == complete.None {
		return nil
	}

	cands := complete.Candidates(ctx, snapshot)
	prefix := strings.ToLower(ctx.Prefix)
	items := make([]protocol.CompletionItem, 0, len(cands))
	for _, c := range cands {
		if prefix != "" && !strings.HasPrefix(strings.ToLower(c.Label), prefix) {
			continue
		}
		items = append(items, completionItem(c, ctx, pos))
	}

	return items
}

// completionItem builds a protocol item with a replace TextEdit so the typed
// prefix is overwritten rather than duplicated.
func completionItem(c complete.Candidate, ctx complete.Context, pos protocol.Position) protocol.CompletionItem {
	kind := candKind(c.Kind)
	newText := c.Label
	if c.Snippet != "" {
		newText = c.Snippet
	}
	item := protocol.CompletionItem{
		Label: c.Label,
		Kind:  &kind,
		TextEdit: protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{Line: pos.Line, Character: uint32(ctx.ReplaceStart)},
				End:   pos,
			},
			NewText: newText,
		},
	}
	if c.Snippet != "" {
		snip := protocol.InsertTextFormatSnippet
		item.InsertTextFormat = &snip
	}
	if c.Detail != "" {
		item.Detail = &c.Detail
	}

	return item
}

func candKind(k complete.CandKind) protocol.CompletionItemKind {
	switch k {
	case complete.CKeyword:
		return protocol.CompletionItemKindKeyword
	case complete.CFunction:
		return protocol.CompletionItemKindFunction
	case complete.CClass:
		return protocol.CompletionItemKindClass
	case complete.CField:
		return protocol.CompletionItemKindField
	case complete.CType:
		return protocol.CompletionItemKindTypeParameter
	case complete.CSnippet:
		return protocol.CompletionItemKindSnippet
	default:
		return protocol.CompletionItemKindText
	}
}
