package lang

import (
	"sort"
	"strings"
)

// Semantic token type indices into the legend returned by SemanticTokenLegend.
const (
	tokenFunction  uint32 = 0 // stages and pipelines
	tokenParameter uint32 = 1 // in/out parameters and references to them

	// tokenStride is the number of uint32s per token in the LSP encoding:
	// (deltaLine, deltaStart, length, tokenType, tokenModifiers).
	tokenStride = 5
)

// SemanticTokenLegend is the ordered list of token type names the encoder uses.
// The index of each name is the token type emitted in the token data.
func SemanticTokenLegend() []string {
	return []string{"function", "parameter"}
}

// SemanticTokens returns LSP semantic tokens for the document, encoded as the
// flat quintuple (deltaLine, deltaStart, length, tokenType, modifiers) stream
// the protocol requires. It reuses the position index, so coloring stays
// consistent with hover/definition.
func (d *Document) SemanticTokens() []uint32 {
	type tok struct {
		line, start, length, typ uint32
	}

	ix := d.Index()
	toks := make([]tok, 0, len(ix.entries))
	for _, e := range ix.entries {
		if e.rng.Start.Line != e.rng.End.Line {
			continue // semantic tokens may not span lines
		}
		typ, ok := tokenType(e.sym)
		if !ok {
			continue
		}
		toks = append(toks, tok{
			line:   e.rng.Start.Line,
			start:  e.rng.Start.Character,
			length: e.rng.End.Character - e.rng.Start.Character,
			typ:    typ,
		})
	}

	sort.Slice(toks, func(i, j int) bool {
		if toks[i].line != toks[j].line {
			return toks[i].line < toks[j].line
		}

		return toks[i].start < toks[j].start
	})

	data := make([]uint32, 0, len(toks)*tokenStride)
	var prevLine, prevStart uint32
	for i, t := range toks {
		// Skip a duplicate token at the same position (defensive).
		if i > 0 && t.line == toks[i-1].line && t.start == toks[i-1].start {
			continue
		}
		deltaLine := t.line - prevLine
		deltaStart := t.start
		if deltaLine == 0 {
			deltaStart = t.start - prevStart
		}
		data = append(data, deltaLine, deltaStart, t.length, t.typ, 0)
		prevLine, prevStart = t.line, t.start
	}

	return data
}

func tokenType(sym string) (uint32, bool) {
	switch {
	case strings.HasPrefix(sym, "callable:"):
		return tokenFunction, true
	case strings.HasPrefix(sym, "in:"), strings.HasPrefix(sym, "out:"):
		return tokenParameter, true
	default:
		return 0, false
	}
}
