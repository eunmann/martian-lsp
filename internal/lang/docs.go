// Package lang is the language-intelligence layer of the Martian language
// server: it owns open documents and turns Martian's parser/type-checker output
// (from github.com/martian-lang/martian/martian/syntax) into LSP features.
package lang

import (
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/martian-lang/martian/martian/syntax"
)

// Document is an in-memory copy of a single open .mro file.
type Document struct {
	URI     string   // the LSP document URI (file://...)
	Path    string   // the resolved absolute filesystem path
	Text    string   // the current full text
	Version int32    // the LSP document version
	Extra   []string // additional MRO include search paths (configured MROPATH)
}

// MROPaths returns the include search paths used when compiling this document,
// in priority order:
//
//	1. the file's own directory;
//	2. the nearest ancestor "mro" directory, if any — Martian projects keep
//	   their pipelines under an mro/ root and write @include paths relative to
//	   it, so this resolves them without explicit configuration;
//	3. any configured MROPATH entries (from initializationOptions / $MROPATH).
//
// The result is de-duplicated, preserving the order above.
func (d *Document) MROPaths() []string {
	if d.Path == "" {
		return dedupePaths(d.Extra)
	}
	paths := make([]string, 0, len(d.Extra)+2) //nolint:mnd // dir + mro-root
	paths = append(paths, filepath.Dir(d.Path))
	if root := mroRootOf(d.Path); root != "" {
		paths = append(paths, root)
	}
	paths = append(paths, d.Extra...)

	return dedupePaths(paths)
}

// mroRootOf returns the nearest ancestor directory of path named "mro", or ""
// if there is none. This is the conventional include root for Martian projects.
func mroRootOf(path string) string {
	dir := filepath.Dir(path)
	for {
		if filepath.Base(dir) == "mro" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached the filesystem root
			return ""
		}
		dir = parent
	}
}

// dedupePaths returns in with empty strings and duplicates removed, preserving
// first-seen order.
func dedupePaths(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}

	return out
}

// Store is a concurrency-safe set of open documents keyed by URI. It also caches
// the last successfully-parsed AST per URI ("snapshot"), which survives the
// Document being replaced on each edit and powers completion on buffers that no
// longer parse.
type Store struct {
	mu        sync.RWMutex
	docs      map[string]*Document
	snapshots map[string]*syntax.Ast
	mroPaths  []string // configured MROPATH, applied to every document
}

// NewStore returns an empty document store.
func NewStore() *Store {
	return &Store{
		docs:      make(map[string]*Document),
		snapshots: make(map[string]*syntax.Ast),
	}
}

// SetMROPaths configures the MROPATH applied to every document's compilation.
func (s *Store) SetMROPaths(paths []string) {
	s.mu.Lock()
	s.mroPaths = paths
	s.mu.Unlock()
}

// MROPaths returns the configured MROPATH.
func (s *Store) MROPaths() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.mroPaths
}

// OpenDocs returns a snapshot of the currently open documents.
func (s *Store) OpenDocs() []*Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Document, 0, len(s.docs))
	for _, d := range s.docs {
		out = append(out, d)
	}

	return out
}

// SetSnapshot records the last-good AST for uri. Nil ASTs are ignored so a
// transient syntax error doesn't erase known symbols.
func (s *Store) SetSnapshot(uri string, ast *syntax.Ast) {
	if ast == nil {
		return
	}
	s.mu.Lock()
	s.snapshots[uri] = ast
	s.mu.Unlock()
}

// Snapshot returns the last-good AST for uri, or nil.
func (s *Store) Snapshot(uri string) *syntax.Ast {
	s.mu.RLock()
	ast := s.snapshots[uri]
	s.mu.RUnlock()

	return ast
}

// Set inserts or replaces the document for uri and returns it.
func (s *Store) Set(uri, text string, version int32) *Document {
	s.mu.Lock()
	doc := &Document{
		URI:     uri,
		Path:    URIToPath(uri),
		Text:    text,
		Version: version,
		Extra:   s.mroPaths,
	}
	s.docs[uri] = doc
	s.mu.Unlock()

	return doc
}

// Get returns the document for uri, if open.
func (s *Store) Get(uri string) (*Document, bool) {
	s.mu.RLock()
	doc, ok := s.docs[uri]
	s.mu.RUnlock()

	return doc, ok
}

// Delete drops the document for uri (on didClose).
func (s *Store) Delete(uri string) {
	s.mu.Lock()
	delete(s.docs, uri)
	delete(s.snapshots, uri)
	s.mu.Unlock()
}

// URIToPath converts a file:// URI to a filesystem path. Non-file URIs are
// returned with the scheme stripped as a best effort.
func URIToPath(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" {
		// u.Path is already percent-decoded.
		return u.Path
	}

	return strings.TrimPrefix(uri, "file://")
}

// PathToURI converts an absolute filesystem path to a file:// URI, escaping as
// needed so paths with spaces or unusual characters round-trip correctly.
func PathToURI(path string) string {
	u := url.URL{Scheme: "file", Path: path}

	return u.String()
}
