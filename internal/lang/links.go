package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

var includeRe = regexp.MustCompile(`@include\s+"([^"]+)"`)

// DocumentLinks turns each `@include "path"` into a clickable link to the
// resolved file (searched along the document's MRO paths).
func (d *Document) DocumentLinks() []protocol.DocumentLink {
	matches := includeRe.FindAllStringSubmatchIndex(d.Text, -1)
	links := make([]protocol.DocumentLink, 0, len(matches))
	for _, m := range matches {
		pathStart, pathEnd := m[2], m[3] // the path inside the quotes
		rel := d.Text[pathStart:pathEnd]
		uri := PathToURI(d.resolveInclude(rel))
		links = append(links, protocol.DocumentLink{
			Range:  protocol.Range{Start: d.offsetToPos(pathStart), End: d.offsetToPos(pathEnd)},
			Target: &uri,
		})
	}

	return links
}

// resolveInclude resolves an include path against the MRO search paths, falling
// back to the first candidate if none exists on disk yet.
func (d *Document) resolveInclude(rel string) string {
	var first string
	for _, dir := range d.MROPaths() {
		cand := filepath.Join(dir, rel)
		if first == "" {
			first = cand
		}
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	if first == "" {
		first = rel
	}

	return first
}

// offsetToPos converts a byte offset in the document text to an LSP Position.
func (d *Document) offsetToPos(off int) protocol.Position {
	line := strings.Count(d.Text[:off], "\n")
	col := off - (strings.LastIndex(d.Text[:off], "\n") + 1)

	return protocol.Position{Line: uint32(line), Character: uint32(col)}
}
