package lang

import "testing"

const messyMRO = `filetype txt;
stage HELLO(
in string name,
out string greeting,
src py "stages/hello",
)
pipeline HELLO_BATCH(in string name, out string greeting,)
{
call HELLO(name=self.name,)
return(greeting=HELLO.greeting,)
}
`

func TestFormatProducesEditAndIsIdempotent(t *testing.T) {
	d := docFrom(messyMRO)
	edits := d.Format()
	if len(edits) != 1 {
		t.Fatalf("expected 1 format edit, got %d", len(edits))
	}
	formatted := edits[0].NewText
	if formatted == messyMRO {
		t.Fatal("formatted text equals input; expected reformatting")
	}

	// The formatted output must still compile clean...
	nd := docFrom(formatted)
	if n := totalDiagnostics(nd); n != 0 {
		t.Errorf("formatted output has %d diagnostics:\n%s", n, formatted)
	}
	// ...and be idempotent (formatting it again is a no-op).
	if again := nd.Format(); again != nil {
		t.Errorf("formatting is not idempotent; second pass produced %d edits", len(again))
	}
}

func TestFormatCleanDocReturnsNil(t *testing.T) {
	// Format messy once to get canonical text, then confirm it's stable.
	canonical := docFrom(messyMRO).Format()[0].NewText
	if edits := docFrom(canonical).Format(); edits != nil {
		t.Errorf("clean doc produced %d format edits, want 0", len(edits))
	}
}

func TestFormatSyntaxErrorReturnsNil(t *testing.T) {
	// Unbalanced parens -> parse failure -> must not clobber the file.
	broken := "pipeline P((( {\n"
	if edits := docFrom(broken).Format(); edits != nil {
		t.Errorf("syntax-error doc produced %d format edits, want 0 (no clobber)", len(edits))
	}
}
