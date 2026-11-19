//go:build js && wasm

package readline

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ergochat/readline/internal/platform"
)

// TestWasm tests the basic functions of readline in WebAssembly, run in a
// headless browser (see the wasm target of the Makefile). There is no
// terminal there, so the lines of the input are read as given, without a
// prompt or any line editing.
func TestWasm(t *testing.T) {
	if platform.DefaultIsTerminal() {
		t.Error("there is no terminal in the browser")
	}
	var out bytes.Buffer
	rl, err := NewFromConfig(&Config{
		Prompt: "> ",
		Stdin:  io.NopCloser(strings.NewReader("hello\nworld\n")),
		Stdout: &out,
		Stderr: &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rl.Close()
	for _, want := range []string{"hello", "world"} {
		line, err := rl.ReadLine()
		if err != nil || line != want {
			t.Errorf("ReadLine: got %q, %v; wants %q", line, err, want)
		}
	}
	if line, err := rl.ReadLine(); err != io.EOF {
		t.Errorf("ReadLine at the end: got %q, %v; wants io.EOF", line, err)
	}
	if _, err := rl.Write([]byte("output\n")); err != nil {
		t.Error(err)
	}
	if got := out.String(); got != "output\n" {
		t.Errorf("output: got %q, wants only what was written (no prompt)", got)
	}
}
