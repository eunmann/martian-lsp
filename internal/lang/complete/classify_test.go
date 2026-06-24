package complete

import (
	"strings"
	"testing"
)

// cursor splits a snippet containing a single █ marker into (text, line, col).
func cursor(s string) (string, int, int) {
	before, after, ok := strings.Cut(s, "█")
	if !ok {
		panic("no cursor marker")
	}
	text := before + after
	pre := before
	line := strings.Count(pre, "\n")
	col := len(pre) - (strings.LastIndex(pre, "\n") + 1)

	return text, line, col
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    Kind
		prefix  string
		nameTok string // expected Context.Name (BindingLHS callee / OutputRef base)
		pipe    string // expected Context.Pipeline
	}{
		{name: "top-level empty", src: "█", want: TopLevel},
		{name: "top-level prefix", src: "stru█", want: TopLevel, prefix: "stru"},
		{name: "in type", src: "stage FOO(\n    in  █", want: InOutType},
		{name: "out type prefix", src: "stage F(\n    out file█ result", want: InOutType, prefix: "file"},
		{name: "in string suppress", src: "stage F(\n    out file r \"x█\"", want: None},
		{name: "src lang", src: "stage F(\n    src █", want: SrcLang},
		{name: "src lang prefix", src: "stage F(\n    src p█", want: SrcLang, prefix: "p"},
		{
			name: "output ref", src: "pipeline P(\n) {\n  call H()\n  x = HELLO.█",
			want: OutputRef, nameTok: "HELLO", pipe: "P",
		},
		{
			name: "output ref prefix", src: "pipeline P(\n) {\n  y = HELLO.gre█",
			want: OutputRef, nameTok: "HELLO", prefix: "gre", pipe: "P",
		},
		{
			name: "self ref", src: "pipeline P(\n  in int n,\n) {\n  return (\n    x = self.█",
			want: SelfRef, pipe: "P",
		},
		{
			name: "binding lhs", src: "pipeline P(\n) {\n  call ALIGN(\n    █",
			want: BindingLHS, nameTok: "ALIGN",
		},
		{
			name: "binding lhs aliased uses DecId", src: "pipeline P(\n) {\n  call REF as ALIGN(\n    █",
			want: BindingLHS, nameTok: "REF",
		},
		{
			name: "binding lhs map call", src: "pipeline P(\n) {\n  map call ALIGN(\n    █",
			want: BindingLHS, nameTok: "ALIGN",
		},
		{name: "binding value suppressed", src: "pipeline P(\n) {\n  call A(\n    reads = █", want: None},
		{name: "call name", src: "pipeline P(\n) {\n  call █", want: CallName},
		{name: "call name with modifiers", src: "pipeline P(\n) {\n  local preflight call █", want: CallName},
		{name: "pipeline body stmt", src: "pipeline P(\n) {\n  █", want: PipelineBody},
		{name: "comment suppress", src: "stage F(\n    in int x  # foo█", want: None},
		{name: "hash inside string", src: "stage F(\n    in int x = \"a # b█\"", want: None},
		{name: "multiline in type", src: "stage FOO(\n  in  int reads,\n  in  █", want: InOutType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, line, col := cursor(tc.src)
			got := Classify(text, line, col)
			if got.Kind != tc.want {
				t.Fatalf("Kind = %v, want %v (ctx=%+v)", got.Kind, tc.want, got)
			}
			if tc.prefix != "" && got.Prefix != tc.prefix {
				t.Errorf("Prefix = %q, want %q", got.Prefix, tc.prefix)
			}
			if tc.nameTok != "" && got.Name != tc.nameTok {
				t.Errorf("Name = %q, want %q", got.Name, tc.nameTok)
			}
			if tc.pipe != "" && got.Pipeline != tc.pipe {
				t.Errorf("Pipeline = %q, want %q", got.Pipeline, tc.pipe)
			}
		})
	}
}

func TestClassifyUnresolvedCalleeNoPanic(t *testing.T) {
	// HELLO not declared anywhere; classifier must still return OutputRef cleanly.
	text, line, col := cursor("pipeline P(\n) {\n  x = HELLO.█")
	got := Classify(text, line, col)
	if got.Kind != OutputRef || got.Name != "HELLO" {
		t.Fatalf("got %+v", got)
	}
}
