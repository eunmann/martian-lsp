package lang

import (
	"github.com/martian-lang/martian/martian/syntax"
)

// Compile parses and type-checks the document's current text using Martian's
// own front end. It returns the parsed AST (which may be partial or nil on a
// hard syntax error) and any error, which may be a syntax.ErrorList aggregating
// many semantic errors.
//
// checkSrcPaths is false on purpose: an editor should not flag a pipeline just
// because a stage's `src` file is not present on disk in the current working
// tree. We care about parse + type/wiring errors here.
func (d *Document) Compile() (*syntax.Ast, error) {
	_, _, ast, err := syntax.ParseSourceBytes(
		[]byte(d.Text),
		d.Path,
		d.MROPaths(),
		false, // checkSrcPaths
	)

	// Returned verbatim: callers type-inspect it (syntax.ErrorList / *AstError)
	// to build diagnostics, so it must not be wrapped.
	return ast, err //nolint:wrapcheck
}
