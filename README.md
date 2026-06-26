# martian-lsp

A Language Server for the [Martian](https://martian-lang.org) (`.mro`) pipeline
language ([martian-lang/martian](https://github.com/martian-lang/martian)). It
reuses Martian's own parser and type-checker
([`martian/syntax`](https://github.com/martian-lang/martian/tree/master/martian/syntax)),
so its diagnostics match `mro check`.

## Features

| Feature | Notes |
|---|---|
| **Diagnostics** | Live type/wiring/`filetype`/syntax errors, matching `mro check`. Republished on every change. |
| **Document symbols** | Outline of stages, pipelines, their params and calls. |
| **Hover** | Signature of a stage/pipeline; type + help text of a param or reference. |
| **Go-to-definition** | Jump from `STAGE.output` / `self.input` / a call to its declaration, across `@include` files. |
| **Find references** | All uses of a callable, input, or output, workspace-wide. |
| **Rename** | Rename a callable/param everywhere it's used, including call-site bindings, workspace-wide. |
| **Formatting** | Canonical `mro` formatting of the whole document. |
| **Semantic tokens** | Server-driven highlighting (callables, parameters) — consistent across LSP clients. |
| **Completion** | Context-aware: keywords, types, `src` langs, callable names, call input params, `self.` inputs, `CALL.` outputs. Works mid-edit on unparseable buffers. Triggers on `.`. |
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

### Include resolution

`@include "..."` paths are resolved against, in order:

1. the including file's own directory;
2. the nearest ancestor **`mro/`** directory;
3. the configured `mroPaths` — relative entries resolve against the workspace
   root, so `mroPaths = { 'mro' }` works;
4. the **`MROPATH`** environment variable.

Workspace features (cross-file references, rename, symbols) search the workspace
folders, the `mroPaths`/`MROPATH` entries, and the directories of open files.

Planned (not yet implemented): `using(...)` member completion and nested
struct-member completion.

## Install

### The server (`mrlsp`)

```sh
go install github.com/eunmann/martian-lsp/cmd/mrlsp@latest
```

…or download a prebuilt binary for your platform from
[Releases](https://github.com/eunmann/martian-lsp/releases) and put it on your
PATH. (VS Code users can skip this — the extension auto-downloads it.)

### VS Code

Install the extension from the packaged `.vsix` attached to each
[release](https://github.com/eunmann/martian-lsp/releases) (Marketplace and Open
VSX listings are planned). It bundles a TextMate grammar and auto-downloads the
matching `mrlsp` on first use — no Go toolchain required. See
[`vscode/README.md`](vscode/README.md) for details.

Then configure your editor — see [Editor setup](#editor-setup) below for Neovim
and VS Code.

## Build (from source)

Requires Go 1.26.4+ (matching the `go` directive in `go.mod`).

```sh
make build          # produces ./mrlsp
make test           # unit + stdio end-to-end tests
make test-nvim      # headless Neovim integration checks (requires nvim 0.12+)
make lint           # golangci-lint with auto-fix
make lint-check     # golangci-lint, zero-issue CI gate
make install        # installs to ~/.local/bin (override with PREFIX=)
```

> The Martian parser
> ([martian-lang/martian](https://github.com/martian-lang/martian)) is pinned by
> **commit** pseudo-version in `go.mod` (its `go.mod` has no `/v4` suffix, so tags
> don't resolve as Go versions). To hack on the parser locally, add a `go.work` or
> a `replace` pointing at a checkout. Bump the parser with
> `go get github.com/martian-lang/martian@<commit>`.

## CI

- **PR validation** (`.github/workflows/pr-validation.yml`): on every push/PR —
  `go mod tidy` drift check, `govulncheck`, golangci-lint, build, `go test -race`,
  and the headless Neovim suite (parallel jobs).
- **Releases** (`.github/workflows/release.yml`): tag `vX.Y.Z` to build
  cross-platform `mrlsp` binaries plus the VS Code `.vsix` and attach them to the
  GitHub release (publishing to the Marketplace / Open VSX when the
  `VSCE_PAT` / `OVSX_TOKEN` secrets are set).

## Editor setup

The server speaks LSP over stdio, so any LSP client works.

### Neovim 0.11+ (native LSP API, no nvim-lspconfig)

Neovim 0.11 added a built-in LSP config API (`vim.lsp.config` / `vim.lsp.enable`).
Put this anywhere in your config that runs at startup (e.g. a file under
`lua/custom/plugins/` on kickstart, which is `require`d automatically):

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

Default LSP keymaps then work in any `.mro` buffer: `K` (hover), `grn` (rename),
`gra` (code action), and your config's go-to-definition / document-symbol maps
(`grd` / `gO` on kickstart). Diagnostics appear automatically; navigate with
`[d` / `]d`.

For tree-sitter highlighting, also install
[`tree-sitter-martian`](https://github.com/eunmann/tree-sitter-martian).

Try it against the bundled [`examples/`](examples): open `examples/hello.mro`
(hover/definition/symbols), `examples/diagnostics.mro` (a live error), and
`examples/include/aligner.mro` (cross-file definition into `dna.mro`).

### VS Code

The extension in [`vscode/`](vscode) is a full language client: it bundles a
TextMate grammar for `.mro` syntax highlighting and launches `mrlsp` (downloaded
automatically on first use) over `vscode-languageclient`. Install it from the
`.vsix` on a [release](https://github.com/eunmann/martian-lsp/releases), then open
any `.mro` file. Configure extra include paths with the `martian.mroPaths`
setting; see [`vscode/README.md`](vscode/README.md).

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

## License

MIT — see [`LICENSE`](LICENSE). Bundled third-party dependencies and their
licenses (MIT/Apache-2.0/BSD) are listed in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
