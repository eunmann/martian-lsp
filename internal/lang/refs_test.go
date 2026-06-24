package lang

import (
	"sort"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestHoverOnCallPartOfRef(t *testing.T) {
	d := docFrom(validMRO)
	// The "HELLO" part of "HELLO.greeting" should describe the stage.
	pos := posAt(t, validMRO, "greeting = HELLO.greeting", "HELLO.greeting", 1)
	md, ok := d.Hover(pos)
	if !ok || !strings.Contains(md, "stage HELLO") {
		t.Errorf("hover on call part = %q, want \"stage HELLO\"", md)
	}
}

func TestReferencesOfStage(t *testing.T) {
	d := docFrom(validMRO)
	pos := posInDoc(t, validMRO, "stage HELLO(", "HELLO")
	refs := d.References(pos)
	// def (l2) + call DecId (l13) + RefExp call-part (l17) = 3
	if len(refs) != 3 {
		t.Fatalf("references of HELLO = %d, want 3: %+v", len(refs), refs)
	}
}

func TestReferencesOfOutputParam(t *testing.T) {
	d := docFrom(validMRO)
	// stage HELLO's output "greeting" (its decl line).
	pos := posInDoc(t, validMRO, "out string greeting", "greeting")
	refs := d.References(pos)
	// out decl (l4) + RefExp output-part (l17) = 2
	if len(refs) != 2 {
		t.Fatalf("references of HELLO.greeting = %d, want 2: %+v", len(refs), refs)
	}
}

func TestRenameStageUpdatesAllOccurrences(t *testing.T) {
	d := docFrom(validMRO)
	pos := posInDoc(t, validMRO, "call HELLO(", "HELLO")
	we := d.Rename(pos, "HI")
	if we == nil {
		t.Fatal("expected a workspace edit")
	}
	edits := we.Changes[d.URI]
	if len(edits) != 3 {
		t.Fatalf("rename edits = %d, want 3", len(edits))
	}
	for _, e := range edits {
		if e.NewText != "HI" {
			t.Errorf("edit NewText = %q, want HI", e.NewText)
		}
	}

	// Apply and confirm the rename hit the stage everywhere but left the
	// unrelated pipeline HELLO_BATCH alone, and the result still compiles.
	out := applyEdits(d.Text, edits)
	for _, gone := range []string{"stage HELLO(", "call HELLO(", "HELLO.greeting"} {
		if strings.Contains(out, gone) {
			t.Errorf("renamed doc still contains %q:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "pipeline HELLO_BATCH(") {
		t.Errorf("rename wrongly touched HELLO_BATCH:\n%s", out)
	}
	for _, want := range []string{"stage HI(", "call HI(", "HI.greeting"} {
		if !strings.Contains(out, want) {
			t.Errorf("renamed doc missing %q:\n%s", want, out)
		}
	}
	nd := docFrom(out)
	if n := totalDiagnostics(nd); n != 0 {
		t.Errorf("renamed doc has %d diagnostics, want 0", n)
	}
}

func TestRenameInputParamUpdatesBindingLHS(t *testing.T) {
	d := docFrom(validMRO)
	// stage HELLO's input "name".
	pos := posInDoc(t, validMRO, "in  string name", "name")
	we := d.Rename(pos, "who")
	if we == nil {
		t.Fatal("expected a workspace edit")
	}
	edits := we.Changes[d.URI]
	// in decl (l3) + call-binding LHS "name =" (l14) = 2
	if len(edits) != 2 {
		t.Fatalf("rename in-param edits = %d, want 2: %+v", len(edits), edits)
	}
	out := applyEdits(d.Text, edits)
	nd := docFrom(out)
	if n := totalDiagnostics(nd); n != 0 {
		t.Errorf("renamed doc has %d diagnostics, want 0:\n%s", n, out)
	}
	// self.name (the pipeline input) must be untouched: still present.
	if !strings.Contains(out, "self.name") {
		t.Errorf("rename wrongly touched self.name:\n%s", out)
	}
}

// ---- test helpers ----

// applyEdits applies single-line token TextEdits to text (last-to-first so
// earlier offsets stay valid).
func applyEdits(text string, edits []protocol.TextEdit) string {
	lines := strings.Split(text, "\n")
	type e struct {
		line, sc, ec int
		nt           string
	}
	es := make([]e, 0, len(edits))
	for _, ed := range edits {
		es = append(es, e{
			line: int(ed.Range.Start.Line),
			sc:   int(ed.Range.Start.Character),
			ec:   int(ed.Range.End.Character),
			nt:   ed.NewText,
		})
	}
	sort.Slice(es, func(i, j int) bool {
		if es[i].line != es[j].line {
			return es[i].line > es[j].line
		}

		return es[i].sc > es[j].sc
	})
	for _, x := range es {
		l := lines[x.line]
		lines[x.line] = l[:x.sc] + x.nt + l[x.ec:]
	}

	return strings.Join(lines, "\n")
}
