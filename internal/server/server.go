// Package server wires Martian language features to the LSP transport via glsp.
// It owns the document store and translates LSP requests into calls on the
// language-intelligence layer (internal/lang).
package server

import (
	"github.com/eunmann/martian-lsp/internal/lang"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

const lsName = "martian-lsp"

// Version is the advertised server version (overridable at build time).
//
//nolint:gochecknoglobals // set via -ldflags at build time
var Version = "0.0.1"

// Server holds the mutable language-server state.
type Server struct {
	docs       *lang.Store
	ws         *lang.Workspace
	watchFiles bool // client supports dynamic file-watcher registration
	handler    protocol.Handler
}

// New constructs a Server and its glsp handler.
func New() *Server {
	s := &Server{docs: lang.NewStore()}
	s.ws = lang.NewWorkspace(s.docs, nil)
	s.handler = protocol.Handler{
		Initialize:  s.initialize,
		Initialized: s.initialized,
		Shutdown:    s.shutdown,
		SetTrace:    s.setTrace,

		TextDocumentDidOpen:   s.didOpen,
		TextDocumentDidChange: s.didChange,
		TextDocumentDidClose:  s.didClose,

		TextDocumentDocumentSymbol:     s.documentSymbol,
		TextDocumentHover:              s.hover,
		TextDocumentDefinition:         s.definition,
		TextDocumentReferences:         s.references,
		TextDocumentRename:             s.rename,
		TextDocumentFormatting:         s.formatting,
		TextDocumentSemanticTokensFull: s.semanticTokensFull,
		TextDocumentCompletion:         s.completion,
		TextDocumentDocumentHighlight:  s.documentHighlight,
		TextDocumentPrepareRename:      s.prepareRename,
		TextDocumentTypeDefinition:     s.typeDefinition,
		TextDocumentDocumentLink:       s.documentLink,
		TextDocumentSignatureHelp:      s.signatureHelp,

		TextDocumentPrepareCallHierarchy: s.prepareCallHierarchy,
		CallHierarchyIncomingCalls:       s.incomingCalls,
		CallHierarchyOutgoingCalls:       s.outgoingCalls,

		TextDocumentCodeAction: s.codeAction,

		WorkspaceSymbol:                s.workspaceSymbol,
		WorkspaceDidChangeWatchedFiles: s.didChangeWatchedFiles,
	}

	return s
}

// Handler returns the handler to register with the transport. It wraps the glsp
// protocol handler to add LSP 3.17 methods (inlay hints) glsp can't model.
func (s *Server) Handler() glsp.Handler {
	return wrappedHandler{inner: &s.handler, srv: s}
}

// serverCapabilities augments glsp's 3.16 capabilities with the 3.17 fields we
// advertise manually.
type serverCapabilities struct {
	protocol.ServerCapabilities

	InlayHintProvider bool `json:"inlayHintProvider,omitempty"`
}

// initializeResult is our InitializeResult so we can return the augmented
// capabilities (glsp's typed result can't carry inlayHintProvider).
type initializeResult struct {
	Capabilities serverCapabilities                   `json:"capabilities"`
	ServerInfo   *protocol.InitializeResultServerInfo `json:"serverInfo,omitempty"`
}

func (s *Server) initialize(_ *glsp.Context, params *protocol.InitializeParams) (any, error) {
	// Establish workspace roots and configured MROPATH for cross-file features.
	roots := workspaceRoots(params)
	s.docs.SetMROPaths(mroPathsFromOptions(params.InitializationOptions))
	s.ws = lang.NewWorkspace(s.docs, roots)
	s.watchFiles = clientSupportsFileWatching(params)

	// CreateServerCapabilities reflects which handler fields are set, so
	// advertised capabilities stay in sync with what we actually implement.
	capabilities := s.handler.CreateServerCapabilities()

	// glsp defaults the change-sync kind to Incremental; we only handle full
	// document text (see wholeText), so advertise Full sync. Without this the
	// client sends incremental deltas we ignore, and diagnostics never refresh
	// on edit.
	if sync, ok := capabilities.TextDocumentSync.(*protocol.TextDocumentSyncOptions); ok {
		full := protocol.TextDocumentSyncKindFull
		sync.Change = &full
	}

	// glsp advertises the semantic-tokens provider but with an empty legend;
	// supply ours so clients can decode the token stream.
	if st, ok := capabilities.SemanticTokensProvider.(*protocol.SemanticTokensOptions); ok {
		st.Legend = protocol.SemanticTokensLegend{
			TokenTypes:     lang.SemanticTokenLegend(),
			TokenModifiers: []string{},
		}
		st.Full = true
	}

	// glsp advertises completion with no trigger characters; add "." so member
	// completion (self./CALL.) fires automatically.
	if capabilities.CompletionProvider != nil {
		capabilities.CompletionProvider.TriggerCharacters = []string{"."}
	}

	// Signature help fires on the opening paren and each argument separator.
	if capabilities.SignatureHelpProvider != nil {
		capabilities.SignatureHelpProvider.TriggerCharacters = []string{"(", ","}
	}

	// Advertise prepareRename so the client validates before prompting.
	prepare := true
	capabilities.RenameProvider = &protocol.RenameOptions{PrepareProvider: &prepare}

	return initializeResult{
		Capabilities: serverCapabilities{
			ServerCapabilities: capabilities,
			InlayHintProvider:  true,
		},
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &Version,
		},
	}, nil
}

func (s *Server) initialized(ctx *glsp.Context, _ *protocol.InitializedParams) error {
	// Ask the client to watch .mro files so we learn when an unopened included
	// file changes on disk. Only when the client advertised dynamic registration,
	// and in a goroutine so this request never blocks the handler.
	if s.watchFiles && ctx.Call != nil {
		go ctx.Call(protocol.ServerClientRegisterCapability, protocol.RegistrationParams{
			Registrations: []protocol.Registration{{
				ID:     "watch-mro",
				Method: "workspace/didChangeWatchedFiles",
				RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{
					Watchers: []protocol.FileSystemWatcher{{GlobPattern: "**/*.mro"}},
				},
			}},
		}, nil)
	}

	return nil
}

// clientSupportsFileWatching reports whether the client advertised dynamic
// registration for workspace/didChangeWatchedFiles.
func clientSupportsFileWatching(params *protocol.InitializeParams) bool {
	ws := params.Capabilities.Workspace
	if ws == nil || ws.DidChangeWatchedFiles == nil {
		return false
	}
	d := ws.DidChangeWatchedFiles.DynamicRegistration

	return d != nil && *d
}

// workspaceRoots extracts root directories from the initialize params.
func workspaceRoots(params *protocol.InitializeParams) []string {
	var roots []string
	for _, f := range params.WorkspaceFolders {
		if p := lang.URIToPath(f.URI); p != "" {
			roots = append(roots, p)
		}
	}
	if len(roots) == 0 && params.RootURI != nil {
		if p := lang.URIToPath(*params.RootURI); p != "" {
			roots = append(roots, p)
		}
	}

	return roots
}

// mroPathsFromOptions reads initializationOptions.mroPaths ([]string).
func mroPathsFromOptions(opts any) []string {
	m, ok := opts.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := m["mroPaths"].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}

	return out
}

func (s *Server) shutdown(_ *glsp.Context) error {
	protocol.SetTraceValue(protocol.TraceValueOff)

	return nil
}

func (s *Server) setTrace(_ *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)

	return nil
}
