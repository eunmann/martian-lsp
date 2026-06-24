package lang

import (
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestCodeActionCreateUndefinedCallable(t *testing.T) {
	text, pos := cur("pipeline P(\n)\n{\n    call FOO(\n        █\n    )\n    return ()\n}\n")
	d := docFrom(text)
	if ast, _ := d.Compile(); ast == nil {
		t.Fatal("fixture must parse so code actions can run")
	}
	actions := d.CodeActions(nil, protocol.Range{Start: pos, End: pos})
	if len(actions) != 2 {
		t.Fatalf("code actions = %d, want 2 (create stage/pipeline)", len(actions))
	}

	titles := map[string]protocol.CodeAction{}
	for _, a := range actions {
		titles[a.Title] = a
	}
	stage, ok := titles["Create stage FOO"]
	if !ok {
		t.Fatalf("missing 'Create stage FOO'; got %v", keys2(titles))
	}
	if _, ok := titles["Create pipeline FOO"]; !ok {
		t.Errorf("missing 'Create pipeline FOO'")
	}

	edits := stage.Edit.Changes[d.URI]
	if len(edits) != 1 || !strings.Contains(edits[0].NewText, "stage FOO(") {
		t.Errorf("stage action edit = %+v, want one inserting 'stage FOO('", edits)
	}
}

func TestCodeActionNoneForDefinedCallable(t *testing.T) {
	// HELLO is defined in validMRO; no create action should be offered.
	text, pos := cur("stage HELLO(\n    in  string name,\n    out string greeting,\n    src py     \"x\",\n)\n\npipeline P(\n)\n{\n    call HELLO(\n        name = \"x\",\n        █\n    )\n    return ()\n}\n")
	if got := docFrom(text).CodeActions(nil, protocol.Range{Start: pos, End: pos}); len(got) != 0 {
		t.Errorf("defined callable with all args produced %d code actions, want 0", len(got))
	}
}

func TestCodeActionAddMissingArguments(t *testing.T) {
	// HELLO requires input `name`; the call binds nothing -> offer to add it.
	text, pos := cur("stage HELLO(\n    in  string name,\n    out string greeting,\n    src py     \"x\",\n)\n\npipeline P(\n)\n{\n    call HELLO(\n        █\n    )\n    return ()\n}\n")
	actions := docFrom(text).CodeActions(nil, protocol.Range{Start: pos, End: pos})
	if len(actions) != 1 || actions[0].Title != "Add missing arguments" {
		t.Fatalf("want one 'Add missing arguments' action, got %+v", actions)
	}
	edits := actions[0].Edit.Changes[docFrom(text).URI]
	if len(edits) != 1 || !strings.Contains(edits[0].NewText, "name = ") {
		t.Errorf("missing-args edit = %+v, want it to insert 'name = '", edits)
	}
}

func keys2(m map[string]protocol.CodeAction) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
