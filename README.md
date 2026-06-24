# martian-lsp

A Language Server for the [Martian](https://martian-lang.org) (`.mro`) pipeline
language. It reuses Martian's own parser and type-checker
(`github.com/martian-lang/martian/martian/syntax`) so its understanding of a
pipeline matches `mro check` exactly.

It lives in a **separate repo on purpose**: the Martian core is intentionally
kept light, and consuming the parser as a Go module means zero code is copied
into — or maintained against — that repo.

## Features (v1)

| Feature | Notes |
|---|---|
| **Diagnostics** | Live type/wiring/`filetype`/syntax errors, matching `mro check`. Republished on every change. |
| **Document symbols** | Outline of stages, pipelines, their params and calls. |
| **Hover** | Signature of a stage/pipeline; type + help text of a param or reference. |
| **Go-to-definition** | Jump from `STAGE.output` / `self.input` / a call to its declaration, across `@include` files. |
| **Find references** | All uses of a callable, input, or output (current document). |
| **Rename** | Rename a callable/param everywhere it's used, including call-site bindings (current document). |
| **Formatting** | Canonical `mro` formatting of the whole document. |
| **Semantic tokens** | Server-driven highlighting (callables, parameters) — consistent across LSP clients. |
| **Completion** | Context-aware: keywords, types, `src` langs, callable names, call input params, `self.` inputs, `CALL.` outputs. Works mid-edit on unparseable buffers (last-good-AST snapshot + pure lexical context). Triggers on `.`. |
| **Signature help** | Inside `call X(`, shows X's input signature with the active argument highlighted. Triggers on `(` and `,`. |
| **Document highlight** | Highlights every occurrence of the symbol under the cursor. |
| **Prepare rename** | Validates the rename target before prompting. |
| **Type definition** | Jumps a param/reference to its `filetype`/`struct` declaration. |
| **Document links** | `@include "..."` paths are clickable. |
| **Call hierarchy** | Incoming (pipelines that call a stage/pipeline) and outgoing (a pipeline's own calls). |
| **Workspace symbols** | Project-wide symbol search across all `.mro` files. |
| **Cross-file references & rename** | References and rename span every `.mro` file under the workspace roots / MROPATH, not just the open buffer. |
| **File watching** | Registers a `**/*.mro` watcher; when an unopened included file changes on disk, open documents' diagnostics refresh. |
| **Code actions** | In a call: "Add missing arguments"; for an undefined callee, "Add @include" (if defined elsewhere) and "Create stage/pipeline" scaffolds. |
| **Inlay hints** | Inline type hints on call-argument and return bindings (e.g. `name: string = self.x`). |
| **Snippets** | `stage`/`pipeline`/`struct`/`call`/`return`/`filetype`/`@include` skeletons offered as completion. |
| **src navigation** | Go-to-definition on a stage's `src "path"` jumps to the implementation file on disk (searched along MROPATH). |

### Configuration

Pass `mroPaths` (extra include/search directories) via the client's
`initializationOptions`, e.g. in Neovim:

```lua
vim.lsp.config('martian', {
  cmd = { 'mrlsp' },
  filetypes = { 'mro' },
  root_markers = { '.git' },
  init_options = { mroPaths = { '/abs/path/to/shared/mros' } },
})
```

Workspace features search the workspace folders, those `mroPaths`, and the
directories of open files.

Planned (not yet implemented): `using(...)` member completion, nested
struct-member completion, and more code actions (add-missing-arguments,
add-`@include`).

## Build

Requires Go 1.26+.

```sh
make build          # produces ./mrlsp
make test           # unit + stdio end-to-end tests
make test-nvim      # headless Neovim integration checks (requires nvim 0.12+)
make lint           # golangci-lint with auto-fix
make lint-check     # golangci-lint, zero-issue CI gate
make install        # installs to ~/.local/bin (override with PREFIX=)
```

> The Martian parser is pinned by **commit** pseudo-version in `go.mod`
> (`github.com/martian-lang/martian`'s `go.mod` has no `/v4` suffix, so tags don't
> resolve as Go versions). To hack on the parser locally, add a `go.work` or a
> `replace` pointing at a checkout. Bump the parser with
> `go get github.com/martian-lang/martian@<commit>`.

## CI & editors

- **CI** (`.github/workflows/ci.yml`): build, vet, `go test`, golangci-lint, and the
  headless Neovim suite on every push/PR.
- **Releases** (`.github/workflows/release.yml`): tag `vX.Y.Z` to build
  cross-platform `mrlsp` binaries and attach them to the GitHub release.
- **VS Code** client in [`vscode/`](vscode) (`npm install && npm run compile`, then F5).
- **Neovim** setup is documented below.

## Editor setup

The server speaks LSP over stdio, so any LSP client works.

### Neovim 0.12+ (native LSP API, no nvim-lspconfig)

Neovim 0.12 has a built-in LSP config API. Put this anywhere in your config that
runs at startup (e.g. a file under `lua/custom/plugins/` on kickstart, which is
`require`d automatically):

```lua
-- Recognize Martian sources and metadata files as filetype "mro".
vim.filetype.add {
  extension = { mro = 'mro' },
  filename = { ['_invocation'] = 'mro', ['_mrosource'] = 'mro' },
}

vim.lsp.config('martian', {
  cmd = { 'mrlsp' },            -- on your PATH (make install -> ~/.local/bin)
  filetypes = { 'mro' },
  root_markers = { '.git' },    -- single-file mode otherwise
})
vim.lsp.enable 'martian'
```

Default LSP keymaps then work in any `.mro` buffer: `K` (hover), `grn` (rename,
not yet implemented), and your config's go-to-definition / document-symbol maps
(`grd` / `gO` on kickstart). Diagnostics appear automatically; navigate with
`[d` / `]d`.

Try it against the bundled [`examples/`](examples): open `examples/hello.mro`
(hover/definition/symbols), `examples/diagnostics.mro` (a live error), and
`examples/include/aligner.mro` (cross-file definition into `dna.mro`).

### VS Code

The existing `martian-lang` extension provides syntax highlighting; wiring it to
launch `mrlsp` as a language client (via `vscode-languageclient`) is a small
follow-up. Until then, any generic "start an LSP for this language" bridge
extension pointed at the `mrlsp` binary with language id `mro` works.

## Architecture

```
LSP client ──stdio/JSON-RPC──▶ cmd/mrlsp ──▶ internal/server (glsp handlers)
                                                  │
                                        internal/lang
                                          ├ docs.go         open documents
                                          ├ compile.go      syntax.ParseSourceBytes
                                          ├ diagnostics.go  errors → LSP diagnostics
                                          ├ index.go        position→node index (hover/definition)
                                          ├ symbols.go      outline
                                          └ ranges.go       range synthesis
                                                  │
                              github.com/martian-lang/martian/martian/syntax  (module dep)
```

The Martian AST exposes start positions but not end ranges, and no public
declaration walker; `internal/lang/index.go` and `ranges.go` synthesize token
ranges from the document text and walk the exported AST (plus `syntax.WalkExp`
for expressions).

## License

MIT — see [`LICENSE`](LICENSE). Bundled third-party dependencies and their
licenses (MIT/Apache-2.0/BSD) are listed in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
