module github.com/eunmann/martian-lsp

go 1.26.4

require (
	github.com/martian-lang/martian v0.0.0-20260506211707-4a558e7dd93b
	github.com/tliron/commonlog v0.2.17
	github.com/tliron/glsp v0.2.2
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/muesli/termenv v0.15.2 // indirect
	github.com/petermattis/goid v0.0.0-20180202154549-b0b1615b78e5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.1 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/sourcegraph/jsonrpc2 v0.2.0 // indirect
	github.com/tliron/kutil v0.3.24 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/term v0.29.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
)

// Martian's go.mod has no /v4 suffix, so a tag won't resolve as a Go module
// version -- it is pinned by commit (see the require above). To hack on the
// Martian parser locally, add a go.work or a replace pointing at a checkout.
