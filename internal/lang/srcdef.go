package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// srcRe matches a stage's `src <lang> "<path>"` declaration, capturing the path.
var srcRe = regexp.MustCompile(`src\s+\w+\s+"([^"]+)"`)

// srcDefinition resolves go-to-definition when the cursor is on a stage's `src`
// path string: it points at the stage's implementation file on disk (searched
// along MROPATH), bridging the .mro and its code.
func (d *Document) srcDefinition(pos protocol.Position) (protocol.Location, bool) {
	lines := strings.Split(d.Text, "\n")
	if int(pos.Line) >= len(lines) {
		return protocol.Location{}, false
	}
	line := lines[pos.Line]
	m := srcRe.FindStringSubmatchIndex(line)
	if m == nil {
		return protocol.Location{}, false
	}
	pathStart, pathEnd := m[2], m[3]
	if col := int(pos.Character); col < pathStart || col > pathEnd {
		return protocol.Location{}, false
	}

	target := d.resolveSrc(line[pathStart:pathEnd])
	if target == "" {
		return protocol.Location{}, false
	}
	zero := protocol.Range{}

	return protocol.Location{URI: PathToURI(target), Range: zero}, true
}

// resolveSrc finds the stage implementation file for a src path along MROPATH,
// trying the path verbatim and with a .py suffix. Falls back to the first
// candidate so navigation still points where the file should live.
func (d *Document) resolveSrc(rel string) string {
	first := ""
	for _, dir := range d.MROPaths() {
		for _, cand := range []string{filepath.Join(dir, rel), filepath.Join(dir, rel+".py")} {
			if first == "" {
				first = cand
			}
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				return cand
			}
		}
	}

	return first
}
