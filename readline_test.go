package readline

import (
	"testing"
	"time"
)

func TestRace(t *testing.T) {
	rl, err := NewFromConfig(&Config{})
	if err != nil {
		t.Fatal(err)
		return
	}

	go func() {
		for range time.Tick(time.Millisecond) {
			rl.SetPrompt("hello")
		}
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		rl.Close()
	}()

	rl.Readline()
}

func TestParseCPRResponse(t *testing.T) {
	badResponses := []string{
		"",
		";",
		"\x00",
		"\x00;",
		";\x00",
		"x",
		"1;a",
		"a;1",
		"a;1;",
		"1;1;",
		"1;1;1",
	}
	for _, response := range badResponses {
		if _, err := parseCPRResponse([]byte(response)); err == nil {
			t.Fatalf("expected parsing of `%s` to fail, but did not", response)
		}
	}

	goodResponses := []struct {
		input  string
		output cursorPosition
	}{
		{"1;2", cursorPosition{1, 2}},
		{"0;2", cursorPosition{0, 2}},
		{"0;0", cursorPosition{0, 0}},
		{"48378;9999999", cursorPosition{48378, 9999999}},
	}

	for _, response := range goodResponses {
		got, err := parseCPRResponse([]byte(response.input))
		if err != nil {
			t.Fatalf("could not parse `%s`: %v", response.input, err)
		}
		if got != response.output {
			t.Fatalf("expected %s to parse to %#v, got %#v", response.input, response.output, got)
		}
	}
}

// TestPromptInLineEdge tests that a prompt that ends at the right edge of
// the terminal moves the cursor to the next line (with " \b") before the
// rest of the line is cleared, which would cancel the wrap at the edge, so
// that the input is written after the prompt, not over its end.
func TestPromptInLineEdge(t *testing.T) {
	output := func(prompt string, width, ppos int, input string) string {
		tm := &terminal{}
		tm.cfg.Store(&Config{Prompt: prompt, Painter: defaultPainter})
		tm.dimensions.Store(&termDimensions{width: width, height: 24})
		r := newRuneBuffer(tm)
		r.ppos = ppos
		r.buf = []rune(input)
		r.idx = len(r.buf)
		return string(r.output())
	}
	const clear = "\x1b[0K"
	for _, c := range []struct {
		prompt       string
		width, ppos  int
		input, wants string
	}{
		{"dir > ", 6, 0, "cmd", "dir >  \b" + clear + "cmd"},
		{"dir > ", 3, 0, "cmd", "dir >  \b" + clear + "cmd \b"}, // two lines, and the input fills one
		{"dir > ", 10, 4, "cmd", "dir >  \b" + clear + "cmd"},   // after output
		{"dir > ", 10, 0, "cmd", "dir > " + clear + "cmd"},
		{"note\ndir > ", 6, 3, "cmd", "note\ndir >  \b" + clear + "cmd"},
		{"", 6, 0, "cmd", clear + "cmd"},
	} {
		got := output(c.prompt, c.width, c.ppos, c.input)
		if got != c.wants {
			t.Errorf("prompt %q, width %d, ppos %d: got %q, wants %q", c.prompt, c.width, c.ppos, got, c.wants)
		}
	}
}
