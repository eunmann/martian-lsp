// Command mrlsp is a Language Server for the Martian (MRO) pipeline language.
// It speaks LSP over stdio and reuses Martian's own parser/type-checker for
// diagnostics, symbols, hover, and go-to-definition.
package main

import (
	"os"

	"github.com/eunmann/martian-lsp/internal/server"

	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"
	glspserver "github.com/tliron/glsp/server"
)

func main() {
	// Logs go to stderr; stdout is reserved for the LSP JSON-RPC stream.
	commonlog.Configure(1, nil)

	srv := server.New()
	s := glspserver.NewServer(srv.Handler(), "martian-lsp", false)
	if err := s.RunStdio(); err != nil {
		os.Exit(1)
	}
}
