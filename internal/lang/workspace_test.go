package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeWorkspace lays down a two-file project: dna.mro declares stage ALIGN,
// main.mro @includes it and calls ALIGN.
func writeWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dna := "filetype bam;\n\nstage ALIGN(\n    in  bam reads,\n    out bam aligned,\n    src py  \"stages/align\",\n)\n"
	main := "@include \"dna.mro\"\n\npipeline P(\n    in  bam reads,\n    out bam aligned,\n)\n{\n    call ALIGN(\n        reads = self.reads,\n    )\n    return (\n        aligned = ALIGN.aligned,\n    )\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "dna.mro"), []byte(dna), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.mro"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestWorkspaceSymbols(t *testing.T) {
	ws := NewWorkspace(NewStore(), []string{writeWorkspace(t)})
	names := map[string]bool{}
	for _, s := range ws.Symbols("") {
		names[s.Name] = true
	}
	if !names["ALIGN"] || !names["P"] {
		t.Fatalf("workspace symbols = %v, want ALIGN and P", keys(names))
	}

	// Query filters.
	got := ws.Symbols("ALIGN")
	for _, s := range got {
		if !strings.Contains(strings.ToLower(s.Name), "align") {
			t.Errorf("query 'ALIGN' returned unrelated symbol %q", s.Name)
		}
	}
}

func TestWorkspaceReferencesCrossFile(t *testing.T) {
	ws := NewWorkspace(NewStore(), []string{writeWorkspace(t)})
	refs := ws.References("callable:ALIGN")
	// dna.mro: stage ALIGN decl (1); main.mro: call ALIGN + ALIGN.aligned (2).
	if len(refs) != 3 {
		t.Fatalf("cross-file references = %d, want 3: %+v", len(refs), refs)
	}
	files := map[string]bool{}
	for _, r := range refs {
		files[filepath.Base(URIToPath(r.URI))] = true
	}
	if !files["dna.mro"] || !files["main.mro"] {
		t.Errorf("references span %v, want both dna.mro and main.mro", keys(files))
	}
}

func TestWorkspaceRenameCrossFile(t *testing.T) {
	ws := NewWorkspace(NewStore(), []string{writeWorkspace(t)})
	we := ws.Rename("callable:ALIGN", "ALN")
	if we == nil {
		t.Fatal("expected a workspace edit")
	}
	if len(we.Changes) != 2 {
		t.Fatalf("rename touched %d files, want 2", len(we.Changes))
	}
	total := 0
	for _, edits := range we.Changes {
		for _, e := range edits {
			if e.NewText != "ALN" {
				t.Errorf("edit NewText = %q, want ALN", e.NewText)
			}
			total++
		}
	}
	if total != 3 {
		t.Errorf("total rename edits = %d, want 3", total)
	}
}
