package lang

import "testing"

const chMRO = `stage A(
    in  int x,
    out int y,
    src py  "stages/a",
)

stage B(
    in  int y,
    out int z,
    src py  "stages/b",
)

pipeline P(
    in  int x,
    out int z,
)
{
    call A(
        x = self.x,
    )
    call B(
        y = A.y,
    )
    return (
        z = B.z,
    )
}
`

func TestCallHierarchyPrepare(t *testing.T) {
	snap := mustCompile(t, chMRO)
	d := docFrom(chMRO)
	items := d.PrepareCallHierarchy(snap, posInDoc(t, chMRO, "stage A(", "A"))
	if len(items) != 1 || items[0].Name != "A" {
		t.Fatalf("prepare on A = %+v, want one item named A", items)
	}
}

func TestCallHierarchyIncoming(t *testing.T) {
	snap := mustCompile(t, chMRO)
	d := docFrom(chMRO)
	item := d.PrepareCallHierarchy(snap, posInDoc(t, chMRO, "stage A(", "A"))[0]

	inc := d.IncomingCalls(snap, item)
	if len(inc) != 1 {
		t.Fatalf("incoming(A) = %d callers, want 1", len(inc))
	}
	if inc[0].From.Name != "P" {
		t.Errorf("incoming(A) caller = %q, want P", inc[0].From.Name)
	}
	if len(inc[0].FromRanges) != 1 {
		t.Errorf("incoming(A) call sites = %d, want 1", len(inc[0].FromRanges))
	}
}

func TestCallHierarchyOutgoing(t *testing.T) {
	snap := mustCompile(t, chMRO)
	d := docFrom(chMRO)

	pItem := d.PrepareCallHierarchy(snap, posInDoc(t, chMRO, "pipeline P(", "P"))[0]
	out := d.OutgoingCalls(snap, pItem)
	names := map[string]bool{}
	for _, c := range out {
		names[c.To.Name] = true
	}
	if !names["A"] || !names["B"] {
		t.Fatalf("outgoing(P) = %v, want A and B", keys(names))
	}

	// A stage has no outgoing calls.
	aItem := d.PrepareCallHierarchy(snap, posInDoc(t, chMRO, "stage A(", "A"))[0]
	if got := d.OutgoingCalls(snap, aItem); len(got) != 0 {
		t.Errorf("outgoing(A) = %d, want 0 (stages don't call)", len(got))
	}
}
