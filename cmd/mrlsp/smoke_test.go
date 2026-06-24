package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestStdioHandshake builds the mrlsp binary and drives a real LSP session over
// stdio: initialize -> initialized -> didOpen(broken doc) -> expect
// publishDiagnostics -> shutdown -> exit.
func TestStdioHandshake(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mrlsp")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build mrlsp: %v\n%s", err, out)
	}

	cmd := exec.Command(bin)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()

	r := bufio.NewReader(stdout)

	writeMsg(t, stdin, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"processId": nil, "rootUri": nil, "capabilities": map[string]any{}},
	})
	writeMsg(t, stdin, map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}})

	// Wait for the initialize result (id == 1) and check capabilities.
	initResult := readUntil(t, r, func(m map[string]any) bool {
		id, ok := m["id"]

		return ok && fmt.Sprint(id) == "1"
	})
	res, _ := initResult["result"].(map[string]any)
	if _, ok := res["capabilities"]; !ok {
		t.Fatalf("initialize result missing capabilities: %v", initResult)
	}

	// Open a document with an undefined-output error and expect diagnostics.
	broken := strings.Replace(validMROForSmoke,
		"greeting = HELLO.greeting,", "greeting = HELLO.missing,", 1)
	writeMsg(t, stdin, map[string]any{
		"jsonrpc": "2.0", "method": "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///tmp/martian-lsp-smoke/pipe.mro", "languageId": "mro",
				"version": 1, "text": broken,
			},
		},
	})

	diag := readUntil(t, r, func(m map[string]any) bool {
		return m["method"] == "textDocument/publishDiagnostics"
	})
	params, _ := diag["params"].(map[string]any)
	diags, _ := params["diagnostics"].([]any)
	if len(diags) == 0 {
		t.Fatalf("expected at least one diagnostic for broken doc, got: %v", params)
	}

	writeMsg(t, stdin, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown"})
	readUntil(t, r, func(m map[string]any) bool {
		id, ok := m["id"]

		return ok && fmt.Sprint(id) == "2"
	})
	writeMsg(t, stdin, map[string]any{"jsonrpc": "2.0", "method": "exit"})
}

const validMROForSmoke = `stage HELLO(
    in  string name,
    out string greeting,
    src py     "stages/hello",
)

pipeline HELLO_BATCH(
    in  string name,
    out string greeting,
)
{
    call HELLO(
        name = self.name,
    )
    return (
        greeting = HELLO.greeting,
    )
}
`

func writeMsg(t *testing.T, w io.Writer, msg map[string]any) {
	t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readMsg(r *bufio.Reader) (map[string]any, error) {
	length := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			n, err := strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
			if err != nil {
				return nil, err
			}
			length = n
		}
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(buf, &m); err != nil {
		return nil, err
	}

	return m, nil
}

func readUntil(t *testing.T, r *bufio.Reader, pred func(map[string]any) bool) map[string]any {
	t.Helper()
	done := make(chan map[string]any, 1)
	errc := make(chan error, 1)
	go func() {
		for {
			m, err := readMsg(r)
			if err != nil {
				errc <- err

				return
			}
			if pred(m) {
				done <- m

				return
			}
		}
	}()
	select {
	case m := <-done:
		return m
	case err := <-errc:
		t.Fatalf("read: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatalf("timeout waiting for matching LSP message")
	}

	return nil
}
