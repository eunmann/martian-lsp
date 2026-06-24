package lang

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// Workspace provides project-wide queries (symbols, references, rename) by
// discovering and reading every .mro file under its search roots. Open buffers
// are used in preference to on-disk contents; closed files are read fresh on each
// query (no cache), which keeps results correct without watcher bookkeeping.
type Workspace struct {
	store *Store
	roots []string // workspace folders / root configured at initialize
}

// NewWorkspace creates a workspace over the given roots backed by the open-doc
// store.
func NewWorkspace(store *Store, roots []string) *Workspace {
	return &Workspace{store: store, roots: roots}
}

// Symbols returns workspace symbols matching query (case-insensitive substring;
// empty matches all).
func (w *Workspace) Symbols(query string) []protocol.SymbolInformation {
	q := strings.ToLower(query)
	var out []protocol.SymbolInformation
	for _, path := range w.files() {
		d := w.doc(path)
		if d == nil {
			continue
		}
		for _, s := range d.Symbols() {
			out = appendSymbolInfo(out, d.URI, "", s, q)
		}
	}

	return out
}

// References returns all locations of the given symbol key across the workspace.
func (w *Workspace) References(sym string) []protocol.Location {
	if sym == "" {
		return nil
	}
	var out []protocol.Location
	for _, path := range w.files() {
		d := w.doc(path)
		if d == nil {
			continue
		}
		for _, r := range d.SymbolOccurrences(sym) {
			out = append(out, protocol.Location{URI: d.URI, Range: r})
		}
	}

	return out
}

// Rename returns a workspace edit renaming every occurrence of sym to newName
// across all files.
func (w *Workspace) Rename(sym, newName string) *protocol.WorkspaceEdit {
	if sym == "" {
		return nil
	}
	changes := map[protocol.DocumentUri][]protocol.TextEdit{}
	for _, path := range w.files() {
		d := w.doc(path)
		if d == nil {
			continue
		}
		var edits []protocol.TextEdit
		for _, r := range d.SymbolOccurrences(sym) {
			edits = append(edits, protocol.TextEdit{Range: r, NewText: newName})
		}
		if len(edits) > 0 {
			changes[d.URI] = edits
		}
	}
	if len(changes) == 0 {
		return nil
	}

	return &protocol.WorkspaceEdit{Changes: changes}
}

// DefiningFile returns the path of the .mro file that declares the callable
// named name, or ok=false if none in the workspace does.
func (w *Workspace) DefiningFile(name string) (string, bool) {
	for _, path := range w.files() {
		d := w.doc(path)
		if d == nil {
			continue
		}
		ast, _ := d.Compile()
		if ast == nil || ast.Callables == nil {
			continue
		}
		for _, c := range ast.Callables.List {
			if c.GetId() != name {
				continue
			}
			if loc := callableLoc(c); loc.File != nil && loc.File.FullPath == path {
				return path, true
			}
		}
	}

	return "", false
}

// files enumerates the .mro files under all search roots (deduplicated).
func (w *Workspace) files() []string {
	seen := map[string]bool{}
	var out []string
	for _, root := range w.searchRoots() {
		_ = filepath.WalkDir(root, func(path string, de fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // skip unreadable entries
			}
			if de.IsDir() {
				if name := de.Name(); len(name) > 1 && strings.HasPrefix(name, ".") {
					return filepath.SkipDir // skip .git and friends
				}

				return nil
			}
			if strings.HasSuffix(path, ".mro") && !seen[path] {
				seen[path] = true
				out = append(out, path)
			}

			return nil
		})
	}

	return out
}

// searchRoots is the deduplicated set of directories to walk: configured roots,
// MROPATH entries, and the directories of currently-open documents.
func (w *Workspace) searchRoots() []string {
	seen := map[string]bool{}
	var roots []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			roots = append(roots, p)
		}
	}
	for _, r := range w.roots {
		add(r)
	}
	for _, p := range w.store.MROPaths() {
		add(p)
	}
	for _, d := range w.store.OpenDocs() {
		if d.Path != "" {
			add(filepath.Dir(d.Path))
		}
	}

	return roots
}

// doc returns the open document for a path, or a transient one read from disk.
func (w *Workspace) doc(path string) *Document {
	uri := PathToURI(path)
	if d, ok := w.store.Get(uri); ok {
		return d
	}
	b, err := os.ReadFile(path) //nolint:gosec // intentionally reading workspace .mro files
	if err != nil {
		return nil
	}

	return &Document{URI: uri, Path: path, Text: string(b), Extra: w.store.MROPaths()}
}

// appendSymbolInfo flattens a DocumentSymbol (and its parameter children) into
// SymbolInformation entries matching the query.
func appendSymbolInfo(out []protocol.SymbolInformation, uri, container string, s protocol.DocumentSymbol, q string) []protocol.SymbolInformation {
	if q == "" || strings.Contains(strings.ToLower(s.Name), q) {
		info := protocol.SymbolInformation{
			Name:     s.Name,
			Kind:     s.Kind,
			Location: protocol.Location{URI: uri, Range: s.SelectionRange},
		}
		if container != "" {
			c := container
			info.ContainerName = &c
		}
		out = append(out, info)
	}
	for _, child := range s.Children {
		if child.Kind == protocol.SymbolKindField { // params, not call sites
			out = appendSymbolInfo(out, uri, s.Name, child, q)
		}
	}

	return out
}
