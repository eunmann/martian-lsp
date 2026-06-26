package server

import (
	"github.com/eunmann/martian-lsp/internal/lang"
	"github.com/martian-lang/martian/martian/syntax"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func (s *Server) documentSymbol(_ *glsp.Context, params *protocol.DocumentSymbolParams) (any, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	return doc.Symbols(), nil
}

func (s *Server) hover(_ *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	md, ok := doc.Hover(params.Position)
	if !ok {
		return nil, nil
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: md,
		},
	}, nil
}

func (s *Server) definition(_ *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	loc, ok := doc.Definition(params.Position)
	if !ok {
		return nil, nil
	}

	return loc, nil
}

func (s *Server) references(_ *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	sym, ok := doc.SymbolAt(params.Position)
	if !ok {
		return nil, nil
	}
	if s.ws != nil {
		return s.ws.References(sym), nil // workspace-wide
	}

	return doc.References(params.Position), nil
}

func (s *Server) rename(_ *glsp.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	sym, ok := doc.SymbolAt(params.Position)
	if !ok {
		return nil, nil
	}
	if s.ws != nil {
		return s.ws.Rename(sym, params.NewName), nil // workspace-wide
	}

	return doc.Rename(params.Position, params.NewName), nil
}

func (s *Server) codeAction(_ *glsp.Context, params *protocol.CodeActionParams) (any, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	return doc.CodeActions(s.ws, params.Range), nil
}

func (s *Server) workspaceSymbol(_ *glsp.Context, params *protocol.WorkspaceSymbolParams) ([]protocol.SymbolInformation, error) {
	if s.ws == nil {
		return nil, nil
	}

	return s.ws.Symbols(params.Query), nil
}

func (s *Server) formatting(_ *glsp.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	return doc.Format(), nil
}

func (s *Server) semanticTokensFull(_ *glsp.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	return &protocol.SemanticTokens{Data: doc.SemanticTokens()}, nil
}

func (s *Server) documentHighlight(_ *glsp.Context, params *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	return doc.DocumentHighlights(params.Position), nil
}

func (s *Server) prepareRename(_ *glsp.Context, params *protocol.PrepareRenameParams) (any, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	rng, ok := doc.PrepareRename(params.Position)
	if !ok {
		return nil, nil
	}

	return rng, nil
}

func (s *Server) typeDefinition(_ *glsp.Context, params *protocol.TypeDefinitionParams) (any, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	loc, ok := doc.TypeDefinition(params.Position)
	if !ok {
		return nil, nil
	}

	return loc, nil
}

func (s *Server) documentLink(_ *glsp.Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	return doc.DocumentLinks(), nil
}

func (s *Server) signatureHelp(_ *glsp.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	if ast, _ := doc.Compile(); ast != nil {
		s.docs.SetSnapshot(doc.URI, ast)
	}

	return doc.SignatureHelp(s.docs.Snapshot(doc.URI), params.Position), nil
}

func (s *Server) prepareCallHierarchy(_ *glsp.Context, params *protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	if ast, _ := doc.Compile(); ast != nil {
		s.docs.SetSnapshot(doc.URI, ast)
	}

	return doc.PrepareCallHierarchy(s.docs.Snapshot(doc.URI), params.Position), nil
}

func (s *Server) incomingCalls(_ *glsp.Context, params *protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error) {
	doc, snap, ok := s.docForItem(params.Item.URI)
	if !ok {
		return nil, nil
	}

	return doc.IncomingCalls(snap, params.Item), nil
}

func (s *Server) outgoingCalls(_ *glsp.Context, params *protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error) {
	doc, snap, ok := s.docForItem(params.Item.URI)
	if !ok {
		return nil, nil
	}

	return doc.OutgoingCalls(snap, params.Item), nil
}

// docForItem returns the open document for a call-hierarchy item's URI plus its
// symbol snapshot (compiling on demand if none is cached).
func (s *Server) docForItem(uri string) (*lang.Document, *syntax.Ast, bool) {
	doc, ok := s.docs.Get(uri)
	if !ok {
		return nil, nil, false
	}
	snap := s.docs.Snapshot(uri)
	if snap == nil {
		if ast, _ := doc.Compile(); ast != nil {
			snap = ast
		}
	}

	return doc, snap, true
}

func (s *Server) completion(_ *glsp.Context, params *protocol.CompletionParams) (any, error) {
	doc, ok := s.docs.Get(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}
	// Refresh the symbol snapshot if the current buffer parses; otherwise fall
	// back to the last good one so completion still works mid-edit.
	if ast, _ := doc.Compile(); ast != nil {
		s.docs.SetSnapshot(doc.URI, ast)
	}

	return doc.Complete(s.docs.Snapshot(doc.URI), params.Position), nil
}
