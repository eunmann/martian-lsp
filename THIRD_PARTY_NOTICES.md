# Third-party notices

martian-lsp is licensed under the MIT License (see `LICENSE`). Distributed
binaries statically link the following third-party Go modules. Their licenses
are reproduced/retained as required.

| Module | License |
|---|---|
| github.com/martian-lang/martian | MIT (Copyright 2014–2017 10x Genomics, Inc.) |
| github.com/tliron/glsp | Apache-2.0 |
| github.com/tliron/commonlog | Apache-2.0 |
| github.com/tliron/kutil | Apache-2.0 |
| github.com/sourcegraph/jsonrpc2 | MIT |
| github.com/gorilla/websocket | BSD-2-Clause |
| github.com/segmentio/ksuid | MIT |
| github.com/muesli/termenv | MIT |
| github.com/aymanbagabas/go-osc52/v2 | MIT |
| github.com/lucasb-eyer/go-colorful | MIT |
| github.com/rivo/uniseg | MIT |
| github.com/iancoleman/strcase | MIT |
| github.com/mattn/go-isatty, go-runewidth | MIT |
| github.com/pkg/errors | BSD-2-Clause |
| github.com/petermattis/goid | Apache-2.0 |
| github.com/sasha-s/go-deadlock | Apache-2.0 |
| golang.org/x/* (crypto, net, sys, term) | BSD-3-Clause |

Apache-2.0 components: per the license, this notice preserves attribution; the
full Apache-2.0 text is available at https://www.apache.org/licenses/LICENSE-2.0
and within each module's source in the Go module cache.

Run `go mod download` and inspect each module's `LICENSE` for the authoritative
text, or regenerate a bundle with a tool such as `go-licenses`.
