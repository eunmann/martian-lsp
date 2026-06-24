package lang

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func findSym(syms []protocol.DocumentSymbol, name string) (protocol.DocumentSymbol, bool) {
	for _, s := range syms {
		if s.Name == name {
			return s, true
		}
	}

	return protocol.DocumentSymbol{}, false
}

func TestSymbolsTopLevel(t *testing.T) {
	d := docFrom(validMRO)
	syms := d.Symbols()

	stage, ok := findSym(syms, "HELLO")
	if !ok {
		t.Fatalf("missing HELLO stage symbol; got %d symbols", len(syms))
	}
	if stage.Kind != protocol.SymbolKindClass {
		t.Errorf("HELLO kind = %v, want Class", stage.Kind)
	}

	pipe, ok := findSym(syms, "HELLO_BATCH")
	if !ok {
		t.Fatalf("missing HELLO_BATCH pipeline symbol")
	}
	if pipe.Kind != protocol.SymbolKindFunction {
		t.Errorf("HELLO_BATCH kind = %v, want Function", pipe.Kind)
	}
}

func TestSymbolChildren(t *testing.T) {
	d := docFrom(validMRO)
	syms := d.Symbols()

	stage, _ := findSym(syms, "HELLO")
	if _, ok := findSym(stage.Children, "name"); !ok {
		t.Errorf("HELLO missing 'name' in-param child")
	}
	if _, ok := findSym(stage.Children, "greeting"); !ok {
		t.Errorf("HELLO missing 'greeting' out-param child")
	}

	pipe, _ := findSym(syms, "HELLO_BATCH")
	if _, ok := findSym(pipe.Children, "HELLO"); !ok {
		t.Errorf("HELLO_BATCH missing 'HELLO' call child")
	}
}

func TestSymbolSelectionRangeOnName(t *testing.T) {
	d := docFrom(validMRO)
	syms := d.Symbols()
	stage, _ := findSym(syms, "HELLO")

	// "stage HELLO(" -> HELLO starts at column 6 (0-indexed).
	sel := stage.SelectionRange
	lines := splitLines(d.Text)
	got := lines[sel.Start.Line][sel.Start.Character:sel.End.Character]
	if got != "HELLO" {
		t.Errorf("selection range covers %q, want \"HELLO\"", got)
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])

	return out
}
