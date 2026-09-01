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

func TestSetBufferWithCursor(t *testing.T) {
	rl, err := NewFromConfig(&Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer rl.Close()

	rl.SetBufferWithCursor("héllo", 2)
	if got := string(rl.operation.buf.buf); got != "héllo" {
		t.Fatalf("buffer = %q, want %q", got, "héllo")
	}
	if idx := rl.operation.buf.idx; idx != 2 {
		t.Fatalf("cursor = %d, want 2", idx)
	}

	rl.SetBufferWithCursor("abc", -1)
	if idx := rl.operation.buf.idx; idx != 0 {
		t.Fatalf("negative pos clamped to %d, want 0", idx)
	}

	rl.SetBufferWithCursor("abc", 100)
	if idx := rl.operation.buf.idx; idx != 3 {
		t.Fatalf("oversized pos clamped to %d, want 3", idx)
	}
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
