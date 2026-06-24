package lang

import (
	"strings"
	"testing"

	"github.com/martian-lang/martian/martian/syntax"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func mustCompile(t *testing.T, src string) *syntax.Ast {
	t.Helper()
	ast, _ := docFrom(src).Compile()
	if ast == nil {
		t.Fatal("fixture failed to parse")
	}

	return ast
}

// cur strips a single █ marker and returns (text, position).
func cur(s string) (string, protocol.Position) {
	before, after, _ := strings.Cut(s, "█")
	text := before + after
	pre := before
	line := strings.Count(pre, "\n")
	col := len(pre) - (strings.LastIndex(pre, "\n") + 1)

	return text, protocol.Position{Line: uint32(line), Character: uint32(col)}
}

func labelSet(items []protocol.CompletionItem) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it.Label] = true
	}

	return m
}

func TestCompleteContexts(t *testing.T) {
	snap := mustCompile(t, validMRO) // defines stage HELLO(in name, out greeting), filetype txt

	cases := []struct {
		name string
		src  string
		want []string // labels that must be present
	}{
		{"top-level", "█", []string{"stage", "pipeline", "filetype", "call"}},
		{"in-type", "stage S(\n    in  █\n)", []string{"string", "int", "file", "txt"}},
		{"src-lang", "stage S(\n    src █\n)", []string{"py", "exec", "comp"}},
		{"call-name", "pipeline X(\n) {\n  call █", []string{"HELLO"}},
		{"binding-lhs", "pipeline X(\n) {\n  call HELLO(\n    █\n  )\n}", []string{"name"}},
		{"output-ref", "pipeline HELLO_BATCH(\n) {\n  x = HELLO.█", []string{"greeting"}},
		{"self-ref", "pipeline HELLO_BATCH(\n  in string name,\n) {\n  return (\n    g = self.█", []string{"name"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, pos := cur(tc.src)
			got := labelSet(docFrom(text).Complete(snap, pos))
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("missing %q; got %v", w, keys(got))
				}
			}
		})
	}
}

func TestSnippetCompletion(t *testing.T) {
	text, pos := cur("stage█")
	items := docFrom(text).Complete(nil, pos)

	var found bool
	for _, it := range items {
		if it.Label != "stage" {
			continue
		}
		found = true
		if it.InsertTextFormat == nil || *it.InsertTextFormat != protocol.InsertTextFormatSnippet {
			t.Errorf("stage completion is not a snippet")
		}
		te, ok := it.TextEdit.(protocol.TextEdit)
		if !ok || !strings.Contains(te.NewText, "stage ${1") {
			t.Errorf("stage snippet body = %v, want a templated skeleton", it.TextEdit)
		}
	}
	if !found {
		t.Fatal("no 'stage' snippet completion offered at top level")
	}
}

func TestCompletePrefixFilter(t *testing.T) {
	snap := mustCompile(t, validMRO)
	text, pos := cur("stage S(\n    in  str█\n)")
	got := labelSet(docFrom(text).Complete(snap, pos))
	if !got["string"] {
		t.Errorf("want 'string' for prefix 'str'; got %v", keys(got))
	}
	if got["int"] {
		t.Errorf("prefix 'str' should filter out 'int'; got %v", keys(got))
	}
}

// The robustness guarantee: completion works on a buffer that no longer parses,
// using the last-good snapshot for symbols.
func TestCompleteOnBrokenBuffer(t *testing.T) {
	snap := mustCompile(t, validMRO) // last-good symbols

	// Corrupt the buffer so it can't parse, but keep a HELLO. reference.
	broken := strings.Replace(validMRO, "pipeline HELLO_BATCH(", "pipeline HELLO_BATCH((( ", 1)
	if ast, _ := docFrom(broken).Compile(); ast != nil {
		t.Fatal("expected broken buffer to fail parsing")
	}

	// Place the cursor right after "HELLO." in the (still-present) return wiring.
	idx := strings.Index(broken, "HELLO.greeting,\n    )") // the return-site reference
	pre := broken[:idx+len("HELLO.")]
	line := strings.Count(pre, "\n")
	col := len(pre) - (strings.LastIndex(pre, "\n") + 1)
	pos := protocol.Position{Line: uint32(line), Character: uint32(col)}

	got := labelSet(docFrom(broken).Complete(snap, pos))
	if !got["greeting"] {
		t.Errorf("broken-buffer completion missing 'greeting'; got %v", keys(got))
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
