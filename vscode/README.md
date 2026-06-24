# Martian (MRO) — VS Code

Language-server client for the Martian (`.mro`) pipeline language. It launches the
`mrlsp` server and provides diagnostics, hover, go-to-definition, references,
rename, completion, signature help, semantic tokens, call hierarchy, inlay hints,
code actions, and more.

## Prerequisites

Install the `mrlsp` binary (from the parent repo):

```sh
make install   # -> ~/.local/bin/mrlsp
```

## Build & run

```sh
cd vscode
npm install
npm run compile
```

Then press F5 in VS Code to launch an Extension Development Host, or package with
`npx vsce package`.

## Settings

- `martian.serverPath` — path to the `mrlsp` binary (default `mrlsp` on PATH).
- `martian.mroPaths` — extra MRO include search directories (MROPATH).

> Syntax highlighting comes from the separate `martian-lang` TextMate grammar;
> this extension provides the language-server features. Server-driven semantic
> tokens layer on top.
