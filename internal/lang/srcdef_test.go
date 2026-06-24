package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestSrcDefinitionJumpsToFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stages", "hello.py"), []byte("def main(args, outs):\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mro := "stage HELLO(\n    src py \"stages/hello\",\n)\n"
	mroPath := filepath.Join(dir, "p.mro")
	d := &Document{URI: PathToURI(mroPath), Path: mroPath, Text: mro}

	line := strings.Split(mro, "\n")[1] // `    src py "stages/hello",`
	col := strings.Index(line, "stages/hello") + 2
	loc, ok := d.Definition(protocol.Position{Line: 1, Character: uint32(col)})
	if !ok {
		t.Fatal("expected src go-to-definition to resolve")
	}
	if !strings.HasSuffix(URIToPath(loc.URI), filepath.Join("stages", "hello.py")) {
		t.Errorf("src definition = %q, want .../stages/hello.py", loc.URI)
	}
}

func TestSrcDefinitionOnlyOnPath(t *testing.T) {
	mro := "stage HELLO(\n    src py \"stages/hello\",\n)\n"
	d := &Document{URI: "file:///tmp/p.mro", Path: "/tmp/p.mro", Text: mro}
	// Cursor on the `src` keyword (column 4), not the path -> no src definition.
	if _, ok := d.srcDefinition(protocol.Position{Line: 1, Character: 4}); ok {
		t.Error("src definition should not trigger on the 'src' keyword")
	}
}
