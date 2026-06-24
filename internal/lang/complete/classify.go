package complete

import (
	"regexp"
	"strings"
)

// Kind is the completion context category at a cursor.
type Kind int

// Completion context kinds.
const (
	None         Kind = iota // suppress (string, comment, or no useful completion)
	TopLevel                 // file scope: declaration keywords
	PipelineBody             // inside pipeline {}: call/return/retain
	InOutType                // after in/out: type names
	SrcLang                  // after src: py/exec/comp
	CallName                 // after call: callable names
	BindingLHS               // call arg list, parameter-name position
	SelfRef                  // after self.: pipeline inputs
	OutputRef                // after CALL.: that call's outputs
)

// Context is the pure result of classification: what to complete, the partial
// token already typed, where it starts (for the replacement range), and any
// names needed to resolve candidates against the symbol snapshot.
type Context struct {
	Kind         Kind
	Prefix       string // the partial identifier already typed
	ReplaceStart int    // 0-indexed column where Prefix begins (replacement start)
	Pipeline     string // enclosing pipeline name (SelfRef / scope)
	Name         string // OutputRef: ref base (call alias); BindingLHS: callee DecId
}

var (
	dotRe      = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z0-9_]*)$`)
	wordRe     = regexp.MustCompile(`@?[A-Za-z_][A-Za-z0-9_]*$`)
	pipelineRe = regexp.MustCompile(`pipeline\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

// Classify determines the completion context at a 0-indexed (line, col) cursor.
// It is pure and never parses the buffer.
func Classify(text string, line, col int) Context {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return Context{Kind: None}
	}
	cur := lines[line]
	if col > len(cur) {
		col = len(cur)
	}
	if inStr, inComment := lineMasked(cur, col); inStr || inComment {
		return Context{Kind: None}
	}
	before := cur[:col]

	if ctx, ok := classifyDot(before, text, lines, line, col); ok {
		return ctx
	}

	partial := wordRe.FindString(before)
	replaceStart := col - len(partial)
	st := scan(text, offsetOf(lines, line, replaceStart))
	stmtStart := strings.TrimSpace(before[:replaceStart]) == ""

	return classifyToken(st, partial, replaceStart, stmtStart)
}

// classifyDot handles `self.<x>` and `CALL.<x>` member references (single-line,
// unambiguous). Returns ok=false when the cursor isn't after such a reference.
func classifyDot(before, text string, lines []string, line, col int) (Context, bool) {
	m := dotRe.FindStringSubmatchIndex(before)
	if m == nil {
		return Context{}, false
	}
	if m[2] != 0 && before[m[2]-1] == '.' {
		return Context{}, false // nested member access (self.a.b) — deferred
	}
	base := before[m[2]:m[3]]
	partial := before[m[4]:m[5]]
	replaceStart := col - len(partial)
	pl := enclosingPipeline(text, lines, line, col)
	if base == "self" {
		return Context{Kind: SelfRef, Pipeline: pl, Prefix: partial, ReplaceStart: replaceStart}, true
	}

	return Context{Kind: OutputRef, Name: base, Pipeline: pl, Prefix: partial, ReplaceStart: replaceStart}, true
}

// classifyToken classifies based on the structural scan (last token + brackets).
func classifyToken(st scanState, partial string, replaceStart int, stmtStart bool) Context {
	switch st.lastToken {
	case "in", "out":
		return Context{Kind: InOutType, Prefix: partial, ReplaceStart: replaceStart}
	case "src":
		return Context{Kind: SrcLang, Prefix: partial, ReplaceStart: replaceStart}
	case "call":
		return Context{Kind: CallName, Prefix: partial, ReplaceStart: replaceStart}
	}

	if top, ok := st.topParen(); ok {
		if top.isCall && !top.sawEq {
			return Context{Kind: BindingLHS, Name: top.callee, Prefix: partial, ReplaceStart: replaceStart}
		}

		return Context{Kind: None} // value position / other parens: handled via dot refs
	}

	if stmtStart {
		if st.braceDepth > 0 {
			return Context{Kind: PipelineBody, Prefix: partial, ReplaceStart: replaceStart}
		}

		return Context{Kind: TopLevel, Prefix: partial, ReplaceStart: replaceStart}
	}

	return Context{Kind: None}
}

// enclosingPipeline returns the name of the pipeline whose declaration most
// recently precedes the cursor, or "".
func enclosingPipeline(text string, lines []string, line, col int) string {
	cursor := offsetOf(lines, line, col)
	name := ""
	for _, m := range pipelineRe.FindAllStringSubmatchIndex(text, -1) {
		if m[0] >= cursor {
			break
		}
		name = text[m[2]:m[3]]
	}

	return name
}

// offsetOf converts a (line, col) position into a byte offset into the text the
// lines were split from (with '\n' separators).
func offsetOf(lines []string, line, col int) int {
	off := 0
	for i := 0; i < line && i < len(lines); i++ {
		off += len(lines[i]) + 1 // + newline
	}

	return off + col
}
