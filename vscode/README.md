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

> Not yet on the VS Code Marketplace. For now, install the packaged extension:
> download the `.vsix` from Releases (or build it — see below) and run
> *Extensions: Install from VSIX…*.

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
- `martian.mroPaths` — extra MRO include search directories (MROPATH).

> Basic syntax highlighting comes from the separate `martian-lang` TextMate
> grammar; this extension adds the language-server features (server-driven
> semantic tokens layer on top).
