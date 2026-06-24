package lang

import (
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// posInDoc returns a Position just inside token, on the first line containing
// lineContains.
func posInDoc(t *testing.T, text, lineContains, token string) protocol.Position {
	t.Helper()
	for i, ln := range strings.Split(text, "\n") {
		if !strings.Contains(ln, lineContains) {
			continue
		}
		idx := strings.Index(ln, token)
		if idx < 0 {
			t.Fatalf("token %q not found on line %q", token, ln)
		}

		return protocol.Position{Line: uint32(i), Character: uint32(idx + 1)}
	}
	t.Fatalf("no line contains %q", lineContains)

	return protocol.Position{}
}

// posAt returns a Position `extra` characters past the start of token, on the
// first line containing lineContains (used to target a sub-token like the
// ".greeting" part of "HELLO.greeting").
func posAt(t *testing.T, text, lineContains, token string, extra int) protocol.Position {
	t.Helper()
	for i, ln := range strings.Split(text, "\n") {
		if !strings.Contains(ln, lineContains) {
			continue
		}
		idx := strings.Index(ln, token)
		if idx < 0 {
			t.Fatalf("token %q not found on line %q", token, ln)
		}

		return protocol.Position{Line: uint32(i), Character: uint32(idx + extra)}
	}
	t.Fatalf("no line contains %q", lineContains)

	return protocol.Position{}
}

func TestHoverOnCallShowsStageSignature(t *testing.T) {
	d := docFrom(validMRO)
	pos := posInDoc(t, validMRO, "call HELLO", "HELLO")
	md, ok := d.Hover(pos)
	if !ok {
		t.Fatal("expected hover on call HELLO")
	}
	if !strings.Contains(md, "stage HELLO") {
		t.Errorf("hover = %q, want it to contain \"stage HELLO\"", md)
	}
}

func TestDefinitionOnCallJumpsToStage(t *testing.T) {
	d := docFrom(validMRO)
	pos := posInDoc(t, validMRO, "call HELLO", "HELLO")
	loc, ok := d.Definition(pos)
	if !ok {
		t.Fatal("expected definition for call HELLO")
	}
	if loc.Range.Start.Line != 2 { // "stage HELLO(" is the 3rd line (index 2)
		t.Errorf("definition line = %d, want 2", loc.Range.Start.Line)
	}
	if loc.URI != d.URI {
		t.Errorf("definition URI = %q, want %q", loc.URI, d.URI)
	}
}

func TestHoverAndDefinitionOnOutputRef(t *testing.T) {
	d := docFrom(validMRO)
	// Target the ".greeting" (output) part of "HELLO.greeting".
	pos := posAt(t, validMRO, "greeting = HELLO.greeting", "HELLO.greeting", len("HELLO.")+1)

	md, ok := d.Hover(pos)
	if !ok || !strings.Contains(md, "out string greeting") {
		t.Errorf("hover = %q, want \"out string greeting\"", md)
	}

	loc, ok := d.Definition(pos)
	if !ok {
		t.Fatal("expected definition for HELLO.greeting")
	}
	if loc.Range.Start.Line != 4 { // "out string greeting" in the stage (index 4)
		t.Errorf("definition line = %d, want 4", loc.Range.Start.Line)
	}
}

func TestDefinitionOnSelfRefJumpsToPipelineInput(t *testing.T) {
	d := docFrom(validMRO)
	// Target the "name" part of "self.name".
	pos := posAt(t, validMRO, "name = self.name", "self.name", len("self.")+1)

	md, ok := d.Hover(pos)
	if !ok || !strings.Contains(md, "in string name") {
		t.Errorf("hover = %q, want \"in string name\"", md)
	}

	loc, ok := d.Definition(pos)
	if !ok {
		t.Fatal("expected definition for self.name")
	}
	if loc.Range.Start.Line != 9 { // pipeline "in  string name" (index 9)
		t.Errorf("definition line = %d, want 9", loc.Range.Start.Line)
	}
}

func TestHoverOnParamDeclaration(t *testing.T) {
	d := docFrom(validMRO)
	// The stage's own "out string greeting" declaration.
	pos := posInDoc(t, validMRO, "out string greeting", "greeting")
	md, ok := d.Hover(pos)
	if !ok || !strings.Contains(md, "out string greeting") {
		t.Errorf("hover = %q, want \"out string greeting\"", md)
	}
}
