package server

import (
	"slices"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) didOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	td := params.TextDocument
	doc := s.docs.Set(td.URI, td.Text, td.Version)
	s.publishDiagnostics(ctx, doc.URI)

	return nil
}

func (s *Server) didChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := params.TextDocument.URI
	text, ok := wholeText(params.ContentChanges)
	if !ok {
		return nil
	}
	version := int32(0)
	if params.TextDocument.Version != 0 {
		version = params.TextDocument.Version
	}
	doc := s.docs.Set(uri, text, version)
	s.publishDiagnostics(ctx, doc.URI)

	return nil
}

func (s *Server) didClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.docs.Delete(params.TextDocument.URI)
	// Clear diagnostics for the closed file.
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []protocol.Diagnostic{},
	})

	return nil
}

// wholeText extracts the full document text from a full-sync change set. We
// advertise TextDocumentSyncKindFull, so clients send the entire document.
func wholeText(changes []any) (string, bool) {
	for _, v := range slices.Backward(changes) {
		switch c := v.(type) {
		case protocol.TextDocumentContentChangeEventWhole:
			return c.Text, true
		case protocol.TextDocumentContentChangeEvent:
			if c.Range == nil {
				return c.Text, true
			}
		}
	}

	return "", false
}

// didChangeWatchedFiles fires when an .mro file changes on disk. An open
// document may @include the changed file, so re-publish diagnostics for every
// open document (workspace queries already read fresh, so nothing else to do).
func (s *Server) didChangeWatchedFiles(ctx *glsp.Context, _ *protocol.DidChangeWatchedFilesParams) error {
	for _, doc := range s.docs.OpenDocs() {
		s.publishDiagnostics(ctx, doc.URI)
	}

	return nil
}

// publishDiagnostics compiles the document and pushes diagnostics for it (and
// any included files that produced errors).
func (s *Server) publishDiagnostics(ctx *glsp.Context, uri string) {
	doc, ok := s.docs.Get(uri)
	if !ok {
		return
	}
	// Keep the completion snapshot warm: record the last buffer that parsed.
	if ast, _ := doc.Compile(); ast != nil {
		s.docs.SetSnapshot(uri, ast)
	}
	for _, fd := range doc.Diagnose() {
		diags := fd.Diagnostics
		if diags == nil {
			diags = []protocol.Diagnostic{}
		}
		ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
			URI:         fd.URI,
			Diagnostics: diags,
		})
	}
}
