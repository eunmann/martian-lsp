package lang

import (
	"strings"
	"testing"
)

const validMRO = `filetype txt;

stage HELLO(
    in  string name,
    out string greeting,
    src py     "stages/hello",
)

pipeline HELLO_BATCH(
    in  string name,
    out string greeting,
)
{
    call HELLO(
        name = self.name,
    )
    return (
        greeting = HELLO.greeting,
    )
}
`

func docFrom(text string) *Document {
	return &Document{
		URI:  "file:///tmp/martian-lsp-test/pipe.mro",
		Path: "/tmp/martian-lsp-test/pipe.mro",
		Text: text,
	}
}

// totalDiagnostics counts diagnostics across all files for the document.
func totalDiagnostics(d *Document) int {
	n := 0
	for _, fd := range d.Diagnose() {
		n += len(fd.Diagnostics)
	}

	return n
}

func TestValidPipelineHasNoDiagnostics(t *testing.T) {
	d := docFrom(validMRO)
	if got := totalDiagnostics(d); got != 0 {
		for _, fd := range d.Diagnose() {
			for _, diag := range fd.Diagnostics {
				t.Logf("unexpected diag @%d: %s", diag.Range.Start.Line, diag.Message)
			}
		}
		t.Fatalf("valid pipeline: want 0 diagnostics, got %d", got)
	}
}

func TestUndefinedOutputProducesDiagnostic(t *testing.T) {
	// HELLO has no output named "missing"; the return wiring is invalid.
	broken := strings.Replace(validMRO,
		"greeting = HELLO.greeting,",
		"greeting = HELLO.missing,", 1)
	d := docFrom(broken)
	if got := totalDiagnostics(d); got == 0 {
		t.Fatalf("undefined output: want >=1 diagnostic, got 0")
	}
}

func TestUndefinedStageCallProducesDiagnostic(t *testing.T) {
	// Call a stage that was never declared.
	broken := strings.Replace(validMRO,
		"call HELLO(",
		"call NOPE(", 1)
	d := docFrom(broken)
	diags := d.Diagnose()
	n := 0
	for _, fd := range diags {
		n += len(fd.Diagnostics)
	}
	if n == 0 {
		t.Fatalf("undefined stage: want >=1 diagnostic, got 0")
	}
}

func TestSyntaxErrorProducesDiagnostic(t *testing.T) {
	// Remove a closing paren to force a parse error.
	broken := strings.Replace(validMRO,
		"    out string greeting,\n    src py",
		"    out string greeting\n    src py", 1) // missing comma; still parses? force harder:
	broken = strings.Replace(broken, "pipeline HELLO_BATCH(", "pipeline HELLO_BATCH(((", 1)
	d := docFrom(broken)
	n := 0
	for _, fd := range d.Diagnose() {
		n += len(fd.Diagnostics)
	}
	if n == 0 {
		t.Fatalf("syntax error: want >=1 diagnostic, got 0")
	}
}

func TestCleanDocAlwaysPublishesOwnURI(t *testing.T) {
	d := docFrom(validMRO)
	fds := d.Diagnose()
	found := false
	for _, fd := range fds {
		if fd.URI == d.URI {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected own URI %q to be present (to clear stale diagnostics)", d.URI)
	}
}
