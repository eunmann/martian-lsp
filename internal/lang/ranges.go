package lang

import (
	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// tokenRange returns the LSP range of the standalone token name on the given
// 1-indexed line of this document.
func (d *Document) tokenRange(line int, name string) protocol.Range {
	_, sel := d.nameRange(line, name)

	return sel
}

// tokenRangeFor returns the range of name at a location. If the location is in
// this document we search its text for a precise token; otherwise (an included
// file we don't have open) we fall back to the AST's column.
func (d *Document) tokenRangeFor(loc syntax.SourceLoc, name string) protocol.Range {
	if loc.File == nil || loc.File.FullPath == "" || loc.File.FullPath == d.Path {
		return d.tokenRange(loc.Line, name)
	}
	l := max(loc.Line-1, 0)
	start := max(loc.Col-1, 0)

	return protocol.Range{
		Start: protocol.Position{Line: uint32(l), Character: uint32(start)},
		End:   protocol.Position{Line: uint32(l), Character: uint32(start + len(name))},
	}
}

// locationOf builds an LSP Location for a declaration at loc, ranged over name,
// resolving the URI for the current document or an included file.
func (d *Document) locationOf(loc syntax.SourceLoc, name string) protocol.Location {
	uri := d.URI
	if loc.File != nil && loc.File.FullPath != "" && loc.File.FullPath != d.Path {
		uri = PathToURI(loc.File.FullPath)
	}

	return protocol.Location{URI: uri, Range: d.tokenRangeFor(loc, name)}
}

// Hover returns markdown describing the symbol at pos, if any.
func (d *Document) Hover(pos protocol.Position) (string, bool) {
	e, ok := d.Index().at(pos)
	if !ok || e.hover == "" {
		return "", false
	}

	return e.hover, true
}

// Definition returns the definition location of the symbol at pos, if any.
func (d *Document) Definition(pos protocol.Position) (protocol.Location, bool) {
	// A stage's `src "path"` jumps to the implementation file on disk.
	if loc, ok := d.srcDefinition(pos); ok {
		return loc, true
	}

	e, ok := d.Index().at(pos)
	if !ok || e.def == nil {
		return protocol.Location{}, false
	}

	return *e.def, true
}
