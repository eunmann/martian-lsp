package lang

import "testing"

type decodedToken struct {
	line, start, length, typ uint32
}

// decodeSemanticTokens reverses the LSP delta encoding into absolute tokens.
func decodeSemanticTokens(t *testing.T, data []uint32) []decodedToken {
	t.Helper()
	if len(data)%5 != 0 {
		t.Fatalf("token data length %d is not a multiple of 5", len(data))
	}
	var line, start uint32
	var out []decodedToken
	for i := 0; i < len(data); i += 5 {
		dl, ds, length, typ := data[i], data[i+1], data[i+2], data[i+3]
		line += dl
		if dl == 0 {
			start += ds
		} else {
			start = ds
		}
		out = append(out, decodedToken{line, start, length, typ})
	}

	return out
}

func hasToken(toks []decodedToken, line, start, length, typ uint32) bool {
	for _, tk := range toks {
		if tk.line == line && tk.start == start && tk.length == length && tk.typ == typ {
			return true
		}
	}

	return false
}

func TestSemanticTokens(t *testing.T) {
	d := docFrom(validMRO)
	toks := decodeSemanticTokens(t, d.SemanticTokens())
	if len(toks) == 0 {
		t.Fatal("no semantic tokens produced")
	}

	// "stage HELLO(" on line 2: HELLO at col 6, len 5, type function.
	if !hasToken(toks, 2, 6, 5, tokenFunction) {
		t.Errorf("missing function token for HELLO at 2:6; got %+v", toks)
	}
	// "    in  string name," on line 3: name at col 15, len 4, type parameter.
	if !hasToken(toks, 3, 15, 4, tokenParameter) {
		t.Errorf("missing parameter token for name at 3:15; got %+v", toks)
	}

	// Tokens must be ordered and non-overlapping (encoder invariant).
	for i := 1; i < len(toks); i++ {
		prev, cur := toks[i-1], toks[i]
		if cur.line < prev.line || (cur.line == prev.line && cur.start < prev.start+prev.length) {
			t.Errorf("tokens overlap or out of order: %+v then %+v", prev, cur)
		}
	}
}
