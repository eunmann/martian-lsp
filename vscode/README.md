# Martian (MRO) — VS Code

Language-server client for the Martian (`.mro`) pipeline language. Provides
diagnostics, hover, go-to-definition, references, rename, completion, signature
help, semantic tokens, call hierarchy, inlay hints, code actions, and more.

## Install

The server binary (`mrlsp`) is handled automatically: on first use the extension
downloads the matching `mrlsp` for your platform from
[GitHub Releases](https://github.com/eunmann/martian-lsp/releases) and caches it.
You don't need Go or anything on your PATH.

Override with the `martian.serverPath` setting if you want a specific binary
(e.g. one you built with `make install`, or `go install github.com/eunmann/martian-lsp/cmd/mrlsp@latest`).

### Install the extension

Until it's on a marketplace, install the packaged `.vsix` attached to each
[release](https://github.com/eunmann/martian-lsp/releases):

```sh
# download martian-lsp-vX.Y.Z.vsix from the release, then:
code --install-extension martian-lsp-vX.Y.Z.vsix
```

…or in the GUI: Extensions view → `…` → **Install from VSIX…**.

Marketplace listings are planned: once published you'll be able to
`ext install eunmann.martian-lsp` (VS Code Marketplace) or install from Open VSX
(VSCodium / Cursor / Windsurf). The release workflow already builds the `.vsix`
and will publish to both automatically when the `VSCE_PAT` / `OVSX_TOKEN`
repository secrets are set.

## Build the extension

```sh
cd vscode
npm install
npm run compile
npx @vscode/vsce package   # -> martian-lsp-<version>.vsix
```

Press F5 in VS Code to launch an Extension Development Host.

## Settings

- `martian.serverPath` — path to the `mrlsp` binary (default: auto-download / `mrlsp` on PATH).
- `martian.mroPaths` — extra MRO include search directories. Relative entries
  resolve against the workspace root. The nearest ancestor `mro/` directory and
  the `$MROPATH` environment variable are also searched automatically.

> This extension bundles a TextMate grammar for `.mro` syntax highlighting; the
> language server adds server-driven semantic-token highlighting on top.
