package lang

import (
	"testing"
)

// structMRO exercises completion against struct types and parameterless stages.
const structMRO = `filetype txt;

struct POINT(
    int x,
    int y,
)

stage NOIN(
    out int z,
    src py     "x",
)

stage NOOUT(
    in  int a,
    src py     "x",
)
`

// With no snapshot, AST-backed contexts yield only their static candidates and
// never panic; symbol-derived candidates are simply absent.
func TestCompleteNilSnapshotStaticOnly(t *testing.T) {
	t.Run("in-type-builtins-only", func(t *testing.T) {
		text, pos := cur("stage S(\n    in  █\n)")
		got := labelSet(docFrom(text).Complete(nil, pos))
		if !got["string"] || !got["int"] {
			t.Errorf("nil-snapshot in-type should still offer builtins; got %v", keys(got))
		}
	})

	t.Run("call-name-empty", func(t *testing.T) {
		text, pos := cur("pipeline X(\n) {\n  call █")
		if got := docFrom(text).Complete(nil, pos); len(got) != 0 {
			t.Errorf("nil-snapshot call-name should be empty; got %v", got)
		}
	})
}

// Struct and user filetype names appear as type candidates.
func TestCompleteOffersStructAndFiletype(t *testing.T) {
	snap := mustCompile(t, structMRO)
	text, pos := cur("stage S(\n    in  █\n)")
	got := labelSet(docFrom(text).Complete(snap, pos))
	if !got["POINT"] {
		t.Errorf("type completion missing struct 'POINT'; got %v", keys(got))
	}
	if !got["txt"] {
		t.Errorf("type completion missing user filetype 'txt'; got %v", keys(got))
	}
}

// A callable with no inputs yields no binding-LHS candidates (and does not panic).
func TestCompleteBindingLHSNoInputs(t *testing.T) {
	snap := mustCompile(t, structMRO) // NOIN has no input params
	text, pos := cur("pipeline X(\n) {\n  call NOIN(\n    █\n  )\n}")
	if got := docFrom(text).Complete(snap, pos); len(got) != 0 {
		t.Errorf("call to parameterless stage should offer no parameters; got %v", got)
	}
}

// An output reference through an unresolved alias degrades to no candidates.
func TestCompleteOutputRefUnresolved(t *testing.T) {
	snap := mustCompile(t, validMRO)
	text, pos := cur("pipeline HELLO_BATCH(\n) {\n  x = NOSUCHCALL.█")
	if got := docFrom(text).Complete(snap, pos); len(got) != 0 {
		t.Errorf("unresolved output ref should offer nothing; got %v", got)
	}
}

// self.<x> outside any pipeline (or for an unknown input) offers nothing.
func TestCompleteSelfRefNoPipeline(t *testing.T) {
	snap := mustCompile(t, validMRO)
	// A self. reference at top level — not inside a pipeline declaration.
	text, pos := cur("stage S(\n    in int x = self.█\n)")
	if got := docFrom(text).Complete(snap, pos); len(got) != 0 {
		t.Errorf("self-ref outside a pipeline should offer nothing; got %v", got)
	}
}

// Completion is suppressed inside comments and string literals.
func TestCompleteSuppressedInCommentAndString(t *testing.T) {
	snap := mustCompile(t, validMRO)

	t.Run("comment", func(t *testing.T) {
		text, pos := cur("stage S(\n    in int x  # stru█\n)")
		if got := docFrom(text).Complete(snap, pos); len(got) != 0 {
			t.Errorf("completion in a comment should be suppressed; got %v", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		text, pos := cur("stage S(\n    src py \"stages/he█\"\n)")
		if got := docFrom(text).Complete(snap, pos); len(got) != 0 {
			t.Errorf("completion inside a string should be suppressed; got %v", got)
		}
	})
}
