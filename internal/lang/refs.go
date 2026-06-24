package lang

import (
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// occurrences returns the ranges of every indexed entry that shares the symbol
// at pos (including the one under the cursor). Returns nil if pos is not on a
// known symbol.
//
// Scope note: the index only covers the current document, so references and
// rename are limited to this file. Symbols defined in @include'd files are still
// found where they are *used* here, but occurrences in other files are not
// touched.
func (d *Document) occurrences(pos protocol.Position) []protocol.Range {
	ix := d.Index()
	e, ok := ix.at(pos)
	if !ok || e.sym == "" {
		return nil
	}
	seen := map[protocol.Range]bool{}
	var ranges []protocol.Range
	for _, x := range ix.entries {
		if x.sym != e.sym || seen[x.rng] {
			continue
		}
		seen[x.rng] = true
		ranges = append(ranges, x.rng)
	}

	return ranges
}

// References returns the locations of all occurrences of the symbol at pos.
func (d *Document) References(pos protocol.Position) []protocol.Location {
	ranges := d.occurrences(pos)
	if ranges == nil {
		return nil
	}
	locs := make([]protocol.Location, 0, len(ranges))
	for _, r := range ranges {
		locs = append(locs, protocol.Location{URI: d.URI, Range: r})
	}

	return locs
}

// SymbolAt returns the index symbol key at pos (e.g. "callable:HELLO"), used to
// drive workspace-wide references and rename.
func (d *Document) SymbolAt(pos protocol.Position) (string, bool) {
	e, ok := d.Index().at(pos)
	if !ok || e.sym == "" {
		return "", false
	}

	return e.sym, true
}

// SymbolOccurrences returns every range in this document with the given symbol
// key (used by workspace references/rename to aggregate across files).
func (d *Document) SymbolOccurrences(sym string) []protocol.Range {
	var ranges []protocol.Range
	for _, e := range d.Index().entries {
		if e.sym == sym {
			ranges = append(ranges, e.rng)
		}
	}

	return ranges
}

// DocumentHighlights returns highlight ranges for every occurrence of the symbol
// at pos (used to highlight a symbol's uses under the cursor).
func (d *Document) DocumentHighlights(pos protocol.Position) []protocol.DocumentHighlight {
	ranges := d.occurrences(pos)
	out := make([]protocol.DocumentHighlight, 0, len(ranges))
	kind := protocol.DocumentHighlightKindText
	for _, r := range ranges {
		out = append(out, protocol.DocumentHighlight{Range: r, Kind: &kind})
	}

	return out
}

// PrepareRename returns the range of the renameable symbol at pos, or ok=false
// if pos is not on one (so the client blocks the rename before prompting).
func (d *Document) PrepareRename(pos protocol.Position) (protocol.Range, bool) {
	e, ok := d.Index().at(pos)
	if !ok || e.sym == "" {
		return protocol.Range{}, false
	}

	return e.rng, true
}

// Rename returns a workspace edit that renames every occurrence of the symbol at
// pos to newName. Returns nil if pos is not on a renameable symbol.
func (d *Document) Rename(pos protocol.Position, newName string) *protocol.WorkspaceEdit {
	ranges := d.occurrences(pos)
	if ranges == nil {
		return nil
	}
	edits := make([]protocol.TextEdit, 0, len(ranges))
	for _, r := range ranges {
		edits = append(edits, protocol.TextEdit{Range: r, NewText: newName})
	}

	return &protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentUri][]protocol.TextEdit{
			d.URI: edits,
		},
	}
}
