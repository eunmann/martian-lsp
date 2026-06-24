package complete

import "strings"

// CallSignature reports the enclosing call's callee (DecId) and the active
// argument index (number of commas before the cursor at that call's level), or
// ok=false when the cursor is not inside a call argument list (or is in a string
// or comment). Pure; never parses the buffer.
func CallSignature(text string, line, col int) (string, int, bool) {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return "", 0, false
	}
	cur := lines[line]
	if col > len(cur) {
		col = len(cur)
	}
	if inStr, inComment := lineMasked(cur, col); inStr || inComment {
		return "", 0, false
	}

	st := scan(text, offsetOf(lines, line, col))
	if top, ok := st.topParen(); ok && top.isCall {
		return top.callee, top.commas, true
	}

	return "", 0, false
}

// CallContext reports the enclosing call's callee and the binding LHS names
// already present, or ok=false when the cursor is not inside a call argument
// list. Pure; never parses the buffer.
func CallContext(text string, line, col int) (string, []string, bool) {
	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return "", nil, false
	}
	cur := lines[line]
	if col > len(cur) {
		col = len(cur)
	}
	if inStr, inComment := lineMasked(cur, col); inStr || inComment {
		return "", nil, false
	}

	st := scan(text, offsetOf(lines, line, col))
	if top, found := st.topParen(); found && top.isCall {
		return top.callee, top.bound, true
	}

	return "", nil, false
}
