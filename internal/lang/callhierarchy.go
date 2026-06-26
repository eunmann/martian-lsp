package lang

import (
	"strings"

	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

const callableSymPrefix = "callable:"

// PrepareCallHierarchy returns the callable (stage/pipeline) at pos as a call
// hierarchy item, or nil if the cursor isn't on a callable.
func (d *Document) PrepareCallHierarchy(snapshot *syntax.Ast, pos protocol.Position) []protocol.CallHierarchyItem {
	e, ok := d.Index().at(pos)
	if !ok || !strings.HasPrefix(e.sym, callableSymPrefix) {
		return nil
	}
	c := lookupCallable(snapshot, strings.TrimPrefix(e.sym, callableSymPrefix))
	if c == nil {
		return nil
	}

	return []protocol.CallHierarchyItem{d.callItem(c)}
}

// IncomingCalls reports the pipelines that call the item's callable.
func (d *Document) IncomingCalls(snapshot *syntax.Ast, item protocol.CallHierarchyItem) []protocol.CallHierarchyIncomingCall {
	if snapshot == nil {
		return nil
	}
	target := itemID(item)
	var out []protocol.CallHierarchyIncomingCall
	for _, pl := range snapshot.Pipelines {
		var ranges []protocol.Range
		for _, c := range pl.Calls {
			if c.DecId == target {
				ranges = append(ranges, d.tokenRangeFor(c.Node.Loc, c.DecId))
			}
		}
		if len(ranges) > 0 {
			out = append(out, protocol.CallHierarchyIncomingCall{From: d.callItem(pl), FromRanges: ranges})
		}
	}

	return out
}

// OutgoingCalls reports the callables that the item's pipeline calls. Stages have
// no outgoing calls.
func (d *Document) OutgoingCalls(snapshot *syntax.Ast, item protocol.CallHierarchyItem) []protocol.CallHierarchyOutgoingCall {
	if snapshot == nil {
		return nil
	}
	pl, ok := lookupCallable(snapshot, itemID(item)).(*syntax.Pipeline)
	if !ok || pl == nil {
		return nil
	}

	byCallee := map[string][]protocol.Range{}
	var order []string
	for _, c := range pl.Calls {
		if _, seen := byCallee[c.DecId]; !seen {
			order = append(order, c.DecId)
		}
		byCallee[c.DecId] = append(byCallee[c.DecId], d.tokenRangeFor(c.Node.Loc, c.DecId))
	}

	out := make([]protocol.CallHierarchyOutgoingCall, 0, len(order))
	for _, decID := range order {
		callee := lookupCallable(snapshot, decID)
		if callee == nil {
			continue
		}
		out = append(out, protocol.CallHierarchyOutgoingCall{
			To:         d.callItem(callee),
			FromRanges: byCallee[decID],
		})
	}

	return out
}

// callItem builds a CallHierarchyItem for a callable.
func (d *Document) callItem(c syntax.Callable) protocol.CallHierarchyItem {
	loc := callableLoc(c)
	kind := protocol.SymbolKindFunction
	if c.Type() == kindStage {
		kind = protocol.SymbolKindClass
	}
	uri := d.URI
	if loc.File != nil && loc.File.FullPath != "" && loc.File.FullPath != d.Path {
		uri = PathToURI(loc.File.FullPath)
	}
	rng := d.tokenRangeFor(loc, c.GetId())
	detail := c.Type()

	return protocol.CallHierarchyItem{
		Name:           c.GetId(),
		Kind:           kind,
		Detail:         &detail,
		URI:            uri,
		Range:          rng,
		SelectionRange: rng,
		Data:           c.GetId(),
	}
}

// itemID recovers the callable id from an item round-tripped through the client.
func itemID(item protocol.CallHierarchyItem) string {
	if s, ok := item.Data.(string); ok && s != "" {
		return s
	}

	return item.Name
}
