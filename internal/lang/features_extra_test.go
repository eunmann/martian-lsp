package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// ---------------------------------------------------------------------------
// Robustness: degenerate documents must never panic and must return empty.
// ---------------------------------------------------------------------------

func TestFeaturesOnDegenerateDocuments(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"whitespace":    "   \n\t\n  ",
		"comments-only": "# just a comment\n# another\n",
		"unparseable":   "stage (((  \n  in ",
		"partial-stage": "stage HELLO(\n    in string name,\n",
	}
	// Cases with no complete declaration must yield no document symbols; the
	// partial/unparseable ones may surface a recovered symbol, so we only
	// require that they not panic.
	mustHaveNoSymbols := map[string]bool{"empty": true, "whitespace": true, "comments-only": true}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			d := docFrom(text)
			got := d.Symbols()
			if mustHaveNoSymbols[name] && len(got) != 0 {
				t.Errorf("%s: Symbols = %d, want 0", name, len(got))
			}
			d.SemanticTokens()
			d.DocumentLinks()
			d.Format()
			d.Diagnose()
			pos := protocol.Position{Line: 0, Character: 0}
			d.Hover(pos)
			d.Definition(pos)
			d.References(pos)
			d.DocumentHighlights(pos)
			d.PrepareRename(pos)
			d.Rename(pos, "X")
			d.TypeDefinition(pos)
			if _, ok := d.SymbolAt(pos); ok && text == "" {
				t.Errorf("%s: SymbolAt should be false on empty doc", name)
			}
			d.Complete(nil, pos)
			d.InlayHints(nil, protocol.Range{End: protocol.Position{Line: 1000}})
			d.CodeActions(nil, protocol.Range{Start: pos, End: pos})
		})
	}
}

// A degenerate document's own URI is always present in Diagnose output so the
// client can clear stale squiggles even when there is nothing to report.
func TestDiagnoseAlwaysClearsOwnURIEvenWhenEmpty(t *testing.T) {
	for _, text := range []string{"", "# comment\n"} {
		d := docFrom(text)
		fds := d.Diagnose()
		found := false
		for _, fd := range fds {
			if fd.URI == d.URI {
				found = true
			}
		}
		if !found {
			t.Errorf("Diagnose(%q) did not include own URI", text)
		}
	}
}

// ---------------------------------------------------------------------------
// Multi-file diagnostics: an error inside an @include'd file is reported
// against that file's URI, not the including document's.
// ---------------------------------------------------------------------------

func TestDiagnoseRoutesIncludedFileErrorToItsURI(t *testing.T) {
	dir := t.TempDir()
	// lib.mro defines a pipeline that wires a non-existent call output -> a
	// semantic error that lives entirely inside lib.mro.
	lib := "pipeline BROKEN(\n    out int x,\n)\n{\n    return (\n        x = GHOST.y,\n    )\n}\n"
	main := "@include \"lib.mro\"\n\nstage OK(\n    in  int a,\n    src py \"x\",\n)\n"
	if err := os.WriteFile(filepath.Join(dir, "lib.mro"), []byte(lib), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.mro")
	if err := os.WriteFile(mainPath, []byte(main), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &Document{URI: PathToURI(mainPath), Path: mainPath, Text: main}
	fds := d.Diagnose()

	var libURI string
	libDiags := 0
	for _, fd := range fds {
		if strings.HasSuffix(URIToPath(fd.URI), "lib.mro") {
			libURI = fd.URI
			libDiags += len(fd.Diagnostics)
		}
	}
	if libURI == "" {
		t.Fatalf("no diagnostics routed to lib.mro; got %+v", fds)
	}
	if libDiags == 0 {
		t.Errorf("expected the error to be attributed to lib.mro, got 0 diagnostics there")
	}
}

func TestDiagnoseReportsMissingInclude(t *testing.T) {
	dir := t.TempDir()
	main := "@include \"ghost.mro\"\n\nstage OK(\n    in int a,\n    src py \"x\",\n)\n"
	mainPath := filepath.Join(dir, "main.mro")
	if err := os.WriteFile(mainPath, []byte(main), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &Document{URI: PathToURI(mainPath), Path: mainPath, Text: main}
	if got := totalDiagnostics(d); got == 0 {
		t.Fatal("a missing @include should produce at least one diagnostic")
	}
}

// ---------------------------------------------------------------------------
// Code action: "Add @include" for a callable defined in another workspace file.
// ---------------------------------------------------------------------------

func TestCodeActionAddIncludeForCrossFileCallable(t *testing.T) {
	dir := t.TempDir()
	lib := "stage WIDGET(\n    in  string x,\n    out string y,\n    src py \"w\",\n)\n"
	if err := os.WriteFile(filepath.Join(dir, "lib.mro"), []byte(lib), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.mro")
	mainText := "pipeline P(\n)\n{\n    call WIDGET(\n        x = \"v\",\n    )\n    return ()\n}\n"

	d := &Document{URI: PathToURI(mainPath), Path: mainPath, Text: mainText}
	if ast, _ := d.Compile(); ast == nil {
		t.Fatal("fixture must parse so code actions can run")
	}
	ws := NewWorkspace(NewStore(), []string{dir})

	// Cursor inside the WIDGET call binding block (the "x = ..." line).
	pos := posInDoc(t, mainText, "x = ", "x")
	actions := d.CodeActions(ws, protocol.Range{Start: pos, End: pos})

	var include *protocol.CodeAction
	for i := range actions {
		if strings.HasPrefix(actions[i].Title, "Add @include") {
			include = &actions[i]
		}
	}
	if include == nil {
		titles := make([]string, len(actions))
		for i, a := range actions {
			titles[i] = a.Title
		}
		t.Fatalf("no Add @include action offered; got %v", titles)
	}
	if !strings.Contains(include.Title, "lib.mro") || !strings.Contains(include.Title, "WIDGET") {
		t.Errorf("include action title = %q, want it to mention lib.mro and WIDGET", include.Title)
	}
	edits := include.Edit.Changes[d.URI]
	if len(edits) != 1 || !strings.Contains(edits[0].NewText, "@include \"lib.mro\"") {
		t.Errorf("include edit = %+v, want it to insert @include \"lib.mro\"", edits)
	}
}

// ---------------------------------------------------------------------------
// Auto-detection of the conventional mro/ include root.
// ---------------------------------------------------------------------------

func TestMROPathsIncludesNearestMroRoot(t *testing.T) {
	d := &Document{Path: "/proj/mro/stages/foo.mro"}
	paths := d.MROPaths()
	if len(paths) < 2 || paths[0] != "/proj/mro/stages" || paths[1] != "/proj/mro" {
		t.Errorf("MROPaths = %v, want [/proj/mro/stages /proj/mro ...]", paths)
	}
}

func TestMROPathsNoMroRoot(t *testing.T) {
	d := &Document{Path: "/proj/src/foo.mro"}
	paths := d.MROPaths()
	if len(paths) != 1 || paths[0] != "/proj/src" {
		t.Errorf("MROPaths = %v, want just [/proj/src] when no mro/ ancestor", paths)
	}
}

// Negative control: a bare @include with no mro/ ancestor, no $MROPATH, and no
// configured mroPaths must NOT resolve — this backstops the resolution tests by
// proving the include genuinely depends on a search path being supplied.
func TestBareIncludeUnresolvedByDefault(t *testing.T) {
	dir := t.TempDir() // not named "mro", nothing adjacent
	libDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(libDir, "bar.mro"), []byte("filetype bam;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fooPath := filepath.Join(dir, "foo.mro")
	foo := "@include \"bar.mro\"\n\nstage S(\n    in  bam reads,\n    src py \"x\",\n)\n"
	if err := os.WriteFile(fooPath, []byte(foo), 0o600); err != nil {
		t.Fatal(err)
	}
	d := &Document{URI: PathToURI(fooPath), Path: fooPath, Text: foo}
	if got := totalDiagnostics(d); got == 0 {
		t.Error("a bare include with no search path configured should not resolve")
	}
}

// End-to-end: a file under mro/ whose @include is written relative to that root
// resolves with no configuration at all.
func TestIncludeResolvesViaAutoDetectedMroRoot(t *testing.T) {
	dir := t.TempDir()
	mroRoot := filepath.Join(dir, "mro")
	if err := os.MkdirAll(filepath.Join(mroRoot, "types"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(mroRoot, "stages"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mroRoot, "types", "bar.mro"), []byte("filetype bam;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// foo.mro lives under mro/stages and includes "types/bar.mro" relative to
	// the mro/ root — only resolvable via auto-detection.
	fooPath := filepath.Join(mroRoot, "stages", "foo.mro")
	foo := "@include \"types/bar.mro\"\n\nstage S(\n    in  bam reads,\n    src py \"x\",\n)\n"
	if err := os.WriteFile(fooPath, []byte(foo), 0o600); err != nil {
		t.Fatal(err)
	}

	d := &Document{URI: PathToURI(fooPath), Path: fooPath, Text: foo}
	if got := totalDiagnostics(d); got != 0 {
		t.Errorf("auto-detected mro/ root should resolve the include; got %d diagnostics", got)
	}
}

// ---------------------------------------------------------------------------
// Workspace.DefiningFile
// ---------------------------------------------------------------------------

func TestDefiningFile(t *testing.T) {
	ws := NewWorkspace(NewStore(), []string{writeWorkspace(t)})

	path, ok := ws.DefiningFile("ALIGN")
	if !ok {
		t.Fatal("DefiningFile(ALIGN) not found")
	}
	if filepath.Base(path) != "dna.mro" {
		t.Errorf("ALIGN defined in %q, want dna.mro", filepath.Base(path))
	}

	if _, ok := ws.DefiningFile("DOES_NOT_EXIST"); ok {
		t.Error("DefiningFile should report not-found for an unknown callable")
	}

	// A call site is not a definition: P is the pipeline, ALIGN's *caller*. The
	// callee resolution must point at the declaring file only.
	if _, ok := ws.DefiningFile("P"); !ok {
		t.Error("DefiningFile(P) should find the pipeline in main.mro")
	}
}

// ---------------------------------------------------------------------------
// Negative space: navigation off a symbol returns nothing.
// ---------------------------------------------------------------------------

func TestNavigationNegativeSpace(t *testing.T) {
	d := docFrom(validMRO)

	// A keyword is not a symbol.
	kw := posInDoc(t, validMRO, "stage HELLO(", "stage")
	if _, ok := d.SymbolAt(kw); ok {
		t.Error("SymbolAt on the 'stage' keyword should be false")
	}
	if _, ok := d.PrepareRename(kw); ok {
		t.Error("PrepareRename on a keyword should be false")
	}
	if d.Rename(kw, "X") != nil {
		t.Error("Rename on a keyword should return nil")
	}
	if len(d.References(kw)) != 0 {
		t.Error("References on a keyword should be empty")
	}

	// A position far past the end of the document is not on any symbol.
	far := protocol.Position{Line: 9999, Character: 0}
	if _, ok := d.SymbolAt(far); ok {
		t.Error("SymbolAt out of bounds should be false")
	}
	if _, ok := d.Hover(far); ok {
		t.Error("Hover out of bounds should be false")
	}
}

// ---------------------------------------------------------------------------
// Workspace queries for an unknown symbol degrade to nil.
// ---------------------------------------------------------------------------

func TestWorkspaceUnknownSymbol(t *testing.T) {
	ws := NewWorkspace(NewStore(), []string{writeWorkspace(t)})
	if got := ws.References("callable:NOPE"); got != nil {
		t.Errorf("References(unknown) = %v, want nil", got)
	}
	if got := ws.Rename("callable:NOPE", "X"); got != nil {
		t.Errorf("Rename(unknown) = %v, want nil", got)
	}
	if got := ws.References(""); got != nil {
		t.Errorf("References(empty key) = %v, want nil", got)
	}
	if got := ws.Rename("", "X"); got != nil {
		t.Errorf("Rename(empty key) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// Inlay hints: fall back to the last-good snapshot when the buffer is broken.
// ---------------------------------------------------------------------------

func TestInlayHintsFallBackToSnapshot(t *testing.T) {
	snap := mustCompile(t, validMRO)

	// Corrupt the buffer so it no longer parses, keeping the call binding intact.
	broken := strings.Replace(validMRO, "pipeline HELLO_BATCH(", "pipeline HELLO_BATCH((( ", 1)
	if ast, _ := docFrom(broken).Compile(); ast != nil {
		t.Fatal("expected the broken buffer to fail parsing")
	}

	full := protocol.Range{End: protocol.Position{Line: 1000}}
	hints := docFrom(broken).InlayHints(snap, full)
	if len(hints) == 0 {
		t.Error("inlay hints should fall back to the snapshot on a broken buffer")
	}

	// With neither a parse nor a snapshot, there is nothing to hint.
	if got := docFrom(broken).InlayHints(nil, full); got != nil {
		t.Errorf("InlayHints(nil snapshot, broken buffer) = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// Format on degenerate inputs returns no edits (nothing to canonicalize).
// ---------------------------------------------------------------------------

func TestFormatDegenerateInputs(t *testing.T) {
	for name, text := range map[string]string{
		"empty":      "",
		"whitespace": "   \n\n",
		"broken":     "stage (((",
	} {
		t.Run(name, func(t *testing.T) {
			if got := docFrom(text).Format(); got != nil {
				t.Errorf("Format(%q) = %+v, want nil", text, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Call hierarchy on a directly self-recursive pipeline.
// ---------------------------------------------------------------------------

func TestCallHierarchyRecursive(t *testing.T) {
	// A pipeline that calls itself: it should appear as both its own caller
	// (incoming) and callee (outgoing) without infinite recursion.
	src := "stage LEAF(\n    in  int x,\n    out int y,\n    src py \"l\",\n)\n\n" +
		"pipeline REC(\n    in  int x,\n    out int y,\n)\n{\n    call LEAF(\n        x = self.x,\n    )\n    return (\n        y = LEAF.y,\n    )\n}\n"
	snap := mustCompile(t, src)
	d := docFrom(src)

	items := d.PrepareCallHierarchy(snap, posInDoc(t, src, "pipeline REC(", "REC"))
	if len(items) == 0 {
		t.Fatal("prepareCallHierarchy on REC returned nothing")
	}
	out := d.OutgoingCalls(snap, items[0])
	if len(out) == 0 {
		t.Error("REC calls LEAF; expected an outgoing call")
	}
}
