// Package complete implements Martian (MRO) autocompletion. The classifier and
// scanner are intentionally pure (no glsp or martian/syntax dependencies) so
// they can be exhaustively unit-tested on raw text — which matters because the
// buffer is usually unparseable mid-edit.
package complete

const maxRecent = 6 // identifier window kept before an opening paren

// lineMasked reports whether the byte offset upto on a single physical line
// falls inside a string literal or a comment. MRO strings and comments are both
// line-bounded (no triple-quotes; `#` runs to end of line), so a single-line
// forward scan is correct and sufficient for suppression.
func lineMasked(line string, upto int) (bool, bool) {
	if upto > len(line) {
		upto = len(line)
	}
	inString, esc := false, false
	for i := range upto {
		c := line[i]
		if inString {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inString = false
			}

			continue
		}
		switch c {
		case '#':
			return false, true // comment to end of line
		case '"':
			inString = true
		}
	}

	return inString, false
}

// parenFrame describes an open '(' encountered while scanning.
type parenFrame struct {
	isCall  bool     // a call argument list
	isUsing bool     // a using(...) block
	isDecl  bool     // a stage/pipeline/struct parameter list
	callee  string   // for call frames, the callable (DecId — the name after `call`)
	sawEq   bool     // an '=' has appeared since the last ',' or '(' at this level
	commas  int      // number of ',' seen at this level (the active argument index)
	bound   []string // binding LHS names already typed at this level
}

// scanState is the structural context at the scan endpoint.
type scanState struct {
	lastToken  string
	parens     []parenFrame
	braceDepth int
}

func (s *scanState) topParen() (parenFrame, bool) {
	if len(s.parens) == 0 {
		return parenFrame{}, false
	}

	return s.parens[len(s.parens)-1], true
}

func (s *scanState) setSawEq(v bool) {
	if n := len(s.parens); n > 0 {
		s.parens[n-1].sawEq = v
	}
}

// scan walks text[:end] tracking the bracket stack, brace depth, and last
// significant token, skipping string and comment spans.
func scan(text string, end int) scanState {
	if end > len(text) {
		end = len(text)
	}
	var st scanState
	var recent []string // identifiers since the last bracket/separator boundary

	for i := 0; i < end; {
		switch c := text[i]; {
		case c == '#':
			i = skipComment(text, i, end)
		case c == '"':
			i = skipString(text, i, end)
			recent, st.lastToken = nil, `""`
		case isIdentStart(c) || c == '@':
			var word string
			word, i = readIdent(text, i, end)
			st.lastToken = word
			recent = appendRecent(recent, word)
		default:
			i++
			recent = st.punct(c, recent)
		}
	}

	return st
}

// punct applies a single punctuation byte to the scan state and returns the
// identifier window to carry forward.
func (s *scanState) punct(c byte, recent []string) []string {
	switch c {
	case '(':
		s.parens = append(s.parens, makeFrame(recent))
		s.lastToken = "("
	case ')':
		if len(s.parens) > 0 {
			s.parens = s.parens[:len(s.parens)-1]
		}
		s.lastToken = ")"
	case '{':
		s.braceDepth++
		s.lastToken = "{"
	case '}':
		if s.braceDepth > 0 {
			s.braceDepth--
		}
		s.lastToken = "}"
	case ',':
		s.setSawEq(false)
		if n := len(s.parens); n > 0 {
			s.parens[n-1].commas++
		}
		s.lastToken = ","
	case '=':
		s.setSawEq(true)
		if n := len(s.parens); n > 0 && len(recent) > 0 {
			s.parens[n-1].bound = append(s.parens[n-1].bound, recent[len(recent)-1])
		}
		s.lastToken = "="

		return recent // value position; window irrelevant until next '('
	case ';':
		s.lastToken = ";"
	default:
		return recent // whitespace, '.', '*', '[', ']', '<', '>', ':'
	}

	return nil
}

func skipComment(text string, i, end int) int {
	for i < end && text[i] != '\n' {
		i++
	}

	return i
}

func skipString(text string, i, end int) int {
	i++ // opening quote
	for i < end {
		switch text[i] {
		case '\\':
			i += 2
		case '"':
			return i + 1
		default:
			i++
		}
	}

	return i
}

func readIdent(text string, i, end int) (string, int) {
	j := i + 1
	for j < end && isIdentPart(text[j]) {
		j++
	}

	return text[i:j], j
}

func appendRecent(recent []string, word string) []string {
	recent = append(recent, word)
	if len(recent) > maxRecent {
		recent = recent[len(recent)-maxRecent:]
	}

	return recent
}

// makeFrame classifies an opening paren from the identifier words preceding it.
func makeFrame(recent []string) parenFrame {
	var f parenFrame
	if len(recent) == 0 {
		return f
	}
	if recent[len(recent)-1] == "using" {
		f.isUsing = true

		return f
	}
	for k, w := range recent {
		if w == "call" {
			f.isCall = true
			if k+1 < len(recent) {
				f.callee = recent[k+1] // DecId; `call REF as ALIAS(` -> REF
			}

			return f
		}
	}
	for _, w := range recent {
		if w == "stage" || w == "pipeline" || w == "struct" {
			f.isDecl = true

			break
		}
	}

	return f
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
