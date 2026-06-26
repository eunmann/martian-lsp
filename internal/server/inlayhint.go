package server

import (
	"encoding/json"
	"fmt"

	"github.com/eunmann/martian-lsp/internal/lang"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// inlayHintMethod is the LSP 3.17 method; glsp (3.16) doesn't model it, so we
// dispatch it manually in wrappedHandler.
const inlayHintMethod = "textDocument/inlayHint"

type inlayHintParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Range        protocol.Range                  `json:"range"`
}

// wrappedHandler intercepts methods glsp's 3.16 Handler can't model (inlay
// hints) and delegates everything else to it.
type wrappedHandler struct {
	inner *protocol.Handler
	srv   *Server
}

func (h wrappedHandler) Handle(ctx *glsp.Context) (any, bool, bool, error) {
	if ctx.Method == inlayHintMethod {
		var p inlayHintParams
		if err := json.Unmarshal(ctx.Params, &p); err != nil {
			return nil, true, false, fmt.Errorf("inlayHint params: %w", err)
		}

		return h.srv.inlayHints(p), true, true, nil
	}

	return h.inner.Handle(ctx) //nolint:wrapcheck // pure delegation to glsp's handler
}

func (s *Server) inlayHints(p inlayHintParams) []lang.InlayHint {
	doc, ok := s.docs.Get(p.TextDocument.URI)
	if !ok {
		return nil
	}
	if ast, _ := doc.Compile(); ast != nil {
		s.docs.SetSnapshot(doc.URI, ast)
	}

	return doc.InlayHints(s.docs.Snapshot(doc.URI), p.Range)
}
