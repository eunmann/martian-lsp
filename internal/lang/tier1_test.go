package lang

import (
	"strings"
	"testing"
)

func TestDocumentHighlights(t *testing.T) {
	d := docFrom(validMRO)
	pos := posInDoc(t, validMRO, "stage HELLO(", "HELLO")
	hl := d.DocumentHighlights(pos)
	if len(hl) != 3 { // def + call + ref, same as references
		t.Fatalf("highlights = %d, want 3", len(hl))
	}
}

func TestPrepareRename(t *testing.T) {
	d := docFrom(validMRO)

	pos := posInDoc(t, validMRO, "stage HELLO(", "HELLO")
	rng, ok := d.PrepareRename(pos)
	if !ok {
		t.Fatal("expected prepareRename to allow renaming HELLO")
	}
	lines := strings.Split(validMRO, "\n")
	got := lines[rng.Start.Line][rng.Start.Character:rng.End.Character]
	if got != "HELLO" {
		t.Errorf("prepareRename range = %q, want HELLO", got)
	}

	// A keyword position is not renameable.
	kw := posInDoc(t, validMRO, "stage HELLO(", "stage")
	if _, ok := d.PrepareRename(kw); ok {
		t.Error("prepareRename should reject the 'stage' keyword")
	}
}

func TestSignatureHelp(t *testing.T) {
	snap := mustCompile(t, validMRO) // HELLO(in string name) ...
	text, pos := cur("pipeline X(\n) {\n  call HELLO(█")
	sh := docFrom(text).SignatureHelp(snap, pos)
	if sh == nil || len(sh.Signatures) == 0 {
		t.Fatal("expected signature help for call HELLO(")
	}
	if !strings.Contains(sh.Signatures[0].Label, "HELLO(") {
		t.Errorf("signature label = %q", sh.Signatures[0].Label)
	}
	found := false
	for _, p := range sh.Signatures[0].Parameters {
		if p.Label == "name" {
			found = true
		}
	}
	if !found {
		t.Error("signature missing 'name' parameter")
	}
	if sh.ActiveParameter == nil || *sh.ActiveParameter != 0 {
		t.Errorf("active parameter = %v, want 0", sh.ActiveParameter)
	}
}

func TestTypeDefinition(t *testing.T) {
	text, pos := cur("filetype bam;\n\nstage ALIGN(\n    in  bam re█ads,\n    out bam aligned,\n    src py  \"x\",\n)\n")
	loc, ok := docFrom(text).TypeDefinition(pos)
	if !ok {
		t.Fatal("expected typeDefinition for a bam-typed param")
	}
	if loc.Range.Start.Line != 0 { // `filetype bam;` is line 0
		t.Errorf("typeDefinition line = %d, want 0", loc.Range.Start.Line)
	}
}

func TestTypeDefinitionBuiltinHasNone(t *testing.T) {
	// `name` is a string (builtin) — no type declaration to jump to.
	d := docFrom(validMRO)
	pos := posInDoc(t, validMRO, "in  string name", "name")
	if _, ok := d.TypeDefinition(pos); ok {
		t.Error("builtin-typed param should have no type definition")
	}
}

func TestDocumentLinks(t *testing.T) {
	d := &Document{
		URI:  "file:///tmp/martian-lsp-test/main.mro",
		Path: "/tmp/martian-lsp-test/main.mro",
		Text: "@include \"dna.mro\"\n\npipeline P(\n) {\n}\n",
	}
	links := d.DocumentLinks()
	if len(links) != 1 {
		t.Fatalf("links = %d, want 1", len(links))
	}
	l := links[0]
	lines := strings.Split(d.Text, "\n")
	got := lines[l.Range.Start.Line][l.Range.Start.Character:l.Range.End.Character]
	if got != "dna.mro" {
		t.Errorf("link range covers %q, want dna.mro", got)
	}
	if l.Target == nil || !strings.Contains(*l.Target, "dna.mro") {
		t.Errorf("link target = %v, want one containing dna.mro", l.Target)
	}
}
