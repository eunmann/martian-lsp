package lang

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestInlayHints(t *testing.T) {
	d := docFrom(validMRO)
	full := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 1000, Character: 0},
	}
	hints := d.InlayHints(nil, full)

	// call HELLO(name = ...) -> ": string"; return(greeting = ...) -> ": string"
	if len(hints) != 2 {
		t.Fatalf("inlay hints = %d, want 2: %+v", len(hints), hints)
	}
	for _, h := range hints {
		if h.Label != ": string" {
			t.Errorf("hint label = %q, want \": string\"", h.Label)
		}
		if h.Kind != inlayKindType {
			t.Errorf("hint kind = %d, want %d", h.Kind, inlayKindType)
		}
	}
}

func TestInlayHintsRangeFilter(t *testing.T) {
	d := docFrom(validMRO)
	// A range covering only line 0 should yield no binding hints (bindings are
	// deeper in the file).
	narrow := protocol.Range{
		Start: protocol.Position{Line: 0, Character: 0},
		End:   protocol.Position{Line: 0, Character: 0},
	}
	if got := d.InlayHints(nil, narrow); len(got) != 0 {
		t.Errorf("narrow range produced %d hints, want 0", len(got))
	}
}
