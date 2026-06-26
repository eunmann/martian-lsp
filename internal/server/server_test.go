// Package server_test drives the language server through its public surface:
// New() + Handler().Handle(). Each test speaks LSP method names and JSON params
// exactly as a real client would, then inspects the wire-shape result. This
// keeps the suite focused on observable behavior rather than internal helpers.
package server_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eunmann/martian-lsp/internal/server"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
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

type notification struct {
	method string
	params json.RawMessage
}

// session is a public-API harness around a Server backed by a real temp dir, so
// workspace discovery (which walks the filesystem) works.
type session struct {
	t      *testing.T
	srv    *server.Server
	h      glsp.Handler
	dir    string
	notes  []notification
	calls  []string // captured outbound client/* requests
	callCh chan string
}

func newSession(t *testing.T) *session {
	t.Helper()
	s := &session{
		t:      t,
		srv:    server.New(),
		dir:    t.TempDir(),
		callCh: make(chan string, 8),
	}
	s.h = s.srv.Handler()

	return s
}

// ctx builds a glsp.Context that records notifications and outbound calls.
func (s *session) ctx(method string, params any) *glsp.Context {
	raw, err := json.Marshal(params)
	if err != nil {
		s.t.Fatalf("marshal params for %s: %v", method, err)
	}

	return &glsp.Context{
		Method: method,
		Params: raw,
		Notify: func(m string, p any) {
			pr, _ := json.Marshal(p)
			s.notes = append(s.notes, notification{method: m, params: pr})
		},
		Call: func(m string, _ any, _ any) {
			s.calls = append(s.calls, m)
			select {
			case s.callCh <- m:
			default:
			}
		},
	}
}

// request dispatches a method and returns its result re-marshalled to JSON. A
// nil result (the server's "not applicable" answer) comes back as "null".
func (s *session) request(method string, params any) json.RawMessage {
	s.t.Helper()
	res, validMethod, _, err := s.h.Handle(s.ctx(method, params))
	if err != nil {
		s.t.Fatalf("%s returned error: %v", method, err)
	}
	if !validMethod {
		s.t.Fatalf("%s not recognized by handler", method)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		s.t.Fatalf("marshal %s result: %v", method, err)
	}

	return raw
}

// notify dispatches a notification (no result expected).
func (s *session) notify(method string, params any) {
	s.t.Helper()
	if _, _, _, err := s.h.Handle(s.ctx(method, params)); err != nil {
		s.t.Fatalf("%s returned error: %v", method, err)
	}
}

func (s *session) uri(name string) string {
	return "file://" + filepath.Join(s.dir, name)
}

// writeAndOpen writes text to disk and sends didOpen, returning the URI.
func (s *session) writeAndOpen(name, text string) string {
	s.t.Helper()
	path := filepath.Join(s.dir, name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		s.t.Fatalf("write %s: %v", name, err)
	}
	uri := "file://" + path
	s.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri": uri, "languageId": "mro", "version": 1, "text": text,
		},
	})

	return uri
}

func (s *session) initialize(opts map[string]any) json.RawMessage {
	s.t.Helper()
	params := map[string]any{
		"processId":    nil,
		"rootUri":      "file://" + s.dir,
		"capabilities": map[string]any{},
	}
	if opts != nil {
		params["initializationOptions"] = opts
	}

	return s.request("initialize", params)
}

// pos finds the LSP position of token on the first line containing lineMark.
func pos(t *testing.T, text, lineMark, token string) map[string]any {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, lineMark) {
			continue
		}
		col := strings.Index(line, token)
		if col < 0 {
			continue
		}

		return map[string]any{"line": i, "character": col}
	}
	t.Fatalf("no line with %q containing %q", lineMark, token)

	return nil
}

// posAfter returns the position one column past token (used to land the cursor
// just inside an open paren, where signature help fires).
func posAfter(t *testing.T, text, lineMark, token string) map[string]any {
	t.Helper()
	p := pos(t, text, lineMark, token)
	col, _ := p["character"].(int)
	p["character"] = col + len(token)

	return p
}

func tdParams(uri string, extra map[string]any) map[string]any {
	m := map[string]any{"textDocument": map[string]any{"uri": uri}}
	maps.Copy(m, extra)

	return m
}

// ---------------------------------------------------------------------------
// initialize / capabilities
// ---------------------------------------------------------------------------

func TestInitializeAdvertisesCapabilities(t *testing.T) {
	s := newSession(t)
	raw := s.initialize(nil)

	var res struct {
		Capabilities struct {
			InlayHintProvider  bool `json:"inlayHintProvider"`
			CompletionProvider struct {
				TriggerCharacters []string `json:"triggerCharacters"`
			} `json:"completionProvider"`
			SignatureHelpProvider struct {
				TriggerCharacters []string `json:"triggerCharacters"`
			} `json:"signatureHelpProvider"`
			RenameProvider struct {
				PrepareProvider bool `json:"prepareProvider"`
			} `json:"renameProvider"`
			SemanticTokensProvider struct {
				Legend struct {
					TokenTypes []string `json:"tokenTypes"`
				} `json:"legend"`
			} `json:"semanticTokensProvider"`
			TextDocumentSync struct {
				Change int `json:"change"`
			} `json:"textDocumentSync"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("decode initialize result: %v\n%s", err, raw)
	}

	c := res.Capabilities
	if !c.InlayHintProvider {
		t.Error("inlayHintProvider not advertised")
	}
	if got := c.CompletionProvider.TriggerCharacters; len(got) != 1 || got[0] != "." {
		t.Errorf("completion trigger chars = %v, want [.]", got)
	}
	if got := c.SignatureHelpProvider.TriggerCharacters; len(got) != 2 || got[0] != "(" || got[1] != "," {
		t.Errorf("signature trigger chars = %v, want [( ,]", got)
	}
	if !c.RenameProvider.PrepareProvider {
		t.Error("rename provider should advertise prepareProvider")
	}
	if len(c.SemanticTokensProvider.Legend.TokenTypes) == 0 {
		t.Error("semantic tokens legend is empty; clients can't decode the stream")
	}
	if c.TextDocumentSync.Change != int(protocol.TextDocumentSyncKindFull) {
		t.Errorf("sync change kind = %d, want %d (Full)", c.TextDocumentSync.Change, protocol.TextDocumentSyncKindFull)
	}
	if res.ServerInfo.Name == "" {
		t.Error("serverInfo.name is empty")
	}
}

// ---------------------------------------------------------------------------
// text sync + diagnostics
// ---------------------------------------------------------------------------

func (s *session) diagnosticsFor(uri string) ([]protocol.Diagnostic, bool) {
	for _, v := range slices.Backward(s.notes) {
		n := v
		if n.method != "textDocument/publishDiagnostics" {
			continue
		}
		var p protocol.PublishDiagnosticsParams
		if err := json.Unmarshal(n.params, &p); err != nil {
			continue
		}
		if p.URI == uri {
			return p.Diagnostics, true
		}
	}

	return nil, false
}

func TestDidOpenPublishesDiagnosticsForBrokenDoc(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	broken := strings.Replace(validMRO, "greeting = HELLO.greeting,", "greeting = HELLO.missing,", 1)
	uri := s.writeAndOpen("pipe.mro", broken)

	diags, ok := s.diagnosticsFor(uri)
	if !ok {
		t.Fatal("no diagnostics published on didOpen")
	}
	if len(diags) == 0 {
		t.Fatal("broken doc should produce at least one diagnostic")
	}
}

func TestDidOpenCleanDocPublishesEmpty(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	uri := s.writeAndOpen("pipe.mro", validMRO)

	diags, ok := s.diagnosticsFor(uri)
	if !ok {
		t.Fatal("clean doc must still publish (to clear stale squiggles)")
	}
	if len(diags) != 0 {
		t.Errorf("clean doc diagnostics = %d, want 0: %+v", len(diags), diags)
	}
}

func TestDidChangeRefreshesDiagnostics(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	uri := s.writeAndOpen("pipe.mro", validMRO)
	if d, _ := s.diagnosticsFor(uri); len(d) != 0 {
		t.Fatalf("precondition: clean doc, got %d diagnostics", len(d))
	}

	broken := strings.Replace(validMRO, "greeting = HELLO.greeting,", "greeting = HELLO.missing,", 1)
	s.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []any{map[string]any{"text": broken}},
	})

	diags, ok := s.diagnosticsFor(uri)
	if !ok || len(diags) == 0 {
		t.Fatalf("breaking edit should surface diagnostics; ok=%v n=%d", ok, len(diags))
	}
}

func TestDidChangeIgnoresRangedDelta(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	broken := strings.Replace(validMRO, "greeting = HELLO.greeting,", "greeting = HELLO.missing,", 1)
	uri := s.writeAndOpen("pipe.mro", broken)
	if d, _ := s.diagnosticsFor(uri); len(d) == 0 {
		t.Fatal("precondition: broken doc should have diagnostics")
	}

	// A ranged (incremental) change carries no whole-document text. The server
	// advertises Full sync and must ignore it — diagnostics stay unchanged.
	s.notes = nil
	s.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{"uri": uri, "version": 3},
		"contentChanges": []any{map[string]any{
			"range": map[string]any{
				"start": map[string]any{"line": 0, "character": 0},
				"end":   map[string]any{"line": 0, "character": 0},
			},
			"text": "filetype fixed;\n",
		}},
	})

	// Force a republish from the *current* buffer (watched-files change re-runs
	// diagnostics for open docs). If the ranged delta had been wrongly applied
	// as a whole-doc replace, the buffer would now be the clean "filetype
	// fixed;" text and report zero diagnostics. It must still be broken.
	s.notes = nil
	s.notify("workspace/didChangeWatchedFiles", map[string]any{
		"changes": []any{map[string]any{"uri": s.uri("other.mro"), "type": 2}},
	})
	diags, ok := s.diagnosticsFor(uri)
	if !ok {
		t.Fatal("expected a republish after the watched-files change")
	}
	if len(diags) == 0 {
		t.Error("ranged-only delta must be ignored; the buffer should still be broken")
	}
}

func TestDidCloseClearsDiagnostics(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	uri := s.writeAndOpen("pipe.mro", validMRO)
	s.notes = nil

	s.notify("textDocument/didClose", tdParams(uri, nil))

	diags, ok := s.diagnosticsFor(uri)
	if !ok {
		t.Fatal("didClose should publish to clear diagnostics")
	}
	if len(diags) != 0 {
		t.Errorf("didClose should publish empty diagnostics, got %d", len(diags))
	}

	// A query on the now-closed document yields null.
	if got := string(s.request("textDocument/documentSymbol", tdParams(uri, nil))); got != "null" {
		t.Errorf("documentSymbol on closed doc = %s, want null", got)
	}
}

func TestWatchedFilesChangeRepublishes(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	uri := s.writeAndOpen("pipe.mro", validMRO)
	s.notes = nil

	s.notify("workspace/didChangeWatchedFiles", map[string]any{
		"changes": []any{map[string]any{"uri": s.uri("other.mro"), "type": 2}},
	})

	if _, ok := s.diagnosticsFor(uri); !ok {
		t.Error("watched-file change should re-publish diagnostics for open docs")
	}
}

// ---------------------------------------------------------------------------
// feature requests on a valid document
// ---------------------------------------------------------------------------

func TestFeatureRequestsOnValidDoc(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	uri := s.writeAndOpen("pipe.mro", validMRO)

	helloDef := pos(t, validMRO, "stage HELLO(", "HELLO")
	helloCall := pos(t, validMRO, "call HELLO(", "HELLO")

	notNull := func(method string, params any) json.RawMessage {
		raw := s.request(method, params)
		if string(raw) == "null" {
			t.Errorf("%s returned null, want a result", method)
		}

		return raw
	}

	notNull("textDocument/documentSymbol", tdParams(uri, nil))
	hov := notNull("textDocument/hover", tdParams(uri, map[string]any{"position": helloDef}))
	if !strings.Contains(string(hov), "HELLO") {
		t.Errorf("hover did not mention HELLO: %s", hov)
	}
	notNull("textDocument/definition", tdParams(uri, map[string]any{"position": helloCall}))
	notNull("textDocument/references", tdParams(uri, map[string]any{
		"position": helloDef, "context": map[string]any{"includeDeclaration": true},
	}))
	notNull("textDocument/documentHighlight", tdParams(uri, map[string]any{"position": helloDef}))
	notNull("textDocument/prepareRename", tdParams(uri, map[string]any{"position": helloDef}))
	notNull("textDocument/semanticTokens/full", tdParams(uri, nil))
	notNull("textDocument/completion", tdParams(uri, map[string]any{
		"position": map[string]any{"line": 0, "character": 0},
	}))
	notNull("textDocument/signatureHelp", tdParams(uri, map[string]any{
		"position": posAfter(t, validMRO, "call HELLO(", "("),
	}))

	// Rename produces a workspace edit touching HELLO.
	re := notNull("textDocument/rename", tdParams(uri, map[string]any{
		"position": helloDef, "newName": "GOODBYE",
	}))
	if !strings.Contains(string(re), "GOODBYE") {
		t.Errorf("rename edit missing new name: %s", re)
	}

	// documentLink / typeDefinition / formatting may legitimately be empty for
	// this fixture; assert only that they don't error (already covered by request).
	s.request("textDocument/documentLink", tdParams(uri, nil))
	s.request("textDocument/formatting", tdParams(uri, map[string]any{
		"options": map[string]any{"tabSize": 4, "insertSpaces": true},
	}))
	s.request("textDocument/typeDefinition", tdParams(uri, map[string]any{
		"position": pos(t, validMRO, "in  string name", "name"),
	}))
}

func TestCallHierarchyRoundTrip(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	uri := s.writeAndOpen("pipe.mro", validMRO)

	prep := s.request("textDocument/prepareCallHierarchy", tdParams(uri, map[string]any{
		"position": pos(t, validMRO, "stage HELLO(", "HELLO"),
	}))
	var items []protocol.CallHierarchyItem
	if err := json.Unmarshal(prep, &items); err != nil || len(items) == 0 {
		t.Fatalf("prepareCallHierarchy on HELLO: items=%d err=%v", len(items), err)
	}

	inc := s.request("callHierarchy/incomingCalls", map[string]any{"item": items[0]})
	var incoming []protocol.CallHierarchyIncomingCall
	if err := json.Unmarshal(inc, &incoming); err != nil {
		t.Fatalf("incomingCalls decode: %v", err)
	}
	if len(incoming) == 0 {
		t.Error("HELLO is called by HELLO_BATCH; expected an incoming call")
	}

	// Outgoing from the pipeline should include HELLO.
	prepPipe := s.request("textDocument/prepareCallHierarchy", tdParams(uri, map[string]any{
		"position": pos(t, validMRO, "pipeline HELLO_BATCH(", "HELLO_BATCH"),
	}))
	var pItems []protocol.CallHierarchyItem
	if err := json.Unmarshal(prepPipe, &pItems); err != nil || len(pItems) == 0 {
		t.Fatalf("prepareCallHierarchy on HELLO_BATCH: items=%d err=%v", len(pItems), err)
	}
	out := s.request("callHierarchy/outgoingCalls", map[string]any{"item": pItems[0]})
	var outgoing []protocol.CallHierarchyOutgoingCall
	if err := json.Unmarshal(out, &outgoing); err != nil {
		t.Fatalf("outgoingCalls decode: %v", err)
	}
	if len(outgoing) == 0 {
		t.Error("HELLO_BATCH calls HELLO; expected an outgoing call")
	}
}

func TestInlayHintsViaWrappedHandler(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	uri := s.writeAndOpen("pipe.mro", validMRO)

	raw := s.request("textDocument/inlayHint", tdParams(uri, map[string]any{
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": 100, "character": 0},
		},
	}))
	if string(raw) == "null" {
		t.Error("inlayHint returned null for a doc with call bindings")
	}
}

func TestInlayHintMalformedParams(t *testing.T) {
	s := newSession(t)
	ctx := &glsp.Context{Method: "textDocument/inlayHint", Params: json.RawMessage(`{bad`)}
	_, validMethod, _, err := s.h.Handle(ctx)
	if !validMethod {
		t.Error("malformed inlayHint should still be marked handled")
	}
	if err == nil {
		t.Error("malformed inlayHint params should error")
	}
}

func TestWorkspaceSymbolFindsCallables(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	s.writeAndOpen("pipe.mro", validMRO)

	raw := s.request("workspace/symbol", map[string]any{"query": "HELLO"})
	var syms []protocol.SymbolInformation
	if err := json.Unmarshal(raw, &syms); err != nil {
		t.Fatalf("decode workspace symbols: %v", err)
	}
	found := false
	for _, sym := range syms {
		if sym.Name == "HELLO" || sym.Name == "HELLO_BATCH" {
			found = true
		}
	}
	if !found {
		t.Errorf("workspace/symbol query HELLO found none of HELLO/HELLO_BATCH: %v", syms)
	}
}

// ---------------------------------------------------------------------------
// feature requests on an unknown document → null
// ---------------------------------------------------------------------------

func TestFeatureRequestsUnknownDocReturnNull(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	missing := s.uri("nope.mro")
	at := map[string]any{"position": map[string]any{"line": 0, "character": 0}}

	methods := []struct {
		name   string
		params any
	}{
		{"textDocument/documentSymbol", tdParams(missing, nil)},
		{"textDocument/hover", tdParams(missing, at)},
		{"textDocument/definition", tdParams(missing, at)},
		{"textDocument/references", tdParams(missing, map[string]any{
			"position": at["position"], "context": map[string]any{"includeDeclaration": true},
		})},
		{"textDocument/rename", tdParams(missing, map[string]any{"position": at["position"], "newName": "X"})},
		{"textDocument/codeAction", tdParams(missing, map[string]any{
			"range":   map[string]any{"start": at["position"], "end": at["position"]},
			"context": map[string]any{"diagnostics": []any{}},
		})},
		{"textDocument/formatting", tdParams(missing, map[string]any{"options": map[string]any{"tabSize": 4, "insertSpaces": true}})},
		{"textDocument/semanticTokens/full", tdParams(missing, nil)},
		{"textDocument/documentHighlight", tdParams(missing, at)},
		{"textDocument/prepareRename", tdParams(missing, at)},
		{"textDocument/typeDefinition", tdParams(missing, at)},
		{"textDocument/documentLink", tdParams(missing, nil)},
		{"textDocument/signatureHelp", tdParams(missing, at)},
		{"textDocument/prepareCallHierarchy", tdParams(missing, at)},
		{"textDocument/completion", tdParams(missing, at)},
	}
	for _, m := range methods {
		t.Run(m.name, func(t *testing.T) {
			if got := string(s.request(m.name, m.params)); got != "null" {
				t.Errorf("%s on unknown doc = %s, want null", m.name, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// initializationOptions.mroPaths affects include resolution
// ---------------------------------------------------------------------------

func TestMROPathsResolveCrossDirInclude(t *testing.T) {
	s := newSession(t)
	libDir := filepath.Join(s.dir, "lib")
	if err := os.MkdirAll(libDir, 0o750); err != nil {
		t.Fatalf("mkdir lib: %v", err)
	}
	// A stage living only under lib/, referenced by bare name from the include.
	if err := os.WriteFile(filepath.Join(libDir, "dna.mro"),
		[]byte("stage ALIGN(\n    in  string ref,\n    out string bam,\n    src py \"x\",\n)\n"), 0o600); err != nil {
		t.Fatalf("write lib/dna.mro: %v", err)
	}
	s.initialize(map[string]any{"mroPaths": []any{libDir}})

	// main.mro includes dna.mro by bare name — only resolvable via mroPaths.
	main := "@include \"dna.mro\"\n\npipeline P(\n    out string bam,\n)\n{\n    call ALIGN(\n        ref = \"r\",\n    )\n    return (\n        bam = ALIGN.bam,\n    )\n}\n"
	uri := s.writeAndOpen("main.mro", main)

	diags, ok := s.diagnosticsFor(uri)
	if !ok {
		t.Fatal("no diagnostics published for main.mro")
	}
	// With mroPaths set, the include resolves and the pipeline type-checks clean.
	if len(diags) != 0 {
		t.Errorf("mroPaths should resolve the include; got diagnostics: %+v", diags)
	}
}

// $MROPATH in the environment resolves includes with no client configuration.
func TestMROPathFromEnvironment(t *testing.T) {
	s := newSession(t)
	libDir := t.TempDir() // a directory outside the workspace
	if err := os.WriteFile(filepath.Join(libDir, "dna.mro"),
		[]byte("stage ALIGN(\n    in  string ref,\n    out string bam,\n    src py \"x\",\n)\n"), 0o600); err != nil {
		t.Fatalf("write lib dna.mro: %v", err)
	}
	t.Setenv("MROPATH", libDir)
	s.initialize(nil) // initialize reads $MROPATH

	main := "@include \"dna.mro\"\n\npipeline P(\n    out string bam,\n)\n{\n    call ALIGN(\n        ref = \"r\",\n    )\n    return (\n        bam = ALIGN.bam,\n    )\n}\n"
	uri := s.writeAndOpen("main.mro", main)

	diags, ok := s.diagnosticsFor(uri)
	if !ok {
		t.Fatal("no diagnostics published")
	}
	if len(diags) != 0 {
		t.Errorf("$MROPATH should resolve the include; got: %+v", diags)
	}
}

// A relative martian.mroPaths entry resolves against the workspace root.
func TestMROPathsRelativeResolvedAgainstRoot(t *testing.T) {
	s := newSession(t)
	libDir := filepath.Join(s.dir, "lib")
	if err := os.MkdirAll(libDir, 0o750); err != nil {
		t.Fatalf("mkdir lib: %v", err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "dna.mro"),
		[]byte("stage ALIGN(\n    in  string ref,\n    out string bam,\n    src py \"x\",\n)\n"), 0o600); err != nil {
		t.Fatalf("write lib/dna.mro: %v", err)
	}
	// Configure a *relative* path; the server must resolve it against rootUri.
	s.initialize(map[string]any{"mroPaths": []any{"lib"}})

	main := "@include \"dna.mro\"\n\npipeline P(\n    out string bam,\n)\n{\n    call ALIGN(\n        ref = \"r\",\n    )\n    return (\n        bam = ALIGN.bam,\n    )\n}\n"
	uri := s.writeAndOpen("main.mro", main)

	diags, ok := s.diagnosticsFor(uri)
	if !ok {
		t.Fatal("no diagnostics published")
	}
	if len(diags) != 0 {
		t.Errorf("relative mroPaths should resolve against the workspace root; got: %+v", diags)
	}
}

// ---------------------------------------------------------------------------
// lifecycle: initialized registers a file watcher iff the client supports it
// ---------------------------------------------------------------------------

func TestInitializedRegistersWatcherWhenSupported(t *testing.T) {
	s := newSession(t)
	s.request("initialize", map[string]any{
		"processId": nil,
		"rootUri":   "file://" + s.dir,
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"didChangeWatchedFiles": map[string]any{"dynamicRegistration": true},
			},
		},
	})
	s.notify("initialized", map[string]any{})

	select {
	case m := <-s.callCh:
		if m != "client/registerCapability" {
			t.Errorf("registered via %q, want client/registerCapability", m)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected a client/registerCapability call for watcher registration")
	}
}

func TestInitializedNoWatcherWhenUnsupported(t *testing.T) {
	s := newSession(t)
	s.initialize(nil) // empty capabilities → no dynamic registration
	s.notify("initialized", map[string]any{})

	select {
	case m := <-s.callCh:
		t.Errorf("unexpected outbound call %q when client lacks watcher support", m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestShutdownAndSetTrace(t *testing.T) {
	s := newSession(t)
	s.initialize(nil)
	s.notify("$/setTrace", map[string]any{"value": "verbose"})
	if got := string(s.request("shutdown", nil)); got != "null" {
		t.Errorf("shutdown result = %s, want null", got)
	}
}
